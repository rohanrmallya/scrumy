package jira

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"scrumy/internal/models"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	BaseURL  string
	Username string
	Token    string
	Insecure bool
}

func NewClient(baseURL, username, token string) *Client {
	baseURL = strings.TrimSuffix(baseURL, "/")
	return &Client{
		BaseURL:  baseURL,
		Username: username,
		Token:    token,
	}
}

func (c *Client) request(method, path string, query url.Values, body io.Reader) (*http.Response, error) {
	u, err := url.Parse(c.BaseURL + path)
	if err != nil {
		return nil, err
	}
	if query != nil {
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.Username, c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	var transport http.RoundTripper = http.DefaultTransport
	if c.Insecure {
		transport = &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   15 * time.Second,
	}
	return client.Do(req)
}

func (c *Client) TestConnection() error {
	resp, err := c.request("GET", "/rest/api/3/myself", nil, nil)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed with status %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *Client) DetectStoryPointsField() (string, error) {
	resp, err := c.request("GET", "/rest/api/3/field", nil, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch fields: status %d", resp.StatusCode)
	}

	var fields []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&fields); err != nil {
		return "", err
	}

	// Exact match checks
	for _, f := range fields {
		nameLower := strings.ToLower(f.Name)
		if nameLower == "story points" || nameLower == "story point estimate" {
			return f.ID, nil
		}
	}

	// Partial match checks
	for _, f := range fields {
		nameLower := strings.ToLower(f.Name)
		if strings.Contains(nameLower, "story point") {
			return f.ID, nil
		}
	}

	return "", fmt.Errorf("story points field not found")
}

func (c *Client) FetchAllWorklogs(issueKey string) ([]jiraWorklog, error) {
	resp, err := c.request("GET", fmt.Sprintf("/rest/api/3/issue/%s/worklog", issueKey), nil, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch worklogs: status %d", resp.StatusCode)
	}

	var page jiraWorklogPage
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, err
	}
	return page.Worklogs, nil
}

func (c *Client) FetchRetroData(baseJQL, startStr, endStr, spField string) (*models.JiraSnapshotData, error) {
	startLoc, err := time.Parse("2006-01-02", startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start_date: %w", err)
	}
	endLoc, err := time.Parse("2006-01-02", endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end_date: %w", err)
	}
	// Extend end date to cover full day (23:59:59)
	endLoc = endLoc.AddDate(0, 0, 1).Add(-time.Second)

	// Automatically detect SP field if not provided
	if spField == "" {
		detected, err := c.DetectStoryPointsField()
		if err == nil {
			spField = detected
		} else {
			spField = "customfield_10016" // Common default
		}
	}

	// Date-bound JQL query
	finalJQL := fmt.Sprintf("(%s) AND statusCategoryChangedDate >= %s AND statusCategoryChangedDate <= \"%s 23:59\"", baseJQL, startStr, endStr)

	var allIssues []jiraSearchIssue
	nextPageToken := ""

	for {
		q := url.Values{}
		q.Set("jql", finalJQL)
		q.Set("maxResults", "50")
		if nextPageToken != "" {
			q.Set("nextPageToken", nextPageToken)
		}
		q.Set("fields", fmt.Sprintf("summary,status,assignee,worklog,timespent,statuscategorychangeddate,%s", spField))

		resp, err := c.request("GET", "/rest/api/3/search/jql", q, nil)
		if err != nil {
			return nil, err
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("search failed with status %d: %s", resp.StatusCode, string(body))
		}

		var searchResp struct {
			Issues        []jiraSearchIssue `json:"issues"`
			NextPageToken string            `json:"nextPageToken"`
		}
		decodeErr := json.NewDecoder(resp.Body).Decode(&searchResp)
		resp.Body.Close()
		if decodeErr != nil {
			return nil, decodeErr
		}

		allIssues = append(allIssues, searchResp.Issues...)
		if searchResp.NextPageToken == "" || len(searchResp.Issues) == 0 {
			break
		}
		nextPageToken = searchResp.NextPageToken
	}

	worklogFilteredHours := 0.0
	worklogFilteredCount := 0
	leaderboardMap := map[string]float64{}

	var issues []models.JiraIssue
	totalStoryPoints := 0.0

	for _, issue := range allIssues {
		var summary string
		if raw, ok := issue.RawFields["summary"]; ok {
			_ = json.Unmarshal(raw, &summary)
		}

		var status jiraStatus
		if raw, ok := issue.RawFields["status"]; ok {
			_ = json.Unmarshal(raw, &status)
		}

		var statusCategoryChangedDate string
		if raw, ok := issue.RawFields["statuscategorychangeddate"]; ok {
			_ = json.Unmarshal(raw, &statusCategoryChangedDate)
		}

		var worklogPage jiraWorklogPage
		var hasWorklog bool
		if raw, ok := issue.RawFields["worklog"]; ok {
			if json.Unmarshal(raw, &worklogPage) == nil {
				hasWorklog = true
			}
		}

		// Story points parsing
		var sp float64
		if rawSP, ok := issue.RawFields[spField]; ok {
			sp = getFloat64FromRaw(rawSP)
		}
		totalStoryPoints += sp

		// Get all worklogs
		var worklogs []jiraWorklog
		if hasWorklog {
			if worklogPage.Total > len(worklogPage.Worklogs) {
				fetched, err := c.FetchAllWorklogs(issue.Key)
				if err == nil {
					worklogs = fetched
				} else {
					worklogs = worklogPage.Worklogs
				}
			} else {
				worklogs = worklogPage.Worklogs
			}
		}

		issueTimeSpentSeconds := 0.0
		for _, wl := range worklogs {
			wlTime, err := parseJiraTime(wl.Started)
			if err != nil {
				continue
			}
			// Only count work logged strictly within start_date and end_date
			if (wlTime.After(startLoc) || wlTime.Equal(startLoc)) && (wlTime.Before(endLoc) || wlTime.Equal(endLoc)) {
				hours := wl.TimeSpentSeconds / 3600.0
				issueTimeSpentSeconds += wl.TimeSpentSeconds
				worklogFilteredHours += hours
				worklogFilteredCount++

				author := wl.Author.DisplayName
				if author == "" {
					author = "Unknown"
				}
				leaderboardMap[author] += hours
			}
		}

		issues = append(issues, models.JiraIssue{
			Key:                       issue.Key,
			Summary:                   summary,
			Status:                    status.Name,
			StoryPoints:               sp,
			TimeSpentHours:            issueTimeSpentSeconds / 3600.0,
			StatusCategoryChangedDate: statusCategoryChangedDate,
		})
	}

	var leaderboard []models.JiraLeaderboardEntry
	for author, hours := range leaderboardMap {
		pct := 0.0
		if worklogFilteredHours > 0 {
			pct = (hours / worklogFilteredHours) * 100.0
		}
		leaderboard = append(leaderboard, models.JiraLeaderboardEntry{
			AuthorName:  author,
			HoursLogged: round2(hours),
			Percentage:  round2(pct),
		})
	}

	// Sort leaderboard descending by hours logged
	sort.Slice(leaderboard, func(i, j int) bool {
		return leaderboard[i].HoursLogged > leaderboard[j].HoursLogged
	})

	avgHours := 0.0
	if totalStoryPoints > 0 {
		avgHours = worklogFilteredHours / totalStoryPoints
	}

	return &models.JiraSnapshotData{
		Issues: issues,
		Totals: models.JiraTotals{
			TotalStoryPoints: round2(totalStoryPoints),
			TotalHoursLogged: round2(worklogFilteredHours),
			TotalWorkLogs:    worklogFilteredCount,
			AvgHoursPerSP:    round2(avgHours),
		},
		Leaderboard: leaderboard,
	}, nil
}

// Helper structs and functions

type jiraSearchIssue struct {
	Key       string                     `json:"key"`
	RawFields map[string]json.RawMessage `json:"fields"`
}

type jiraStatus struct {
	Name           string             `json:"name"`
	StatusCategory jiraStatusCategory `json:"statusCategory"`
}

type jiraStatusCategory struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

type jiraUser struct {
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

type jiraWorklogPage struct {
	Total      int           `json:"total"`
	MaxResults int           `json:"maxResults"`
	Worklogs   []jiraWorklog `json:"worklogs"`
}

type jiraWorklog struct {
	Author           jiraUser `json:"author"`
	TimeSpentSeconds float64  `json:"timeSpentSeconds"`
	Started          string   `json:"started"`
}

func getFloat64FromRaw(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var val float64
	if err := json.Unmarshal(raw, &val); err == nil {
		return val
	}
	var str string
	if err := json.Unmarshal(raw, &str); err == nil {
		if f, err := strconv.ParseFloat(str, 64); err == nil {
			return f
		}
	}
	return 0
}

func parseJiraTime(tStr string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339, tStr); err == nil {
		return t, nil
	}
	formats := []string{
		"2006-01-02T15:04:05.000-0700",
		"2006-01-02T15:04:05.000Z0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, tStr); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", tStr)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
