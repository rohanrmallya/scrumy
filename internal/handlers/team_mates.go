package handlers

import (
	"database/sql"
	"net/http"
	"scrumy/internal/models"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func (h *TeamsHandler) findUserIDForMate(email, jiraAccountID string) *string {
	email = strings.TrimSpace(email)
	jiraAccountID = strings.TrimSpace(jiraAccountID)

	var userID string
	if email != "" {
		err := h.DB.QueryRow("SELECT id FROM users WHERE LOWER(email) = LOWER(?) LIMIT 1", email).Scan(&userID)
		if err == nil && userID != "" {
			return &userID
		}
	}
	if jiraAccountID != "" {
		err := h.DB.QueryRow("SELECT id FROM users WHERE jira_account_id = ? AND jira_account_id != '' LIMIT 1", jiraAccountID).Scan(&userID)
		if err == nil && userID != "" {
			return &userID
		}
	}
	return nil
}

func (h *TeamsHandler) ListMates(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)

	rows, err := h.DB.Query(`
		SELECT tm.id, tm.team_id, tm.user_id, COALESCE(u.username, '') as username,
		       tm.name, tm.email, tm.jira_account_id, tm.role, tm.avatar_url,
		       tm.created_at, tm.updated_at
		FROM team_mates tm
		LEFT JOIN users u ON tm.user_id = u.id
		WHERE tm.team_id = ?
		ORDER BY tm.name ASC
	`, teamID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()

	mates := []models.TeamMate{}
	for rows.Next() {
		var m models.TeamMate
		var userID sql.NullString
		var username, createdAt, updatedAt string
		if err := rows.Scan(
			&m.ID, &m.TeamID, &userID, &username,
			&m.Name, &m.Email, &m.JiraAccountID, &m.Role, &m.AvatarURL,
			&createdAt, &updatedAt,
		); err != nil {
			respondErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		if userID.Valid {
			m.UserID = &userID.String
			m.Username = username
		}
		m.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		m.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
		mates = append(mates, m)
	}

	respond(w, http.StatusOK, mates)
}

func (h *TeamsHandler) AddMate(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || (!h.Auth.IsTeamAdmin(user.ID, teamID) && !IsSuperAdmin(user)) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		JiraAccountID string `json:"jira_account_id"`
		Role          string `json:"role"`
		AvatarURL     string `json:"avatar_url"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondErr(w, http.StatusBadRequest, "name is required")
		return
	}
	email := strings.TrimSpace(req.Email)
	jiraAccountID := strings.TrimSpace(req.JiraAccountID)
	role := strings.TrimSpace(req.Role)
	avatarURL := strings.TrimSpace(req.AvatarURL)

	mateID := uuid.New().String()
	linkedUserID := h.findUserIDForMate(email, jiraAccountID)

	_, err := h.DB.Exec(`
		INSERT INTO team_mates (id, team_id, user_id, name, email, jira_account_id, role, avatar_url)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, mateID, teamID, linkedUserID, name, email, jiraAccountID, role, avatarURL)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			respondErr(w, http.StatusConflict, "team mate already exists in this team")
			return
		}
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}

	var created models.TeamMate
	var resolvedUserID sql.NullString
	var username, createdAt, updatedAt string
	err = h.DB.QueryRow(`
		SELECT tm.id, tm.team_id, tm.user_id, COALESCE(u.username, '') as username,
		       tm.name, tm.email, tm.jira_account_id, tm.role, tm.avatar_url,
		       tm.created_at, tm.updated_at
		FROM team_mates tm
		LEFT JOIN users u ON tm.user_id = u.id
		WHERE tm.id = ?
	`, mateID).Scan(
		&created.ID, &created.TeamID, &resolvedUserID, &username,
		&created.Name, &created.Email, &created.JiraAccountID, &created.Role, &created.AvatarURL,
		&createdAt, &updatedAt,
	)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if resolvedUserID.Valid {
		created.UserID = &resolvedUserID.String
		created.Username = username
	}
	created.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	created.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	respond(w, http.StatusCreated, created)
}

func (h *TeamsHandler) BulkAddMates(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || (!h.Auth.IsTeamAdmin(user.ID, teamID) && !IsSuperAdmin(user)) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var items []struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		JiraAccountID string `json:"jira_account_id"`
		Role          string `json:"role"`
		AvatarURL     string `json:"avatar_url"`
	}
	if err := decode(r, &items); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request body")
		return
	}

	addedCount := 0
	for _, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		email := strings.TrimSpace(item.Email)
		jiraAccountID := strings.TrimSpace(item.JiraAccountID)
		role := strings.TrimSpace(item.Role)
		avatarURL := strings.TrimSpace(item.AvatarURL)

		linkedUserID := h.findUserIDForMate(email, jiraAccountID)
		id := uuid.New().String()

		res, err := h.DB.Exec(`
			INSERT OR IGNORE INTO team_mates (id, team_id, user_id, name, email, jira_account_id, role, avatar_url)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		`, id, teamID, linkedUserID, name, email, jiraAccountID, role, avatarURL)
		if err == nil {
			if n, _ := res.RowsAffected(); n > 0 {
				addedCount++
			}
		}
	}

	respond(w, http.StatusOK, map[string]any{
		"added_count": addedCount,
		"total_items": len(items),
	})
}

func (h *TeamsHandler) UpdateMate(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	mateID := chi.URLParam(r, "mateID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || (!h.Auth.IsTeamAdmin(user.ID, teamID) && !IsSuperAdmin(user)) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var req struct {
		Name          string `json:"name"`
		Email         string `json:"email"`
		JiraAccountID string `json:"jira_account_id"`
		Role          string `json:"role"`
		AvatarURL     string `json:"avatar_url"`
	}
	if err := decode(r, &req); err != nil {
		respondErr(w, http.StatusBadRequest, "invalid request")
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		respondErr(w, http.StatusBadRequest, "name is required")
		return
	}
	email := strings.TrimSpace(req.Email)
	jiraAccountID := strings.TrimSpace(req.JiraAccountID)
	role := strings.TrimSpace(req.Role)
	avatarURL := strings.TrimSpace(req.AvatarURL)

	linkedUserID := h.findUserIDForMate(email, jiraAccountID)

	res, err := h.DB.Exec(`
		UPDATE team_mates
		SET name = ?, email = ?, jira_account_id = ?, role = ?, avatar_url = ?, user_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND team_id = ?
	`, name, email, jiraAccountID, role, avatarURL, linkedUserID, mateID, teamID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		respondErr(w, http.StatusNotFound, "team mate not found")
		return
	}

	var updated models.TeamMate
	var resolvedUserID sql.NullString
	var username, createdAt, updatedAt string
	_ = h.DB.QueryRow(`
		SELECT tm.id, tm.team_id, tm.user_id, COALESCE(u.username, '') as username,
		       tm.name, tm.email, tm.jira_account_id, tm.role, tm.avatar_url,
		       tm.created_at, tm.updated_at
		FROM team_mates tm
		LEFT JOIN users u ON tm.user_id = u.id
		WHERE tm.id = ?
	`, mateID).Scan(
		&updated.ID, &updated.TeamID, &resolvedUserID, &username,
		&updated.Name, &updated.Email, &updated.JiraAccountID, &updated.Role, &updated.AvatarURL,
		&createdAt, &updatedAt,
	)
	if resolvedUserID.Valid {
		updated.UserID = &resolvedUserID.String
		updated.Username = username
	}
	updated.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
	updated.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)

	respond(w, http.StatusOK, updated)
}

func (h *TeamsHandler) DeleteMate(w http.ResponseWriter, r *http.Request) {
	teamID := teamIDParam(r)
	mateID := chi.URLParam(r, "mateID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || (!h.Auth.IsTeamAdmin(user.ID, teamID) && !IsSuperAdmin(user)) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec("DELETE FROM team_mates WHERE id = ? AND team_id = ?", mateID, teamID)
	if err != nil {
		respondErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		respondErr(w, http.StatusNotFound, "team mate not found")
		return
	}

	respond(w, http.StatusOK, map[string]bool{"deleted": true})
}
