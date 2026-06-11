package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_TestConnection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/myself" {
			t.Errorf("expected path /rest/api/3/myself, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"123"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user@example.com", "token")
	err := client.TestConnection()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClient_DetectStoryPointsField(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id":"customfield_101", "name":"Sprint"},
			{"id":"customfield_storypoints", "name":"Story Points"}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	field, err := client.DetectStoryPointsField()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if field != "customfield_storypoints" {
		t.Errorf("expected customfield_storypoints, got %s", field)
	}
}

func TestClient_FetchRetroData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			jql := r.URL.Query().Get("jql")
			expectedJQL := `(project = PROJ) AND ((statusCategoryChangedDate >= 2026-05-01 AND statusCategoryChangedDate <= "2026-05-15 23:59") OR (worklogDate >= 2026-05-01 AND worklogDate <= 2026-05-15))`
			if jql != expectedJQL {
				t.Errorf("expected JQL query %q, got %q", expectedJQL, jql)
			}
			// Mock search response
			resp := struct {
				Issues []map[string]interface{} `json:"issues"`
				Total  int                      `json:"total"`
			}{
				Total: 2,
				Issues: []map[string]interface{}{
					{
						"key": "PROJ-101",
						"fields": map[string]interface{}{
							"summary": "Sample Task",
							"status": map[string]interface{}{
								"name": "Done",
								"statusCategory": map[string]interface{}{
									"name": "Done",
									"key":  "done",
								},
							},
							"timespent":                    36000,
							"statuscategorychangeddate":    "2026-05-10T10:00:00.000Z",
							"customfield_storypoints":      3.0,
							"worklog": map[string]interface{}{
								"total":      2,
								"maxResults": 20,
								"worklogs": []map[string]interface{}{
									{
										"author": map[string]interface{}{
											"displayName": "Alice",
										},
										"timeSpentSeconds": 18000, // 5 hours
										"started":          "2026-05-05T09:00:00.000Z", // inside range
									},
									{
										"author": map[string]interface{}{
											"displayName": "Bob",
										},
										"timeSpentSeconds": 7200, // 2 hours
										"started":          "2026-04-28T09:00:00.000Z", // OUTSIDE range (starts before May 1)
									},
								},
							},
						},
					},
					{
						"key": "PROJ-102",
						"fields": map[string]interface{}{
							"summary": "In Progress Task",
							"status": map[string]interface{}{
								"name": "In Progress",
								"statusCategory": map[string]interface{}{
									"name": "In Progress",
									"key":  "indeterminate",
								},
							},
							"timespent":                 0,
							"statuscategorychangeddate": "2026-05-10T10:00:00.000Z",
							"customfield_storypoints":   5.0,
							"worklog": map[string]interface{}{
								"total":      0,
								"maxResults": 20,
								"worklogs":   []map[string]interface{}{},
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	data, err := client.FetchRetroData("project = PROJ", "2026-05-01", "2026-05-15", "customfield_storypoints")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Total story points should only count Done status categories (so only PROJ-101's 3.0 SP, not PROJ-102's 5.0 SP)
	if data.Totals.TotalStoryPoints != 3.0 {
		t.Errorf("expected 3.0 story points, got %f", data.Totals.TotalStoryPoints)
	}

	if len(data.Issues) != 2 {
		t.Fatalf("expected 2 issues in snapshot data, got %d", len(data.Issues))
	}

	if data.Issues[1].Key != "PROJ-102" || data.Issues[1].StoryPoints != 5.0 {
		t.Errorf("expected PROJ-102 to retain its 5.0 SP, got %+v", data.Issues[1])
	}

	// Bob should be filtered out in the date-bounded "totals" because his worklog was dated April 28, which is outside the May 1–15 range.
	// Total hours logged should be 5.0 (only Alice).
	if data.Totals.TotalHoursLogged != 5.0 {
		t.Errorf("expected 5.0 hours logged, got %f", data.Totals.TotalHoursLogged)
	}

	if len(data.Leaderboard) != 1 {
		t.Fatalf("expected window leaderboard to have 1 entry (Alice), got %d", len(data.Leaderboard))
	}

	if data.Leaderboard[0].AuthorName != "Alice" {
		t.Errorf("expected Alice in leaderboard, got %s", data.Leaderboard[0].AuthorName)
	}

	if data.Leaderboard[0].HoursLogged != 5.0 {
		t.Errorf("expected Alice to have 5.0 hours logged, got %f", data.Leaderboard[0].HoursLogged)
	}

	if len(data.Leaderboard[0].Worklogs) != 1 {
		t.Fatalf("expected Alice to have 1 worklog item, got %d", len(data.Leaderboard[0].Worklogs))
	}

	wlItem := data.Leaderboard[0].Worklogs[0]
	if wlItem.IssueKey != "PROJ-101" {
		t.Errorf("expected worklog issue key PROJ-101, got %s", wlItem.IssueKey)
	}

	if wlItem.IssueSummary != "Sample Task" {
		t.Errorf("expected worklog issue summary 'Sample Task', got '%s'", wlItem.IssueSummary)
	}

	if wlItem.HoursLogged != 5.0 {
		t.Errorf("expected worklog hours logged 5.0, got %f", wlItem.HoursLogged)
	}

	// Verify all-time metrics are also computed correctly in the same snapshot
	if data.TotalsAll.TotalHoursLogged != 7.0 {
		t.Errorf("expected 7.0 all-time hours logged, got %f", data.TotalsAll.TotalHoursLogged)
	}

	if len(data.LeaderboardAll) != 2 {
		t.Errorf("expected all-time leaderboard to have 2 entries, got %d", len(data.LeaderboardAll))
	}

	if data.Issues[0].TimeSpentHours != 5.0 {
		t.Errorf("expected issue window time spent to be 5.0, got %f", data.Issues[0].TimeSpentHours)
	}

	if data.Issues[0].TimeSpentHoursAll != 7.0 {
		t.Errorf("expected issue all time spent to be 7.0, got %f", data.Issues[0].TimeSpentHoursAll)
	}
}

func TestClient_InsecureTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"accountId":"123"}`))
	}))
	defer server.Close()

	clientDefault := NewClient(server.URL, "user@example.com", "token")
	err := clientDefault.TestConnection()
	if err == nil {
		t.Fatal("expected connection to fail on self-signed cert, but got no error")
	}

	clientInsecure := NewClient(server.URL, "user@example.com", "token")
	clientInsecure.Insecure = true
	err = clientInsecure.TestConnection()
	if err != nil {
		t.Fatalf("expected no error with Insecure=true, got %v", err)
	}
}

func TestClient_FetchRetroData_Pagination(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/3/search/jql" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		calls++

		q := r.URL.Query()
		if calls == 1 {
			if q.Get("nextPageToken") != "" {
				t.Errorf("expected no nextPageToken on first call, got %s", q.Get("nextPageToken"))
			}
			resp := map[string]interface{}{
				"nextPageToken": "token-1",
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-1",
						"fields": map[string]interface{}{
							"summary":                   "Task 1",
							"statuscategorychangeddate": "2026-05-10T10:00:00.000Z",
							"status": map[string]interface{}{
								"name": "Done",
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else if calls == 2 {
			if q.Get("nextPageToken") != "token-1" {
				t.Errorf("expected nextPageToken=token-1 on second call, got %s", q.Get("nextPageToken"))
			}
			resp := map[string]interface{}{
				"nextPageToken": "",
				"issues": []map[string]interface{}{
					{
						"key": "PROJ-2",
						"fields": map[string]interface{}{
							"summary":                   "Task 2",
							"statuscategorychangeddate": "2026-05-11T10:00:00.000Z",
							"status": map[string]interface{}{
								"name": "Done",
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			t.Errorf("unexpected call count: %d", calls)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	data, err := client.FetchRetroData("project = PROJ", "2026-05-01", "2026-05-15", "customfield_storypoints")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if calls != 2 {
		t.Errorf("expected 2 page requests, got %d", calls)
	}

	if len(data.Issues) != 2 {
		t.Fatalf("expected 2 issues fetched, got %d", len(data.Issues))
	}

	if data.Issues[0].Key != "PROJ-1" || data.Issues[1].Key != "PROJ-2" {
		t.Errorf("fetched issues are incorrect: %+v", data.Issues)
	}
}

func TestClient_FetchRetroData_BothWorklogs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/3/search/jql" {
			jql := r.URL.Query().Get("jql")
			expectedJQL := `(project = PROJ) AND ((statusCategoryChangedDate >= 2026-05-01 AND statusCategoryChangedDate <= "2026-05-15 23:59") OR (worklogDate >= 2026-05-01 AND worklogDate <= 2026-05-15))`
			if jql != expectedJQL {
				t.Errorf("expected JQL query %q, got %q", expectedJQL, jql)
			}
			resp := struct {
				Issues []map[string]interface{} `json:"issues"`
				Total  int                      `json:"total"`
			}{
				Total: 1,
				Issues: []map[string]interface{}{
					{
						"key": "PROJ-101",
						"fields": map[string]interface{}{
							"summary": "Sample Task",
							"status": map[string]interface{}{
								"name": "Done",
								"statusCategory": map[string]interface{}{
									"name": "Done",
									"key":  "done",
								},
							},
							"timespent":                 36000,
							"statuscategorychangeddate": "2026-05-10T10:00:00.000Z",
							"customfield_storypoints":   3.0,
							"worklog": map[string]interface{}{
								"total":      2,
								"maxResults": 20,
								"worklogs": []map[string]interface{}{
									{
										"author": map[string]interface{}{
											"displayName": "Alice",
										},
										"timeSpentSeconds": 18000, // 5 hours
										"started":          "2026-05-05T09:00:00.000Z", // inside range
									},
									{
										"author": map[string]interface{}{
											"displayName": "Bob",
										},
										"timeSpentSeconds": 7200, // 2 hours
										"started":          "2026-04-28T09:00:00.000Z", // OUTSIDE range
									},
								},
							},
						},
					},
				},
			}
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(resp)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL, "user", "token")
	data, err := client.FetchRetroData("project = PROJ", "2026-05-01", "2026-05-15", "customfield_storypoints")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Total hours logged in window should be 5.0 (Alice).
	if data.Totals.TotalHoursLogged != 5.0 {
		t.Errorf("expected 5.0 hours logged in window, got %f", data.Totals.TotalHoursLogged)
	}

	// Total hours logged all-time should be 7.0 (5.0 Alice + 2.0 Bob).
	if data.TotalsAll.TotalHoursLogged != 7.0 {
		t.Errorf("expected 7.0 hours logged all-time, got %f", data.TotalsAll.TotalHoursLogged)
	}

	if len(data.Leaderboard) != 1 {
		t.Fatalf("expected window leaderboard to have 1 entry, got %d", len(data.Leaderboard))
	}

	if len(data.LeaderboardAll) != 2 {
		t.Fatalf("expected all-time leaderboard to have 2 entries, got %d", len(data.LeaderboardAll))
	}
}


