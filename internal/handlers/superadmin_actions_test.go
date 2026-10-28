package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"scrumy/internal/db"
	"scrumy/internal/models"
)

func setupTestDB(t *testing.T) (*db.DB, *AuthHandler, *TeamsHandler, *JiraHandler) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	authH := &AuthHandler{DB: database}
	teamsH := &TeamsHandler{DB: database, Auth: authH}
	jiraH := &JiraHandler{DB: database, Auth: authH}

	return database, authH, teamsH, jiraH
}

func TestSuperadminIsTeamAdmin(t *testing.T) {
	database, authH, _, _ := setupTestDB(t)

	// Create a team
	teamID := "team-1"
	if _, err := database.Exec("INSERT INTO teams (id, name) VALUES (?, ?)", teamID, "Alpha"); err != nil {
		t.Fatalf("insert team: %v", err)
	}

	// Create users:
	// 1. regular user
	// 2. team admin user
	// 3. global admin (role = admin)
	// 4. superuser by username (username = root)
	regularID := "user-reg"
	teamAdminID := "user-ta"
	superAdminRoleID := "user-sa-role"
	var rootID string
	_ = database.QueryRow("SELECT id FROM users WHERE username = 'root'").Scan(&rootID)

	_, _ = database.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)", regularID, "regular", "hash", "user")
	_, _ = database.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)", teamAdminID, "teamadmin", "hash", "user")
	_, _ = database.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)", superAdminRoleID, "adminuser", "hash", "admin")

	_, _ = database.Exec("INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)", teamID, teamAdminID)

	if authH.IsTeamAdmin(regularID, teamID) {
		t.Errorf("expected regular user to NOT be team admin")
	}
	if !authH.IsTeamAdmin(teamAdminID, teamID) {
		t.Errorf("expected team admin user to be team admin")
	}
	if !authH.IsTeamAdmin(superAdminRoleID, teamID) {
		t.Errorf("expected global admin to be treated as team admin")
	}
	if !authH.IsTeamAdmin(rootID, teamID) {
		t.Errorf("expected superadmin by username to be treated as team admin")
	}
}

func TestAddAdminToAllTeams(t *testing.T) {
	database, _, teamsH, _ := setupTestDB(t)

	// Create 2 teams
	_, _ = database.Exec("INSERT INTO teams (id, name) VALUES (?, ?)", "t1", "Team 1")
	_, _ = database.Exec("INSERT INTO teams (id, name) VALUES (?, ?)", "t2", "Team 2")

	// Create target user
	targetID := "user-target"
	_, _ = database.Exec("INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)", targetID, "bob", "hash", "user")

	// Already admin of t1
	_, _ = database.Exec("INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)", "t1", targetID)

	// Superadmin requester
	adminUser := &models.User{ID: "admin-1", Username: "root", Role: "admin"}

	reqBody, _ := json.Marshal(map[string]string{"username": "bob"})
	req := httptest.NewRequest("POST", "/api/admin/teams/add-admin", bytes.NewReader(reqBody))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, adminUser))
	rec := httptest.NewRecorder()

	teamsH.AddAdminToAllTeams(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Username   string `json:"username"`
		TotalTeams int    `json:"total_teams"`
		Added      int    `json:"added"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.TotalTeams != 2 {
		t.Errorf("expected 2 total teams, got %d", resp.TotalTeams)
	}
	if resp.Added != 1 {
		t.Errorf("expected 1 added team, got %d", resp.Added)
	}

	// Verify bob is now admin of both t1 and t2
	var c1, c2 int
	_ = database.QueryRow("SELECT COUNT(*) FROM team_admins WHERE team_id = 't1' AND user_id = ?", targetID).Scan(&c1)
	_ = database.QueryRow("SELECT COUNT(*) FROM team_admins WHERE team_id = 't2' AND user_id = ?", targetID).Scan(&c2)
	if c1 != 1 || c2 != 1 {
		t.Errorf("expected bob to be admin in both teams: t1=%d, t2=%d", c1, c2)
	}
}

func TestRefreshAllSnapshots(t *testing.T) {
	database, _, _, jiraH := setupTestDB(t)

	// Team 1 with mock Jira
	_, _ = database.Exec(`
		INSERT INTO teams (id, name, jira_url, jira_user, jira_token, jira_jql, jira_sp_field) 
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "t1", "Mock Team", "mock", "user@test.com", "token", "project = TEST", "customfield_10001")

	// Team 2 with no Jira
	_, _ = database.Exec(`
		INSERT INTO teams (id, name) VALUES (?, ?)
	`, "t2", "Unconfigured Team")

	// Snapshots
	_, _ = database.Exec(`
		INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, all_worklogs, data)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "s1", "t1", "Sprint 1", "2026-05-01", "2026-05-15", 0, "{}")

	_, _ = database.Exec(`
		INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, all_worklogs, data)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, "s2", "t2", "Sprint 2", "2026-05-01", "2026-05-15", 0, "{}")

	adminUser := &models.User{ID: "admin-1", Username: "root", Role: "admin"}

	req := httptest.NewRequest("POST", "/api/admin/snapshots/refresh-all", nil)
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, adminUser))
	rec := httptest.NewRecorder()

	jiraH.RefreshAllSnapshots(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp BulkRefreshResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}

	if resp.Total != 2 {
		t.Errorf("expected 2 total snapshots, got %d", resp.Total)
	}
	if resp.Refreshed != 1 {
		t.Errorf("expected 1 refreshed snapshot, got %d", resp.Refreshed)
	}
	if resp.Skipped != 1 {
		t.Errorf("expected 1 skipped snapshot, got %d", resp.Skipped)
	}
}
