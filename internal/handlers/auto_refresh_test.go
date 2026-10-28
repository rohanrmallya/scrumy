package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"scrumy/internal/models"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestAutoRefreshSettingsAndDisable(t *testing.T) {
	database, _, _, jiraH := setupTestDB(t)

	// Create a team
	teamID := "team-auto-1"
	_, err := database.Exec(`INSERT INTO teams (id, name) VALUES (?, ?)`, teamID, "Beta Team")
	if err != nil {
		t.Fatalf("insert team: %v", err)
	}

	// Create team admin
	adminUser := &models.User{ID: "user-admin-1", Username: "alicia", Role: "user"}
	_, _ = database.Exec(`INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, ?, ?)`, adminUser.ID, adminUser.Username, "hash", adminUser.Role)
	_, _ = database.Exec(`INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)`, teamID, adminUser.ID)

	// Create snapshot
	snapID := "snap-auto-1"
	_, err = database.Exec(`
		INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, auto_refresh_enabled, auto_refresh_time)
		VALUES (?, ?, ?, ?, ?, 0, '09:00')
	`, snapID, teamID, "Sprint 10", "2026-10-01", "2026-10-31")
	if err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}

	// 1. Enable auto refresh via UpdateAutoRefreshSettings
	body := map[string]any{
		"auto_refresh_enabled": true,
		"auto_refresh_time":    "14:30",
	}
	bodyBytes, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", "/teams/"+teamID+"/jira/snapshots/"+snapID+"/auto-refresh", bytes.NewReader(bodyBytes))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, adminUser))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("teamID", teamID)
	rctx.URLParams.Add("snapshotID", snapID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec := httptest.NewRecorder()

	jiraH.UpdateAutoRefreshSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify database was updated
	var enabled bool
	var rTime, nextRun string
	_ = database.QueryRow(`SELECT auto_refresh_enabled, auto_refresh_time, next_run_at FROM jira_snapshots WHERE id = ?`, snapID).
		Scan(&enabled, &rTime, &nextRun)
	if !enabled {
		t.Errorf("expected auto_refresh_enabled=true")
	}
	if rTime != "14:30" {
		t.Errorf("expected auto_refresh_time='14:30', got %q", rTime)
	}
	if nextRun == "" {
		t.Errorf("expected next_run_at to be populated")
	}

	// Verify audit was logged
	var auditCount int
	_ = database.QueryRow(`SELECT COUNT(*) FROM jira_auto_refresh_audits WHERE snapshot_id = ? AND trigger_type = 'enabled'`, snapID).Scan(&auditCount)
	if auditCount != 1 {
		t.Errorf("expected 1 audit log with trigger_type='enabled', got %d", auditCount)
	}

	// 2. Disable auto refresh via DisableAutoRefresh
	reqDis := httptest.NewRequest("POST", "/teams/"+teamID+"/jira/snapshots/"+snapID+"/auto-refresh/disable", nil)
	reqDis = reqDis.WithContext(context.WithValue(reqDis.Context(), userContextKey, adminUser))
	reqDis = reqDis.WithContext(context.WithValue(reqDis.Context(), chi.RouteCtxKey, rctx))
	recDis := httptest.NewRecorder()

	jiraH.DisableAutoRefresh(recDis, reqDis)
	if recDis.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recDis.Code, recDis.Body.String())
	}

	_ = database.QueryRow(`SELECT auto_refresh_enabled FROM jira_snapshots WHERE id = ?`, snapID).Scan(&enabled)
	if enabled {
		t.Errorf("expected auto_refresh_enabled=false after disable")
	}

	// Verify disable audit was logged
	var disAuditCount int
	_ = database.QueryRow(`SELECT COUNT(*) FROM jira_auto_refresh_audits WHERE snapshot_id = ? AND trigger_type = 'disabled'`, snapID).Scan(&disAuditCount)
	if disAuditCount != 1 {
		t.Errorf("expected 1 audit log with trigger_type='disabled', got %d", disAuditCount)
	}
}

func TestAutoRefreshActiveDateEnforcement(t *testing.T) {
	database, _, _, jiraH := setupTestDB(t)

	teamID := "team-dates-1"
	_, _ = database.Exec(`INSERT INTO teams (id, name) VALUES (?, ?)`, teamID, "Dates Team")

	// Snapshot whose active window ended in the past (Expired)
	pastSnapID := "snap-past"
	_, _ = database.Exec(`
		INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, auto_refresh_enabled, auto_refresh_time)
		VALUES (?, ?, ?, ?, ?, 1, '09:00')
	`, pastSnapID, teamID, "Old Sprint", "2020-01-01", "2020-01-15")

	// Execute scheduled auto-refresh on expired snapshot
	audit, err := jiraH.ExecuteAutoRefresh(teamID, pastSnapID, "scheduled")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if audit.Status != "skipped" {
		t.Errorf("expected audit status 'skipped' for expired snapshot, got %q", audit.Status)
	}

	// Verify database status reflects skipped
	var status, msg string
	_ = database.QueryRow(`SELECT last_auto_refresh_status, last_auto_refresh_msg FROM jira_snapshots WHERE id = ?`, pastSnapID).
		Scan(&status, &msg)
	if status != "skipped" {
		t.Errorf("expected snapshot last_auto_refresh_status='skipped', got %q", status)
	}
}

func TestSuperAdminAutoRefreshViews(t *testing.T) {
	database, _, _, jiraH := setupTestDB(t)

	// Setup team and snapshot with auto-refresh
	teamID := "team-sa-1"
	_, _ = database.Exec(`INSERT INTO teams (id, name, jira_url, jira_user, jira_token) VALUES (?, ?, 'https://jira.example.com', 'user', 'tok')`, teamID, "Gamma Team")

	snapID := "snap-sa-1"
	_, _ = database.Exec(`
		INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, auto_refresh_enabled, auto_refresh_time)
		VALUES (?, ?, ?, ?, ?, 1, '10:00')
	`, snapID, teamID, "Current Sprint", "2026-10-01", "2026-10-31")

	// Superadmin user
	superUser := &models.User{ID: "root-user", Username: "root", Role: "admin"}
	regularUser := &models.User{ID: "reg-user", Username: "bob", Role: "user"}

	// 1. Regular user should be forbidden
	reqReg := httptest.NewRequest("GET", "/api/admin/auto-refresh/jobs", nil)
	reqReg = reqReg.WithContext(context.WithValue(reqReg.Context(), userContextKey, regularUser))
	recReg := httptest.NewRecorder()
	jiraH.GetAllAutoRefreshJobs(recReg, reqReg)
	if recReg.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for regular user, got %d", recReg.Code)
	}

	// 2. Superadmin can list all auto-refresh jobs across projects
	reqSA := httptest.NewRequest("GET", "/api/admin/auto-refresh/jobs", nil)
	reqSA = reqSA.WithContext(context.WithValue(reqSA.Context(), userContextKey, superUser))
	recSA := httptest.NewRecorder()
	jiraH.GetAllAutoRefreshJobs(recSA, reqSA)
	if recSA.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recSA.Code, recSA.Body.String())
	}

	var jobs []models.AutoRefreshJobInfo
	if err := json.Unmarshal(recSA.Body.Bytes(), &jobs); err != nil {
		t.Fatalf("unmarshal jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].SnapshotID != snapID || jobs[0].TeamName != "Gamma Team" {
		t.Errorf("job info mismatch: %+v", jobs[0])
	}
	if !jobs[0].JiraConfigured {
		t.Errorf("expected jira_configured=true")
	}

	// 3. Superadmin can disable any snapshot's auto refresh
	reqDis := httptest.NewRequest("POST", "/api/admin/snapshots/"+snapID+"/auto-refresh/disable", nil)
	reqDis = reqDis.WithContext(context.WithValue(reqDis.Context(), userContextKey, superUser))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("snapshotID", snapID)
	reqDis = reqDis.WithContext(context.WithValue(reqDis.Context(), chi.RouteCtxKey, rctx))
	recDis := httptest.NewRecorder()

	jiraH.SuperAdminDisableAutoRefresh(recDis, reqDis)
	if recDis.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recDis.Code, recDis.Body.String())
	}

	var enabled bool
	_ = database.QueryRow(`SELECT auto_refresh_enabled FROM jira_snapshots WHERE id = ?`, snapID).Scan(&enabled)
	if enabled {
		t.Errorf("expected superadmin disable to set auto_refresh_enabled=false")
	}

	// 4. Superadmin can query all audits
	reqAud := httptest.NewRequest("GET", "/api/admin/auto-refresh/audits", nil)
	reqAud = reqAud.WithContext(context.WithValue(reqAud.Context(), userContextKey, superUser))
	recAud := httptest.NewRecorder()
	jiraH.GetAllAutoRefreshAudits(recAud, reqAud)
	if recAud.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recAud.Code, recAud.Body.String())
	}

	var audits []models.JiraAutoRefreshAudit
	if err := json.Unmarshal(recAud.Body.Bytes(), &audits); err != nil {
		t.Fatalf("unmarshal audits: %v", err)
	}
	if len(audits) < 1 {
		t.Fatalf("expected at least 1 audit from superadmin disable action, got %d", len(audits))
	}
	if audits[0].TriggerType != "disabled" {
		t.Errorf("expected audit trigger_type='disabled', got %s", audits[0].TriggerType)
	}
}
