package handlers

import (
	"database/sql"
	"net/http"
	"scrumy/internal/db"
	"scrumy/internal/models"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type TeamsHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

// PlansHandler is a deprecated alias for TeamsHandler, kept for backward compatibility.
type PlansHandler = TeamsHandler

// teamIDParam reads the team ID from the URL, accepting both the current
// {teamID} param and the legacy {planID} param used by deprecated /plans routes.
func teamIDParam(r *http.Request) string {
	if id := chi.URLParam(r, "teamID"); id != "" {
		return id
	}
	return chi.URLParam(r, "planID")
}

func (h *TeamsHandler) List(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	rows, err := h.DB.Query(`
		SELECT t.id, t.name, t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM capacity_plans cp WHERE cp.team_id = t.id) as capacity_count,
			(SELECT COUNT(*) FROM presentations pr WHERE pr.team_id = t.id) as presentation_count,
			EXISTS(SELECT 1 FROM team_admins ta WHERE ta.team_id = t.id AND ta.user_id = ?) as is_admin
		FROM teams t ORDER BY t.created_at DESC
	`, userID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	isSuper := IsSuperAdmin(user)

	var teams []models.Team
	for rows.Next() {
		var t models.Team
		var createdAt, updatedAt string
		if err := rows.Scan(&t.ID, &t.Name, &createdAt, &updatedAt, &t.CapacityPlanCount, &t.PresentationCount, &t.IsAdmin); err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		if isSuper {
			t.IsAdmin = true
		}
		t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		teams = append(teams, t)
	}
	if teams == nil {
		teams = []models.Team{}
	}
	respond(w, 200, teams)
}

func (h *TeamsHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil {
		respondErr(w, http.StatusUnauthorized, "authentication required")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := decode(r, &body); err != nil || body.Name == "" {
		respondErr(w, 400, "name is required")
		return
	}
	id := uuid.NewString()
	tx, err := h.DB.Begin()
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer tx.Rollback()

	_, err = tx.Exec(`INSERT INTO teams (id, name) VALUES (?, ?)`, id, body.Name)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	_, err = tx.Exec(`INSERT INTO team_admins (team_id, user_id) VALUES (?, ?)`, id, user.ID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	h.GetByID(w, r, id)
}

func (h *TeamsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := teamIDParam(r)
	h.GetByID(w, r, id)
}

func (h *TeamsHandler) GetByID(w http.ResponseWriter, r *http.Request, id string) {
	user := h.Auth.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var t models.Team
	var createdAt, updatedAt string
	var jiraToken string
	err := h.DB.QueryRow(`
		SELECT t.id, t.name, t.created_at, t.updated_at,
			(SELECT COUNT(*) FROM capacity_plans cp WHERE cp.team_id = t.id),
			(SELECT COUNT(*) FROM presentations pr WHERE pr.team_id = t.id),
			EXISTS(SELECT 1 FROM team_admins ta WHERE ta.team_id = t.id AND ta.user_id = ?),
			t.jira_url, t.jira_user, t.jira_token, t.jira_jql, t.jira_sp_field, t.jira_insecure
		FROM teams t WHERE t.id = ?
	`, userID, id).Scan(
		&t.ID, &t.Name, &createdAt, &updatedAt, &t.CapacityPlanCount, &t.PresentationCount, &t.IsAdmin,
		&t.JiraURL, &t.JiraUser, &jiraToken, &t.JiraJQL, &t.JiraSPField, &t.JiraInsecure,
	)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "team not found")
		return
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	if IsSuperAdmin(user) {
		t.IsAdmin = true
	}
	t.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	t.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	t.JiraTokenSet = (jiraToken != "")
	t.JiraToken = ""

	// Load admins
	adminRows, err := h.DB.Query(`
		SELECT u.username FROM users u
		JOIN team_admins ta ON u.id = ta.user_id
		WHERE ta.team_id = ?
	`, id)
	if err == nil {
		for adminRows.Next() {
			var username string
			adminRows.Scan(&username)
			t.Admins = append(t.Admins, username)
		}
		adminRows.Close()
	}

	respond(w, 200, t)
}

func (h *TeamsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, id) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM teams WHERE id = ?`, id)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondErr(w, 404, "team not found")
		return
	}
	respond(w, 200, map[string]bool{"deleted": true})
}

func (h *TeamsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, id) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := decode(r, &body); err != nil || body.Name == "" {
		respondErr(w, 400, "name is required")
		return
	}
	_, err := h.DB.Exec(`UPDATE teams SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, body.Name, id)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.GetByID(w, r, id)
}

func (h *TeamsHandler) addAdminToAllTeamsHelper(userID string) (int, int, error) {
	rows, err := h.DB.Query("SELECT id FROM teams")
	if err != nil {
		return 0, 0, err
	}
	defer rows.Close()

	var teamIDs []string
	for rows.Next() {
		var tid string
		if err := rows.Scan(&tid); err == nil {
			teamIDs = append(teamIDs, tid)
		}
	}

	if len(teamIDs) == 0 {
		return 0, 0, nil
	}

	tx, err := h.DB.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare("INSERT OR IGNORE INTO team_admins (team_id, user_id) VALUES (?, ?)")
	if err != nil {
		return 0, 0, err
	}
	defer stmt.Close()

	addedCount := 0
	for _, tid := range teamIDs {
		res, err := stmt.Exec(tid, userID)
		if err == nil {
			if affected, _ := res.RowsAffected(); affected > 0 {
				addedCount++
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	return len(teamIDs), addedCount, nil
}

func (h *TeamsHandler) AddAdminToAllTeams(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !IsSuperAdmin(user) {
		respondErr(w, http.StatusForbidden, "super admin required")
		return
	}

	var body struct {
		Username string `json:"username"`
	}
	if err := decode(r, &body); err != nil || strings.TrimSpace(body.Username) == "" {
		respondErr(w, 400, "username is required")
		return
	}
	username := strings.TrimSpace(body.Username)

	var targetID, targetUsername string
	err := h.DB.QueryRow("SELECT id, username FROM users WHERE username = ?", username).Scan(&targetID, &targetUsername)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "user not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	total, added, err := h.addAdminToAllTeamsHelper(targetID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	respond(w, 200, map[string]any{
		"username":    targetUsername,
		"user_id":     targetID,
		"total_teams": total,
		"added":       added,
	})
}

func (h *TeamsHandler) AddAdmin(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Username string `json:"username"`
		AllTeams bool   `json:"all_teams"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}

	var userID string
	err := h.DB.QueryRow("SELECT id FROM users WHERE username = ?", body.Username).Scan(&userID)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "user not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	if body.AllTeams && IsSuperAdmin(user) {
		total, added, err := h.addAdminToAllTeamsHelper(userID)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		respond(w, 200, map[string]any{"added": true, "total_teams": total, "added_count": added})
		return
	}

	_, err = h.DB.Exec("INSERT OR IGNORE INTO team_admins (team_id, user_id) VALUES (?, ?)", teamID, userID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]bool{"added": true})
}

func (h *TeamsHandler) RemoveAdmin(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsTeamAdmin(user.ID, teamID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Username string `json:"username"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}

	var userID string
	err := h.DB.QueryRow("SELECT id FROM users WHERE username = ?", body.Username).Scan(&userID)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "user not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	// Prevent removing last admin? Requirement says Admin owns teams.
	var count int
	h.DB.QueryRow("SELECT COUNT(*) FROM team_admins WHERE team_id = ?", teamID).Scan(&count)
	if count <= 1 {
		respondErr(w, 400, "cannot remove the last admin")
		return
	}

	_, err = h.DB.Exec("DELETE FROM team_admins WHERE team_id = ? AND user_id = ?", teamID, userID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]bool{"removed": true})
}
