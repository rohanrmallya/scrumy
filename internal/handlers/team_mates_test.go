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

func TestTeamMates_CRUDAndAutoLink(t *testing.T) {
	database, authH, teamsH, _ := setupTestDB(t)

	// Create Team
	teamID := "team-mate-test-1"
	_, err := database.Exec(`INSERT INTO teams (id, name) VALUES (?, ?)`, teamID, "Engineering Alpha")
	if err != nil {
		t.Fatalf("insert team failed: %v", err)
	}

	// Create Admin User
	adminUser := &models.User{ID: "admin-u1", Username: "lead", Role: "user"}
	_, _ = database.Exec(`INSERT INTO users (id, username, password_hash, role, email) VALUES (?, ?, ?, ?, ?)`,
		adminUser.ID, adminUser.Username, "hash", adminUser.Role, "lead@company.com")
	_, _ = database.Exec(`INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)`, teamID, adminUser.ID)

	// Create a regular Scrumy user with email alice@company.com
	registeredAlice := &models.User{ID: "user-alice-1", Username: "alice_cooper", Role: "user", Email: "alice@company.com"}
	_, _ = database.Exec(`INSERT INTO users (id, username, password_hash, role, email) VALUES (?, ?, ?, ?, ?)`,
		registeredAlice.ID, registeredAlice.Username, "hash", registeredAlice.Role, registeredAlice.Email)

	// Helper to make authenticated chi requests
	makeReq := func(method, path string, body any, urlParams map[string]string) *httptest.ResponseRecorder {
		var bodyReader *bytes.Reader
		if body != nil {
			b, _ := json.Marshal(body)
			bodyReader = bytes.NewReader(b)
		} else {
			bodyReader = bytes.NewReader([]byte{})
		}
		req := httptest.NewRequest(method, path, bodyReader)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, adminUser))
		rctx := chi.NewRouteContext()
		for k, v := range urlParams {
			rctx.URLParams.Add(k, v)
		}
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		rec := httptest.NewRecorder()
		return rec
	}

	// 1. Add Team Mate Alice (matching registered user email alice@company.com)
	rec := makeReq("POST", "/teams/"+teamID+"/mates", map[string]any{
		"name":              "Alice Cooper",
		"email":             "alice@company.com",
		"jira_account_id":   "jira-acc-101",
		"role":              "Frontend Dev",
	}, map[string]string{"teamID": teamID})

	req := httptest.NewRequest("POST", "/teams/"+teamID+"/mates", bytes.NewReader([]byte(`{"name":"Alice Cooper","email":"alice@company.com","jira_account_id":"jira-acc-101","role":"Frontend Dev"}`)))
	req = req.WithContext(context.WithValue(req.Context(), userContextKey, adminUser))
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("teamID", teamID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	rec = httptest.NewRecorder()

	teamsH.AddMate(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("AddMate expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var addedAlice models.TeamMate
	if err := json.NewDecoder(rec.Body).Decode(&addedAlice); err != nil {
		t.Fatalf("decode added alice: %v", err)
	}
	if addedAlice.Name != "Alice Cooper" {
		t.Errorf("expected Alice Cooper, got %s", addedAlice.Name)
	}
	if addedAlice.UserID == nil || *addedAlice.UserID != registeredAlice.ID {
		t.Errorf("expected auto-linked user_id %s, got %v", registeredAlice.ID, addedAlice.UserID)
	}
	if addedAlice.Username != registeredAlice.Username {
		t.Errorf("expected username %s, got %s", registeredAlice.Username, addedAlice.Username)
	}

	// 2. Try adding Alice again -> expect 409 Conflict
	reqDup := httptest.NewRequest("POST", "/teams/"+teamID+"/mates", bytes.NewReader([]byte(`{"name":"Alice Cooper"}`)))
	reqDup = reqDup.WithContext(context.WithValue(reqDup.Context(), userContextKey, adminUser))
	rctxDup := chi.NewRouteContext()
	rctxDup.URLParams.Add("teamID", teamID)
	reqDup = reqDup.WithContext(context.WithValue(reqDup.Context(), chi.RouteCtxKey, rctxDup))
	recDup := httptest.NewRecorder()
	teamsH.AddMate(recDup, reqDup)
	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d: %s", recDup.Code, recDup.Body.String())
	}

	// 3. Bulk Add Bob and Charlie
	bulkReq := httptest.NewRequest("POST", "/teams/"+teamID+"/mates/bulk", bytes.NewReader([]byte(`[
		{"name":"Bob Marley", "email":"bob@company.com", "jira_account_id":"jira-bob-102", "role":"Backend Dev"},
		{"name":"Charlie Brown", "email":"charlie@company.com", "jira_account_id":"jira-charlie-103", "role":"QA"}
	]`)))
	bulkReq = bulkReq.WithContext(context.WithValue(bulkReq.Context(), userContextKey, adminUser))
	rctxBulk := chi.NewRouteContext()
	rctxBulk.URLParams.Add("teamID", teamID)
	bulkReq = bulkReq.WithContext(context.WithValue(bulkReq.Context(), chi.RouteCtxKey, rctxBulk))
	recBulk := httptest.NewRecorder()
	teamsH.BulkAddMates(recBulk, bulkReq)
	if recBulk.Code != http.StatusOK {
		t.Fatalf("expected 200 for bulk add, got %d: %s", recBulk.Code, recBulk.Body.String())
	}

	// 4. List Mates
	listReq := httptest.NewRequest("GET", "/teams/"+teamID+"/mates", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), userContextKey, adminUser))
	rctxList := chi.NewRouteContext()
	rctxList.URLParams.Add("teamID", teamID)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), chi.RouteCtxKey, rctxList))
	recList := httptest.NewRecorder()
	teamsH.ListMates(recList, listReq)
	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 for list mates, got %d: %s", recList.Code, recList.Body.String())
	}
	var mates []models.TeamMate
	json.NewDecoder(recList.Body).Decode(&mates)
	if len(mates) != 3 {
		t.Fatalf("expected 3 mates, got %d", len(mates))
	}

	// 5. Update Bob's role
	var bobID string
	for _, m := range mates {
		if m.Name == "Bob Marley" {
			bobID = m.ID
		}
	}
	upReq := httptest.NewRequest("PUT", "/teams/"+teamID+"/mates/"+bobID, bytes.NewReader([]byte(`{"name":"Bob Marley","email":"bob@company.com","role":"Tech Lead"}`)))
	upReq = upReq.WithContext(context.WithValue(upReq.Context(), userContextKey, adminUser))
	rctxUp := chi.NewRouteContext()
	rctxUp.URLParams.Add("teamID", teamID)
	rctxUp.URLParams.Add("mateID", bobID)
	upReq = upReq.WithContext(context.WithValue(upReq.Context(), chi.RouteCtxKey, rctxUp))
	recUp := httptest.NewRecorder()
	teamsH.UpdateMate(recUp, upReq)
	if recUp.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d: %s", recUp.Code, recUp.Body.String())
	}
	var updatedBob models.TeamMate
	json.NewDecoder(recUp.Body).Decode(&updatedBob)
	if updatedBob.Role != "Tech Lead" {
		t.Errorf("expected Tech Lead, got %s", updatedBob.Role)
	}

	// 6. Delete Charlie
	var charlieID string
	for _, m := range mates {
		if m.Name == "Charlie Brown" {
			charlieID = m.ID
		}
	}
	delReq := httptest.NewRequest("DELETE", "/teams/"+teamID+"/mates/"+charlieID, nil)
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), userContextKey, adminUser))
	rctxDel := chi.NewRouteContext()
	rctxDel.URLParams.Add("teamID", teamID)
	rctxDel.URLParams.Add("mateID", charlieID)
	delReq = delReq.WithContext(context.WithValue(delReq.Context(), chi.RouteCtxKey, rctxDel))
	recDel := httptest.NewRecorder()
	teamsH.DeleteMate(recDel, delReq)
	if recDel.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d: %s", recDel.Code, recDel.Body.String())
	}

	// 7. Verify Auth UpdateProfile links existing mates
	// Suppose a mate existed with dave@company.com before Dave registered
	_, _ = database.Exec(`INSERT INTO team_mates (id, team_id, name, email) VALUES (?, ?, ?, ?)`,
		"mate-dave", teamID, "Dave Smith", "dave@company.com")

	// Now a new user registers or updates profile to dave@company.com
	daveUser := &models.User{ID: "user-dave", Username: "davesmith", Role: "user"}
	_, _ = database.Exec(`INSERT INTO users (id, username, password_hash, role, email) VALUES (?, ?, ?, ?, ?)`,
		daveUser.ID, daveUser.Username, "hash", daveUser.Role, "")

	profReq := httptest.NewRequest("PUT", "/auth/profile", bytes.NewReader([]byte(`{"email":"dave@company.com"}`)))
	profReq = profReq.WithContext(context.WithValue(profReq.Context(), userContextKey, daveUser))
	recProf := httptest.NewRecorder()
	authH.UpdateProfile(recProf, profReq)
	if recProf.Code != http.StatusOK {
		t.Fatalf("expected 200 for update profile, got %d: %s", recProf.Code, recProf.Body.String())
	}

	// Check if Dave's team_mate record was auto-linked to user-dave
	var linkedUserID string
	err = database.QueryRow("SELECT user_id FROM team_mates WHERE id = 'mate-dave'").Scan(&linkedUserID)
	if err != nil || linkedUserID != daveUser.ID {
		t.Errorf("expected mate-dave to be auto-linked to %s, got %s (err: %v)", daveUser.ID, linkedUserID, err)
	}
}

func TestSnapshotRefresh_ZeroLogMembers(t *testing.T) {
	database, _, _, jiraH := setupTestDB(t)

	// Create Team with mock Jira URL
	teamID := "team-snap-refresh-1"
	_, _ = database.Exec(`
		INSERT INTO teams (id, name, jira_url, jira_user, jira_token)
		VALUES (?, ?, 'mock', 'user', 'tok')
	`, teamID, "Beta Team")

	adminUser := &models.User{ID: "admin-u2", Username: "scrummaster", Role: "user"}
	_, _ = database.Exec(`INSERT INTO users (id, username, password_hash, role) VALUES (?, ?, 'hash', 'user')`, adminUser.ID, adminUser.Username)
	_, _ = database.Exec(`INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)`, teamID, adminUser.ID)

	// Add team mates: Alice Cooper (who is in mock data), and David NonLogger (who is NOT in mock data)
	_, _ = database.Exec(`
		INSERT INTO team_mates (id, team_id, name, email, jira_account_id)
		VALUES ('mate-alice', ?, 'Alice Cooper', 'alice@example.com', 'jira-alice-101'),
		       ('mate-david', ?, 'David NonLogger', 'david@example.com', 'jira-david-999')
	`, teamID, teamID)

	// Create snapshot
	snapID := "snap-refresh-test-1"
	_, _ = database.Exec(`
		INSERT INTO jira_snapshots (id, team_id, name, start_date, end_date, all_worklogs, data)
		VALUES (?, ?, 'Sprint 1', '2026-06-01', '2026-06-15', 0, '{}')
	`, snapID, teamID)

	// Refresh snapshot
	refReq := httptest.NewRequest("POST", "/teams/"+teamID+"/jira/snapshots/"+snapID+"/refresh", nil)
	refReq = refReq.WithContext(context.WithValue(refReq.Context(), userContextKey, adminUser))
	rctxRef := chi.NewRouteContext()
	rctxRef.URLParams.Add("teamID", teamID)
	rctxRef.URLParams.Add("snapshotID", snapID)
	refReq = refReq.WithContext(context.WithValue(refReq.Context(), chi.RouteCtxKey, rctxRef))
	recRef := httptest.NewRecorder()
	jiraH.RefreshSnapshot(recRef, refReq)
	if recRef.Code != http.StatusOK {
		t.Fatalf("expected 200 for refresh, got %d: %s", recRef.Code, recRef.Body.String())
	}

	var snap models.JiraSnapshot
	if err := json.NewDecoder(recRef.Body).Decode(&snap); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}

	if len(snap.RefreshHistory) == 0 {
		t.Fatalf("expected refresh history, got 0")
	}
	latestHistory := snap.RefreshHistory[0]

	// David NonLogger should be detected as having logged 0 hours!
	foundDavid := false
	for _, z := range latestHistory.ZeroLogMembers {
		if z.Name == "David NonLogger" {
			foundDavid = true
			if z.TotalHours != 0 {
				t.Errorf("expected 0 total hours for David, got %f", z.TotalHours)
			}
		}
		if z.Name == "Alice Cooper" {
			t.Errorf("Alice Cooper logged hours, should NOT be in ZeroLogMembers")
		}
	}
	if !foundDavid {
		t.Errorf("David NonLogger was not found in ZeroLogMembers: %+v", latestHistory.ZeroLogMembers)
	}
}
