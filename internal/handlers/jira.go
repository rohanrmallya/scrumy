package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"scrumy/internal/calc"
	"scrumy/internal/db"
	"scrumy/internal/jira"
	"scrumy/internal/models"
	"scrumy/internal/scheduler"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func parseDBTime(str string) *time.Time {
	str = strings.TrimSpace(str)
	if str == "" {
		return nil
	}
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		"2006-01-02T15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, str); err == nil {
			return &t
		}
	}
	return nil
}

type JiraHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

func (h *JiraHandler) getClientForTeam(teamID string) (*jira.Client, string, string, error) {
	var urlStr, user, token, jql, spField string
	var insecure bool
	err := h.DB.QueryRow(`
		SELECT jira_url, jira_user, jira_token, jira_jql, jira_sp_field, jira_insecure 
		FROM teams WHERE id = ?
	`, teamID).Scan(&urlStr, &user, &token, &jql, &spField, &insecure)
	if err == sql.ErrNoRows {
		return nil, "", "", fmt.Errorf("team not found")
	} else if err != nil {
		return nil, "", "", err
	}
	if urlStr == "" || user == "" || token == "" {
		return nil, "", "", fmt.Errorf("Jira integration is not fully configured")
	}
	client := jira.NewClient(urlStr, user, token)
	client.Insecure = insecure
	return client, jql, spField, nil
}

func (h *JiraHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		JiraURL      string `json:"jira_url"`
		JiraUser     string `json:"jira_user"`
		JiraToken    string `json:"jira_token"`
		JiraJQL      string `json:"jira_jql"`
		JiraSPField  string `json:"jira_sp_field"`
		JiraInsecure bool   `json:"jira_insecure"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}

	var err error
	if body.JiraToken == "" {
		_, err = h.DB.Exec(`
			UPDATE teams 
			SET jira_url = ?, jira_user = ?, jira_jql = ?, jira_sp_field = ?, jira_insecure = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, body.JiraURL, body.JiraUser, body.JiraJQL, body.JiraSPField, body.JiraInsecure, teamID)
	} else {
		_, err = h.DB.Exec(`
			UPDATE teams 
			SET jira_url = ?, jira_user = ?, jira_token = ?, jira_jql = ?, jira_sp_field = ?, jira_insecure = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, body.JiraURL, body.JiraUser, body.JiraToken, body.JiraJQL, body.JiraSPField, body.JiraInsecure, teamID)
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	respond(w, 200, map[string]bool{"updated": true})
}

func (h *JiraHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		JiraURL      string `json:"jira_url"`
		JiraUser     string `json:"jira_user"`
		JiraToken    string `json:"jira_token"`
		JiraInsecure bool   `json:"jira_insecure"`
	}
	_ = decode(r, &body)

	// If token is omitted, try fetching the existing one from the DB
	if body.JiraToken == "" {
		_ = h.DB.QueryRow("SELECT jira_token FROM teams WHERE id = ?", teamID).Scan(&body.JiraToken)
	}
	if body.JiraURL == "" {
		_ = h.DB.QueryRow("SELECT jira_url FROM teams WHERE id = ?", teamID).Scan(&body.JiraURL)
	}
	if body.JiraUser == "" {
		_ = h.DB.QueryRow("SELECT jira_user FROM teams WHERE id = ?", teamID).Scan(&body.JiraUser)
	}

	if body.JiraURL == "" || body.JiraUser == "" || body.JiraToken == "" {
		respondErr(w, 400, "Missing configuration (URL, User, or Token)")
		return
	}

	client := jira.NewClient(body.JiraURL, body.JiraUser, body.JiraToken)
	client.Insecure = body.JiraInsecure
	if err := client.TestConnection(); err != nil {
		respondErr(w, 400, err.Error())
		return
	}

	respond(w, 200, map[string]bool{"ok": true})
}

func (h *JiraHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name               string `json:"name"`
		StartDate          string `json:"start_date"` // YYYY-MM-DD
		EndDate            string `json:"end_date"`   // YYYY-MM-DD
		AllWorklogs        bool   `json:"all_worklogs"`
		AutoRefreshEnabled bool   `json:"auto_refresh_enabled"`
		AutoRefreshTime    string `json:"auto_refresh_time"`
	}
	if err := decode(r, &body); err != nil || body.Name == "" || body.StartDate == "" || body.EndDate == "" {
		respondErr(w, 400, "name, start_date, and end_date are required")
		return
	}

	if body.AutoRefreshTime == "" {
		body.AutoRefreshTime = "09:00"
	}

	client, jql, spField, err := h.getClientForTeam(teamID)
	if err != nil {
		respondErr(w, 400, err.Error())
		return
	}

	data, err := client.FetchRetroData(jql, body.StartDate, body.EndDate, spField)
	if err != nil {
		respondErr(w, 500, fmt.Sprintf("failed to fetch data from Jira: %v", err))
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	id := uuid.NewString()
	var nextRunAtVal sql.NullString
	if body.AutoRefreshEnabled {
		nextRun := scheduler.CalculateNextRun(body.AutoRefreshTime, time.Now())
		nextRunAtVal = sql.NullString{String: nextRun.Format("2006-01-02 15:04:05"), Valid: true}
	}

	_, err = h.DB.Exec(`
		INSERT INTO jira_snapshots (
			id, team_id, name, start_date, end_date, all_worklogs, data,
			auto_refresh_enabled, auto_refresh_time, next_run_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, id, teamID, body.Name, body.StartDate, body.EndDate, body.AllWorklogs, string(jsonData),
		body.AutoRefreshEnabled, body.AutoRefreshTime, nextRunAtVal)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	h.GetSnapshotByID(w, r, teamID, id)
}

func (h *JiraHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)

	rows, err := h.DB.Query(`
		SELECT id, team_id, name, start_date, end_date, all_worklogs,
		       auto_refresh_enabled, auto_refresh_time, next_run_at,
		       last_auto_refreshed_at, last_auto_refresh_status, last_auto_refresh_msg,
		       created_at, updated_at
		FROM jira_snapshots 
		WHERE team_id = ? 
		ORDER BY created_at DESC
	`, teamID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var list []models.JiraSnapshot
	for rows.Next() {
		var s models.JiraSnapshot
		var createdAt, updatedAt string
		var nextRunAt, lastAutoRefreshedAt sql.NullString
		err := rows.Scan(
			&s.ID, &s.TeamID, &s.Name, &s.StartDate, &s.EndDate, &s.AllWorklogs,
			&s.AutoRefreshEnabled, &s.AutoRefreshTime, &nextRunAt,
			&lastAutoRefreshedAt, &s.LastAutoRefreshStatus, &s.LastAutoRefreshMsg,
			&createdAt, &updatedAt,
		)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		if t := parseDBTime(createdAt); t != nil {
			s.CreatedAt = *t
		}
		if t := parseDBTime(updatedAt); t != nil {
			s.UpdatedAt = *t
		}
		if nextRunAt.Valid {
			s.NextRunAt = parseDBTime(nextRunAt.String)
		}
		if lastAutoRefreshedAt.Valid {
			s.LastAutoRefreshedAt = parseDBTime(lastAutoRefreshedAt.String)
		}
		list = append(list, s)
	}
	if list == nil {
		list = []models.JiraSnapshot{}
	}
	respond(w, 200, list)
}

func (h *JiraHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	id := chi.URLParam(r, "snapshotID")
	h.GetSnapshotByID(w, r, teamID, id)
}

func (h *JiraHandler) fetchSnapshot(teamID, id string) (*models.JiraSnapshot, int, error) {
	var s models.JiraSnapshot
	var dataStr string
	var createdAt, updatedAt string
	var nextRunAt, lastAutoRefreshedAt sql.NullString
	err := h.DB.QueryRow(`
		SELECT id, team_id, name, start_date, end_date, all_worklogs, data,
		       auto_refresh_enabled, auto_refresh_time, next_run_at,
		       last_auto_refreshed_at, last_auto_refresh_status, last_auto_refresh_msg,
		       created_at, updated_at
		FROM jira_snapshots 
		WHERE id = ? AND team_id = ?
	`, id, teamID).Scan(
		&s.ID, &s.TeamID, &s.Name, &s.StartDate, &s.EndDate, &s.AllWorklogs, &dataStr,
		&s.AutoRefreshEnabled, &s.AutoRefreshTime, &nextRunAt,
		&lastAutoRefreshedAt, &s.LastAutoRefreshStatus, &s.LastAutoRefreshMsg,
		&createdAt, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, 404, fmt.Errorf("snapshot not found")
	} else if err != nil {
		return nil, 500, err
	}
	if t := parseDBTime(createdAt); t != nil {
		s.CreatedAt = *t
	}
	if t := parseDBTime(updatedAt); t != nil {
		s.UpdatedAt = *t
	}
	if nextRunAt.Valid {
		s.NextRunAt = parseDBTime(nextRunAt.String)
	}
	if lastAutoRefreshedAt.Valid {
		s.LastAutoRefreshedAt = parseDBTime(lastAutoRefreshedAt.String)
	}

	var parsedData models.JiraSnapshotData
	if err := json.Unmarshal([]byte(dataStr), &parsedData); err == nil {
		s.Data = parsedData
	} else {
		s.Data = map[string]any{}
	}

	rows, err := h.DB.Query(`
		SELECT id, snapshot_id, prev_story_points, new_story_points,
		       prev_hours_logged, new_hours_logged, prev_worklogs_count, new_worklogs_count,
		       prev_issues_count, new_issues_count, refreshed_at,
		       user_deltas, user_deltas_all, COALESCE(zero_log_members, '[]')
		FROM jira_snapshot_refresh_logs
		WHERE snapshot_id = ?
		ORDER BY refreshed_at DESC
	`, id)
	if err != nil {
		return nil, 500, err
	}
	defer rows.Close()

	var history []models.JiraSnapshotRefresh
	for rows.Next() {
		var hr models.JiraSnapshotRefresh
		var refreshedAt string
		var userDeltasStr, userDeltasAllStr, zeroLogMembersStr string
		err := rows.Scan(&hr.ID, &hr.SnapshotID, &hr.PrevStoryPoints, &hr.NewStoryPoints,
			&hr.PrevHoursLogged, &hr.NewHoursLogged, &hr.PrevWorklogsCount, &hr.NewWorklogsCount,
			&hr.PrevIssuesCount, &hr.NewIssuesCount, &refreshedAt,
			&userDeltasStr, &userDeltasAllStr, &zeroLogMembersStr)
		if err != nil {
			return nil, 500, err
		}
		hr.RefreshedAt, _ = time.Parse("2006-01-02 15:04:05", refreshedAt)
		if hr.RefreshedAt.IsZero() {
			hr.RefreshedAt, _ = time.Parse(time.RFC3339, refreshedAt)
		}
		if hr.RefreshedAt.IsZero() {
			hr.RefreshedAt, _ = time.Parse("2006-01-02T15:04:05Z", refreshedAt)
		}
		if userDeltasStr != "" {
			_ = json.Unmarshal([]byte(userDeltasStr), &hr.UserDeltas)
		}
		if hr.UserDeltas == nil {
			hr.UserDeltas = []models.JiraUserRefreshDelta{}
		}
		if userDeltasAllStr != "" {
			_ = json.Unmarshal([]byte(userDeltasAllStr), &hr.UserDeltasAll)
		}
		if hr.UserDeltasAll == nil {
			hr.UserDeltasAll = []models.JiraUserRefreshDelta{}
		}
		if zeroLogMembersStr != "" {
			_ = json.Unmarshal([]byte(zeroLogMembersStr), &hr.ZeroLogMembers)
		}
		if hr.ZeroLogMembers == nil {
			hr.ZeroLogMembers = []models.JiraZeroLogMember{}
		}
		history = append(history, hr)
	}
	if history == nil {
		history = []models.JiraSnapshotRefresh{}
	}
	s.RefreshHistory = history

	return &s, 200, nil
}

func (h *JiraHandler) GetSnapshotByID(w http.ResponseWriter, r *http.Request, teamID, id string) {
	s, code, err := h.fetchSnapshot(teamID, id)
	if err != nil {
		respondErr(w, code, err.Error())
		return
	}
	respond(w, 200, s)
}

func (h *JiraHandler) executeSnapshotRefresh(teamID, id string) (*models.JiraSnapshot, int, error) {
	var name, startDate, endDate, oldDataStr string
	var allWorklogs bool
	err := h.DB.QueryRow(`
		SELECT name, start_date, end_date, all_worklogs, data FROM jira_snapshots 
		WHERE id = ? AND team_id = ?
	`, id, teamID).Scan(&name, &startDate, &endDate, &allWorklogs, &oldDataStr)
	if err == sql.ErrNoRows {
		return nil, 404, fmt.Errorf("snapshot not found")
	} else if err != nil {
		return nil, 500, err
	}

	var oldData models.JiraSnapshotData
	if oldDataStr != "" {
		_ = json.Unmarshal([]byte(oldDataStr), &oldData)
	}

	client, jql, spField, err := h.getClientForTeam(teamID)
	if err != nil {
		return nil, 400, err
	}

	data, err := client.FetchRetroData(jql, startDate, endDate, spField)
	if err != nil {
		return nil, 500, fmt.Errorf("failed to refresh data from Jira: %v", err)
	}

	// Preserve IsUnplanned from oldData for any matching issue keys
	if len(oldData.Issues) > 0 {
		unplannedMap := make(map[string]bool)
		for _, iss := range oldData.Issues {
			if iss.IsUnplanned {
				unplannedMap[iss.Key] = true
			}
		}
		for i := range data.Issues {
			if unplannedMap[data.Issues[i].Key] {
				data.Issues[i].IsUnplanned = true
			}
		}
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, 500, err
	}

	// Calculate changes based on whether the snapshot is configured with all worklogs
	var prevSP, newSP float64
	var prevHours, newHours float64
	var prevWorklogs, newWorklogs int

	if allWorklogs {
		prevSP = oldData.TotalsAll.TotalStoryPoints
		prevHours = oldData.TotalsAll.TotalHoursLogged
		prevWorklogs = oldData.TotalsAll.TotalWorkLogs

		newSP = data.TotalsAll.TotalStoryPoints
		newHours = data.TotalsAll.TotalHoursLogged
		newWorklogs = data.TotalsAll.TotalWorkLogs
	} else {
		prevSP = oldData.Totals.TotalStoryPoints
		prevHours = oldData.Totals.TotalHoursLogged
		prevWorklogs = oldData.Totals.TotalWorkLogs

		newSP = data.Totals.TotalStoryPoints
		newHours = data.Totals.TotalHoursLogged
		newWorklogs = data.Totals.TotalWorkLogs
	}

	prevIssues := len(oldData.Issues)
	newIssues := len(data.Issues)

	userDeltas := calc.ComputeUserDeltas(oldData.Leaderboard, data.Leaderboard)
	userDeltasAll := calc.ComputeUserDeltas(oldData.LeaderboardAll, data.LeaderboardAll)

	userDeltasJson, err := json.Marshal(userDeltas)
	if err != nil {
		userDeltasJson = []byte("[]")
	}
	userDeltasAllJson, err := json.Marshal(userDeltasAll)
	if err != nil {
		userDeltasAllJson = []byte("[]")
	}

	// Compute team mates who did not log work in this snapshot
	activeLeaderboard := data.Leaderboard
	if allWorklogs {
		activeLeaderboard = data.LeaderboardAll
	}

	type loggedContributor struct {
		accountID string
		email     string
		hours     float64
	}
	loggedByLowerName := make(map[string]loggedContributor)
	loggedByAccountID := make(map[string]loggedContributor)
	loggedByEmail := make(map[string]loggedContributor)

	for _, entry := range activeLeaderboard {
		c := loggedContributor{
			accountID: strings.TrimSpace(entry.AuthorAccountID),
			email:     strings.ToLower(strings.TrimSpace(entry.AuthorEmail)),
			hours:     entry.HoursLogged,
		}
		if entry.AuthorName != "" {
			loggedByLowerName[strings.ToLower(strings.TrimSpace(entry.AuthorName))] = c
		}
		if c.accountID != "" {
			loggedByAccountID[c.accountID] = c
		}
		if c.email != "" {
			loggedByEmail[c.email] = c
		}
	}

	var zeroLogMembers []models.JiraZeroLogMember
	mateRows, err := h.DB.Query("SELECT name, email, jira_account_id FROM team_mates WHERE team_id = ?", teamID)
	if err == nil {
		for mateRows.Next() {
			var mName, mEmail, mJiraID string
			if err := mateRows.Scan(&mName, &mEmail, &mJiraID); err == nil {
				nameLower := strings.ToLower(strings.TrimSpace(mName))
				emailLower := strings.ToLower(strings.TrimSpace(mEmail))
				jiraID := strings.TrimSpace(mJiraID)

				var matchedHours float64
				var found bool

				if emailLower != "" {
					if c, ok := loggedByEmail[emailLower]; ok {
						matchedHours = c.hours
						found = true
					}
				}
				if !found && jiraID != "" {
					if c, ok := loggedByAccountID[jiraID]; ok {
						matchedHours = c.hours
						found = true
					}
				}
				if !found {
					if c, ok := loggedByLowerName[nameLower]; ok {
						matchedHours = c.hours
						found = true
					}
				}

				if !found || matchedHours == 0 {
					zeroLogMembers = append(zeroLogMembers, models.JiraZeroLogMember{
						Name:          mName,
						Email:         mEmail,
						JiraAccountID: mJiraID,
						TotalHours:    matchedHours,
					})
				}
			}
		}
		mateRows.Close()
	}

	zeroLogMembersJson, err := json.Marshal(zeroLogMembers)
	if err != nil {
		zeroLogMembersJson = []byte("[]")
	}

	// Update snapshot and write history in a transaction
	tx, err := h.DB.Begin()
	if err != nil {
		return nil, 500, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(`
		UPDATE jira_snapshots 
		SET data = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND team_id = ?
	`, string(jsonData), id, teamID)
	if err != nil {
		return nil, 500, err
	}

	refreshID := uuid.NewString()
	_, err = tx.Exec(`
		INSERT INTO jira_snapshot_refresh_logs (
			id, snapshot_id, prev_story_points, new_story_points,
			prev_hours_logged, new_hours_logged, prev_worklogs_count, new_worklogs_count,
			prev_issues_count, new_issues_count, refreshed_at,
			user_deltas, user_deltas_all, zero_log_members
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?, ?, ?)
	`, refreshID, id, prevSP, newSP, prevHours, newHours, prevWorklogs, newWorklogs, prevIssues, newIssues, string(userDeltasJson), string(userDeltasAllJson), string(zeroLogMembersJson))
	if err != nil {
		return nil, 500, err
	}

	if err := tx.Commit(); err != nil {
		return nil, 500, err
	}

	return h.fetchSnapshot(teamID, id)
}

func (h *JiraHandler) RefreshSnapshot(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	id := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	s, code, err := h.executeSnapshotRefresh(teamID, id)
	if err != nil {
		respondErr(w, code, err.Error())
		return
	}
	respond(w, 200, s)
}

type SnapshotRefreshResult struct {
	TeamID       string `json:"team_id"`
	TeamName     string `json:"team_name"`
	SnapshotID   string `json:"snapshot_id"`
	SnapshotName string `json:"snapshot_name"`
	Status       string `json:"status"` // "refreshed", "skipped", "failed"
	Error        string `json:"error,omitempty"`
}

type BulkRefreshResponse struct {
	Total     int                     `json:"total"`
	Refreshed int                     `json:"refreshed"`
	Skipped   int                     `json:"skipped"`
	Failed    int                     `json:"failed"`
	Results   []SnapshotRefreshResult `json:"results"`
}

func (h *JiraHandler) RefreshAllSnapshots(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT s.id, s.team_id, s.name, t.name, t.jira_url, t.jira_user, t.jira_token
		FROM jira_snapshots s
		JOIN teams t ON s.team_id = t.id
		ORDER BY t.name ASC, s.created_at DESC
	`)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	type snapRow struct {
		id, teamID, snapName, teamName, jiraURL, jiraUser, jiraToken string
	}
	var snapRows []snapRow
	for rows.Next() {
		var sr snapRow
		if err := rows.Scan(&sr.id, &sr.teamID, &sr.snapName, &sr.teamName, &sr.jiraURL, &sr.jiraUser, &sr.jiraToken); err == nil {
			snapRows = append(snapRows, sr)
		}
	}

	resp := BulkRefreshResponse{
		Total:   len(snapRows),
		Results: make([]SnapshotRefreshResult, 0, len(snapRows)),
	}

	for _, sr := range snapRows {
		if sr.jiraURL == "" || sr.jiraUser == "" || sr.jiraToken == "" {
			resp.Skipped++
			resp.Results = append(resp.Results, SnapshotRefreshResult{
				TeamID:       sr.teamID,
				TeamName:     sr.teamName,
				SnapshotID:   sr.id,
				SnapshotName: sr.snapName,
				Status:       "skipped",
				Error:        "Jira integration is not fully configured for this project",
			})
			continue
		}

		_, _, err := h.executeSnapshotRefresh(sr.teamID, sr.id)
		if err != nil {
			resp.Failed++
			resp.Results = append(resp.Results, SnapshotRefreshResult{
				TeamID:       sr.teamID,
				TeamName:     sr.teamName,
				SnapshotID:   sr.id,
				SnapshotName: sr.snapName,
				Status:       "failed",
				Error:        err.Error(),
			})
		} else {
			resp.Refreshed++
			resp.Results = append(resp.Results, SnapshotRefreshResult{
				TeamID:       sr.teamID,
				TeamName:     sr.teamName,
				SnapshotID:   sr.id,
				SnapshotName: sr.snapName,
				Status:       "refreshed",
			})
		}
	}

	respond(w, 200, resp)
}

func (h *JiraHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	id := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM jira_snapshots WHERE id = ? AND team_id = ?`, id, teamID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondErr(w, 404, "snapshot not found")
		return
	}

	respond(w, 200, map[string]bool{"deleted": true})
}

func (h *JiraHandler) UpdateSnapshot(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	id := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name               string  `json:"name"`
		StartDate          string  `json:"start_date"`
		EndDate            string  `json:"end_date"`
		AutoRefreshEnabled *bool   `json:"auto_refresh_enabled"`
		AutoRefreshTime    *string `json:"auto_refresh_time"`
	}
	if err := decode(r, &body); err != nil || body.Name == "" || body.StartDate == "" || body.EndDate == "" {
		respondErr(w, 400, "name, start_date, and end_date are required")
		return
	}

	var oldName, oldStartDate, oldEndDate, oldDataStr, oldAutoRefreshTime string
	var allWorklogs, oldAutoRefreshEnabled bool
	err := h.DB.QueryRow(`
		SELECT name, start_date, end_date, all_worklogs, data, auto_refresh_enabled, auto_refresh_time
		FROM jira_snapshots 
		WHERE id = ? AND team_id = ?
	`, id, teamID).Scan(&oldName, &oldStartDate, &oldEndDate, &allWorklogs, &oldDataStr, &oldAutoRefreshEnabled, &oldAutoRefreshTime)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "snapshot not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	var oldData models.JiraSnapshotData
	if oldDataStr != "" {
		_ = json.Unmarshal([]byte(oldDataStr), &oldData)
	}

	var dataJson *string
	if oldStartDate != body.StartDate || oldEndDate != body.EndDate {
		client, jql, spField, err := h.getClientForTeam(teamID)
		if err != nil {
			respondErr(w, 400, err.Error())
			return
		}

		data, err := client.FetchRetroData(jql, body.StartDate, body.EndDate, spField)
		if err != nil {
			respondErr(w, 500, fmt.Sprintf("failed to fetch data from Jira: %v", err))
			return
		}

		// Preserve IsUnplanned
		if len(oldData.Issues) > 0 {
			unplannedMap := make(map[string]bool)
			for _, iss := range oldData.Issues {
				if iss.IsUnplanned {
					unplannedMap[iss.Key] = true
				}
			}
			for i := range data.Issues {
				if unplannedMap[data.Issues[i].Key] {
					data.Issues[i].IsUnplanned = true
				}
			}
		}

		jsonData, err := json.Marshal(data)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		s := string(jsonData)
		dataJson = &s
	}

	autoRefreshEnabled := oldAutoRefreshEnabled
	if body.AutoRefreshEnabled != nil {
		autoRefreshEnabled = *body.AutoRefreshEnabled
	}
	autoRefreshTime := oldAutoRefreshTime
	if body.AutoRefreshTime != nil && strings.TrimSpace(*body.AutoRefreshTime) != "" {
		autoRefreshTime = strings.TrimSpace(*body.AutoRefreshTime)
	}
	if autoRefreshTime == "" {
		autoRefreshTime = "09:00"
	}

	var nextRunAtVal sql.NullString
	if autoRefreshEnabled {
		nextRun := scheduler.CalculateNextRun(autoRefreshTime, time.Now())
		nextRunAtVal = sql.NullString{String: nextRun.Format("2006-01-02 15:04:05"), Valid: true}
	}

	if dataJson != nil {
		_, err = h.DB.Exec(`
			UPDATE jira_snapshots 
			SET name = ?, start_date = ?, end_date = ?, data = ?,
			    auto_refresh_enabled = ?, auto_refresh_time = ?, next_run_at = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND team_id = ?
		`, body.Name, body.StartDate, body.EndDate, *dataJson,
			autoRefreshEnabled, autoRefreshTime, nextRunAtVal, id, teamID)
	} else {
		_, err = h.DB.Exec(`
			UPDATE jira_snapshots 
			SET name = ?, start_date = ?, end_date = ?,
			    auto_refresh_enabled = ?, auto_refresh_time = ?, next_run_at = ?,
			    updated_at = CURRENT_TIMESTAMP
			WHERE id = ? AND team_id = ?
		`, body.Name, body.StartDate, body.EndDate,
			autoRefreshEnabled, autoRefreshTime, nextRunAtVal, id, teamID)
	}

	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	h.GetSnapshotByID(w, r, teamID, id)
}

func (h *JiraHandler) ToggleIssuePlanned(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	snapshotID := chi.URLParam(r, "snapshotID")
	issueKey := chi.URLParam(r, "issueKey")

	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		IsUnplanned *bool `json:"is_unplanned"`
	}
	_ = decode(r, &body)

	var dataStr string
	err := h.DB.QueryRow(`SELECT data FROM jira_snapshots WHERE id = ? AND team_id = ?`, snapshotID, teamID).Scan(&dataStr)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "snapshot not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	var snapshotData models.JiraSnapshotData
	if err := json.Unmarshal([]byte(dataStr), &snapshotData); err != nil {
		respondErr(w, 500, "failed to parse snapshot data")
		return
	}

	found := false
	for i := range snapshotData.Issues {
		if snapshotData.Issues[i].Key == issueKey {
			if body.IsUnplanned != nil {
				snapshotData.Issues[i].IsUnplanned = *body.IsUnplanned
			} else {
				snapshotData.Issues[i].IsUnplanned = !snapshotData.Issues[i].IsUnplanned
			}
			found = true
			break
		}
	}

	if !found {
		respondErr(w, 404, "issue not found in snapshot")
		return
	}

	updatedBytes, err := json.Marshal(snapshotData)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	_, err = h.DB.Exec(`UPDATE jira_snapshots SET data = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND team_id = ?`, string(updatedBytes), snapshotID, teamID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	h.GetSnapshotByID(w, r, teamID, snapshotID)
}

func (h *JiraHandler) ExecuteAutoRefresh(teamID, snapshotID, triggerType string) (*models.JiraAutoRefreshAudit, error) {
	startTime := time.Now()

	var name, startDate, endDate, autoRefreshTime string
	var enabled bool
	err := h.DB.QueryRow(`
		SELECT name, start_date, end_date, auto_refresh_enabled, auto_refresh_time
		FROM jira_snapshots
		WHERE id = ? AND team_id = ?
	`, snapshotID, teamID).Scan(&name, &startDate, &endDate, &enabled, &autoRefreshTime)
	if err != nil {
		return nil, fmt.Errorf("snapshot not found: %w", err)
	}

	isActive, dateStatus := scheduler.IsSnapshotActive(startDate, endDate, startTime)

	// Refresh only between active dates and not after this
	if triggerType == "scheduled" && !isActive {
		msg := fmt.Sprintf("Auto-refresh skipped: snapshot active window is %s to %s (currently %s)", startDate, endDate, dateStatus)
		nextRun := scheduler.CalculateNextRun(autoRefreshTime, startTime)

		auditID := uuid.NewString()
		_, _ = h.DB.Exec(`
			INSERT INTO jira_auto_refresh_audits (
				id, snapshot_id, team_id, triggered_at, trigger_type, status, message, duration_ms
			) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, 'skipped', ?, 0)
		`, auditID, snapshotID, teamID, triggerType, msg)

		_, _ = h.DB.Exec(`
			UPDATE jira_snapshots
			SET last_auto_refresh_status = 'skipped',
			    last_auto_refresh_msg = ?,
			    next_run_at = ?
			WHERE id = ? AND team_id = ?
		`, msg, nextRun.Format("2006-01-02 15:04:05"), snapshotID, teamID)

		return &models.JiraAutoRefreshAudit{
			ID:           auditID,
			SnapshotID:   snapshotID,
			SnapshotName: name,
			TeamID:       teamID,
			TriggeredAt:  startTime,
			TriggerType:  triggerType,
			Status:       "skipped",
			Message:      msg,
		}, nil
	}

	snap, _, err := h.executeSnapshotRefresh(teamID, snapshotID)
	durationMs := time.Since(startTime).Milliseconds()
	nextRun := scheduler.CalculateNextRun(autoRefreshTime, startTime)

	if err != nil {
		errMsg := err.Error()
		auditID := uuid.NewString()
		_, _ = h.DB.Exec(`
			INSERT INTO jira_auto_refresh_audits (
				id, snapshot_id, team_id, triggered_at, trigger_type, status, message, duration_ms
			) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, 'failed', ?, ?)
		`, auditID, snapshotID, teamID, triggerType, errMsg, durationMs)

		_, _ = h.DB.Exec(`
			UPDATE jira_snapshots
			SET last_auto_refreshed_at = CURRENT_TIMESTAMP,
			    last_auto_refresh_status = 'failed',
			    last_auto_refresh_msg = ?,
			    next_run_at = ?
			WHERE id = ? AND team_id = ?
		`, errMsg, nextRun.Format("2006-01-02 15:04:05"), snapshotID, teamID)

		return &models.JiraAutoRefreshAudit{
			ID:           auditID,
			SnapshotID:   snapshotID,
			SnapshotName: name,
			TeamID:       teamID,
			TriggeredAt:  startTime,
			TriggerType:  triggerType,
			Status:       "failed",
			Message:      errMsg,
			DurationMs:   durationMs,
		}, err
	}

	var prevSP, newSP, prevHours, newHours float64
	var prevIssues, newIssues int
	if len(snap.RefreshHistory) > 0 {
		latest := snap.RefreshHistory[0]
		prevSP = latest.PrevStoryPoints
		newSP = latest.NewStoryPoints
		prevHours = latest.PrevHoursLogged
		newHours = latest.NewHoursLogged
		prevIssues = latest.PrevIssuesCount
		newIssues = latest.NewIssuesCount
	}

	successMsg := fmt.Sprintf("Refreshed successfully (%.1f SP, %.1fh logged)", newSP, newHours)
	auditID := uuid.NewString()
	_, _ = h.DB.Exec(`
		INSERT INTO jira_auto_refresh_audits (
			id, snapshot_id, team_id, triggered_at, trigger_type, status, message,
			prev_story_points, new_story_points, prev_hours_logged, new_hours_logged,
			prev_issues_count, new_issues_count, duration_ms
		) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, 'success', ?, ?, ?, ?, ?, ?, ?, ?)
	`, auditID, snapshotID, teamID, triggerType, successMsg,
		prevSP, newSP, prevHours, newHours, prevIssues, newIssues, durationMs)

	_, _ = h.DB.Exec(`
		UPDATE jira_snapshots
		SET last_auto_refreshed_at = CURRENT_TIMESTAMP,
		    last_auto_refresh_status = 'success',
		    last_auto_refresh_msg = ?,
		    next_run_at = ?
		WHERE id = ? AND team_id = ?
	`, successMsg, nextRun.Format("2006-01-02 15:04:05"), snapshotID, teamID)

	return &models.JiraAutoRefreshAudit{
		ID:              auditID,
		SnapshotID:      snapshotID,
		SnapshotName:    name,
		TeamID:          teamID,
		TriggeredAt:     startTime,
		TriggerType:     triggerType,
		Status:          "success",
		Message:         successMsg,
		PrevStoryPoints: prevSP,
		NewStoryPoints:  newSP,
		PrevHoursLogged: prevHours,
		NewHoursLogged:  newHours,
		PrevIssuesCount: prevIssues,
		NewIssuesCount:  newIssues,
		DurationMs:      durationMs,
	}, nil
}

func (h *JiraHandler) UpdateAutoRefreshSettings(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	snapshotID := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Enabled bool   `json:"auto_refresh_enabled"`
		Time    string `json:"auto_refresh_time"`
	}
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		respondErr(w, 400, "invalid body")
		return
	}
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		var boolVal bool
		if errBool := json.Unmarshal(bodyBytes, &boolVal); errBool == nil {
			body.Enabled = boolVal
		} else {
			respondErr(w, 400, "invalid body")
			return
		}
	}
	if body.Time == "" {
		var existingTime sql.NullString
		_ = h.DB.QueryRow(`SELECT auto_refresh_time FROM jira_snapshots WHERE id = ? AND team_id = ?`, snapshotID, teamID).Scan(&existingTime)
		if existingTime.Valid && existingTime.String != "" {
			body.Time = existingTime.String
		} else {
			body.Time = "09:00"
		}
	}

	now := time.Now()
	var nextRunAtVal sql.NullString
	if body.Enabled {
		nextRun := scheduler.CalculateNextRun(body.Time, now)
		nextRunAtVal = sql.NullString{String: nextRun.Format("2006-01-02 15:04:05"), Valid: true}
	}

	res, err := h.DB.Exec(`
		UPDATE jira_snapshots
		SET auto_refresh_enabled = ?,
		    auto_refresh_time = ?,
		    next_run_at = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND team_id = ?
	`, body.Enabled, body.Time, nextRunAtVal, snapshotID, teamID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondErr(w, 404, "snapshot not found")
		return
	}

	var actionMsg string
	if body.Enabled {
		actionMsg = fmt.Sprintf("Auto-refresh enabled (Daily at %s) by %s", body.Time, user.Username)
	} else {
		actionMsg = fmt.Sprintf("Auto-refresh disabled by %s", user.Username)
	}
	auditID := uuid.NewString()
	triggerType := "enabled"
	if !body.Enabled {
		triggerType = "disabled"
	}
	_, _ = h.DB.Exec(`
		INSERT INTO jira_auto_refresh_audits (
			id, snapshot_id, team_id, triggered_at, trigger_type, status, message
		) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?, 'skipped', ?)
	`, auditID, snapshotID, teamID, triggerType, actionMsg)

	h.GetSnapshotByID(w, r, teamID, snapshotID)
}

func (h *JiraHandler) DisableAutoRefresh(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	snapshotID := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	msg := "Disabled by " + user.Username
	res, err := h.DB.Exec(`
		UPDATE jira_snapshots
		SET auto_refresh_enabled = 0,
		    last_auto_refresh_msg = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND team_id = ?
	`, msg, snapshotID, teamID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondErr(w, 404, "snapshot not found")
		return
	}

	auditID := uuid.NewString()
	_, _ = h.DB.Exec(`
		INSERT INTO jira_auto_refresh_audits (
			id, snapshot_id, team_id, triggered_at, trigger_type, status, message
		) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'disabled', 'skipped', ?)
	`, auditID, snapshotID, teamID, "Auto-refresh disabled by "+user.Username)

	respond(w, 200, map[string]any{"ok": true, "disabled": true})
}

func (h *JiraHandler) RunAutoRefreshNow(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	snapshotID := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	audit, err := h.ExecuteAutoRefresh(teamID, snapshotID, "manual")
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]any{"ok": true, "audit": audit})
}

func (h *JiraHandler) GetTeamAutoRefreshJobs(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var tName, jiraURL, jiraUser, jiraToken string
	err := h.DB.QueryRow(`SELECT name, jira_url, jira_user, jira_token FROM teams WHERE id = ?`, teamID).
		Scan(&tName, &jiraURL, &jiraUser, &jiraToken)
	if err != nil {
		respondErr(w, 404, "team not found")
		return
	}
	jiraConfigured := (jiraURL != "" && jiraUser != "" && jiraToken != "")

	rows, err := h.DB.Query(`
		SELECT id, team_id, name, start_date, end_date,
		       auto_refresh_enabled, auto_refresh_time, next_run_at,
		       last_auto_refreshed_at, last_auto_refresh_status, last_auto_refresh_msg
		FROM jira_snapshots
		WHERE team_id = ?
		ORDER BY auto_refresh_enabled DESC, created_at DESC
	`, teamID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	now := time.Now()
	var jobs []models.AutoRefreshJobInfo
	for rows.Next() {
		var job models.AutoRefreshJobInfo
		job.TeamID = teamID
		job.TeamName = tName
		job.JiraConfigured = jiraConfigured

		var nextRunAt, lastAutoRefreshedAt sql.NullString
		err := rows.Scan(
			&job.SnapshotID, &job.TeamID, &job.SnapshotName, &job.StartDate, &job.EndDate,
			&job.AutoRefreshEnabled, &job.AutoRefreshTime, &nextRunAt,
			&lastAutoRefreshedAt, &job.LastAutoRefreshStatus, &job.LastAutoRefreshMsg,
		)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		if nextRunAt.Valid {
			job.NextRunAt = parseDBTime(nextRunAt.String)
		}
		if lastAutoRefreshedAt.Valid {
			job.LastAutoRefreshedAt = parseDBTime(lastAutoRefreshedAt.String)
		}
		job.IsActive, job.DateStatus = scheduler.IsSnapshotActive(job.StartDate, job.EndDate, now)
		jobs = append(jobs, job)
	}
	if jobs == nil {
		jobs = []models.AutoRefreshJobInfo{}
	}
	respond(w, 200, jobs)
}

func (h *JiraHandler) GetTeamAutoRefreshAudits(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	snapshotID := r.URL.Query().Get("snapshot_id")
	var rows *sql.Rows
	var err error

	if snapshotID != "" {
		rows, err = h.DB.Query(`
			SELECT a.id, a.snapshot_id, s.name, a.team_id, t.name, a.triggered_at,
			       a.trigger_type, a.status, a.message, a.prev_story_points, a.new_story_points,
			       a.prev_hours_logged, a.new_hours_logged, a.prev_issues_count, a.new_issues_count, a.duration_ms
			FROM jira_auto_refresh_audits a
			JOIN jira_snapshots s ON a.snapshot_id = s.id
			JOIN teams t ON a.team_id = t.id
			WHERE a.team_id = ? AND a.snapshot_id = ?
			ORDER BY a.triggered_at DESC
			LIMIT 100
		`, teamID, snapshotID)
	} else {
		rows, err = h.DB.Query(`
			SELECT a.id, a.snapshot_id, s.name, a.team_id, t.name, a.triggered_at,
			       a.trigger_type, a.status, a.message, a.prev_story_points, a.new_story_points,
			       a.prev_hours_logged, a.new_hours_logged, a.prev_issues_count, a.new_issues_count, a.duration_ms
			FROM jira_auto_refresh_audits a
			JOIN jira_snapshots s ON a.snapshot_id = s.id
			JOIN teams t ON a.team_id = t.id
			WHERE a.team_id = ?
			ORDER BY a.triggered_at DESC
			LIMIT 100
		`, teamID)
	}

	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var audits []models.JiraAutoRefreshAudit
	for rows.Next() {
		var a models.JiraAutoRefreshAudit
		var triggeredAtStr string
		err := rows.Scan(
			&a.ID, &a.SnapshotID, &a.SnapshotName, &a.TeamID, &a.TeamName, &triggeredAtStr,
			&a.TriggerType, &a.Status, &a.Message, &a.PrevStoryPoints, &a.NewStoryPoints,
			&a.PrevHoursLogged, &a.NewHoursLogged, &a.PrevIssuesCount, &a.NewIssuesCount, &a.DurationMs,
		)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		if t := parseDBTime(triggeredAtStr); t != nil {
			a.TriggeredAt = *t
		}
		audits = append(audits, a)
	}
	if audits == nil {
		audits = []models.JiraAutoRefreshAudit{}
	}
	respond(w, 200, audits)
}

func (h *JiraHandler) GetAllAutoRefreshJobs(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT s.id, s.team_id, s.name, t.name, s.start_date, s.end_date,
		       s.auto_refresh_enabled, s.auto_refresh_time, s.next_run_at,
		       s.last_auto_refreshed_at, s.last_auto_refresh_status, s.last_auto_refresh_msg,
		       t.jira_url, t.jira_user, t.jira_token
		FROM jira_snapshots s
		JOIN teams t ON s.team_id = t.id
		ORDER BY s.auto_refresh_enabled DESC, t.name ASC, s.name ASC
	`)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	now := time.Now()
	var jobs []models.AutoRefreshJobInfo
	for rows.Next() {
		var job models.AutoRefreshJobInfo
		var nextRunAt, lastAutoRefreshedAt sql.NullString
		var jURL, jUser, jToken string
		err := rows.Scan(
			&job.SnapshotID, &job.TeamID, &job.SnapshotName, &job.TeamName, &job.StartDate, &job.EndDate,
			&job.AutoRefreshEnabled, &job.AutoRefreshTime, &nextRunAt,
			&lastAutoRefreshedAt, &job.LastAutoRefreshStatus, &job.LastAutoRefreshMsg,
			&jURL, &jUser, &jToken,
		)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		if nextRunAt.Valid {
			job.NextRunAt = parseDBTime(nextRunAt.String)
		}
		if lastAutoRefreshedAt.Valid {
			job.LastAutoRefreshedAt = parseDBTime(lastAutoRefreshedAt.String)
		}
		job.JiraConfigured = (jURL != "" && jUser != "" && jToken != "")
		job.IsActive, job.DateStatus = scheduler.IsSnapshotActive(job.StartDate, job.EndDate, now)
		jobs = append(jobs, job)
	}
	if jobs == nil {
		jobs = []models.AutoRefreshJobInfo{}
	}
	respond(w, 200, jobs)
}

func (h *JiraHandler) GetAllAutoRefreshAudits(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return
	}

	rows, err := h.DB.Query(`
		SELECT a.id, a.snapshot_id, s.name, a.team_id, t.name, a.triggered_at,
		       a.trigger_type, a.status, a.message, a.prev_story_points, a.new_story_points,
		       a.prev_hours_logged, a.new_hours_logged, a.prev_issues_count, a.new_issues_count, a.duration_ms
		FROM jira_auto_refresh_audits a
		JOIN jira_snapshots s ON a.snapshot_id = s.id
		JOIN teams t ON a.team_id = t.id
		ORDER BY a.triggered_at DESC
		LIMIT 200
	`)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var audits []models.JiraAutoRefreshAudit
	for rows.Next() {
		var a models.JiraAutoRefreshAudit
		var triggeredAtStr string
		err := rows.Scan(
			&a.ID, &a.SnapshotID, &a.SnapshotName, &a.TeamID, &a.TeamName, &triggeredAtStr,
			&a.TriggerType, &a.Status, &a.Message, &a.PrevStoryPoints, &a.NewStoryPoints,
			&a.PrevHoursLogged, &a.NewHoursLogged, &a.PrevIssuesCount, &a.NewIssuesCount, &a.DurationMs,
		)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		if t := parseDBTime(triggeredAtStr); t != nil {
			a.TriggeredAt = *t
		}
		audits = append(audits, a)
	}
	if audits == nil {
		audits = []models.JiraAutoRefreshAudit{}
	}
	respond(w, 200, audits)
}

func (h *JiraHandler) SuperAdminDisableAutoRefresh(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return
	}

	snapshotID := chi.URLParam(r, "snapshotID")
	var teamID, snapName string
	err := h.DB.QueryRow(`SELECT team_id, name FROM jira_snapshots WHERE id = ?`, snapshotID).Scan(&teamID, &snapName)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "snapshot not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	msg := "Auto-refresh disabled by superadmin " + user.Username
	_, err = h.DB.Exec(`
		UPDATE jira_snapshots
		SET auto_refresh_enabled = 0,
		    last_auto_refresh_msg = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, msg, snapshotID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	auditID := uuid.NewString()
	_, _ = h.DB.Exec(`
		INSERT INTO jira_auto_refresh_audits (
			id, snapshot_id, team_id, triggered_at, trigger_type, status, message
		) VALUES (?, ?, ?, CURRENT_TIMESTAMP, 'disabled', 'skipped', ?)
	`, auditID, snapshotID, teamID, msg)

	respond(w, 200, map[string]any{"ok": true, "disabled": true, "snapshot_id": snapshotID})
}

func (h *JiraHandler) SuperAdminRunAutoRefresh(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return
	}

	snapshotID := chi.URLParam(r, "snapshotID")
	var teamID string
	err := h.DB.QueryRow(`SELECT team_id FROM jira_snapshots WHERE id = ?`, snapshotID).Scan(&teamID)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "snapshot not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	audit, err := h.ExecuteAutoRefresh(teamID, snapshotID, "manual")
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]any{"ok": true, "audit": audit})
}

