package handlers

import (
	"database/sql"
	"net/http"
	"scrumy/internal/db"
	"scrumy/internal/models"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PlansHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

func (h *PlansHandler) List(w http.ResponseWriter, r *http.Request) {
	user := h.Auth.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	rows, err := h.DB.Query(`
		SELECT p.id, p.name, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM capacity_plans cp WHERE cp.plan_id = p.id) as capacity_count,
			(SELECT COUNT(*) FROM presentations pr WHERE pr.plan_id = p.id) as presentation_count,
			EXISTS(SELECT 1 FROM plan_admins pa WHERE pa.plan_id = p.id AND pa.user_id = ?) as is_admin
		FROM plans p ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var plans []models.Plan
	for rows.Next() {
		var p models.Plan
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &createdAt, &updatedAt, &p.CapacityPlanCount, &p.PresentationCount, &p.IsAdmin); err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		plans = append(plans, p)
	}
	if plans == nil {
		plans = []models.Plan{}
	}
	respond(w, 200, plans)
}

func (h *PlansHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	_, err = tx.Exec(`INSERT INTO plans (id, name) VALUES (?, ?)`, id, body.Name)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	_, err = tx.Exec(`INSERT INTO plan_admins (plan_id, user_id) VALUES (?, ?)`, id, user.ID)
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

func (h *PlansHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "planID")
	h.GetByID(w, r, id)
}

func (h *PlansHandler) GetByID(w http.ResponseWriter, r *http.Request, id string) {
	user := h.Auth.GetUserFromContext(r.Context())
	userID := ""
	if user != nil {
		userID = user.ID
	}

	var p models.Plan
	var createdAt, updatedAt string
	var jiraToken string
	err := h.DB.QueryRow(`
		SELECT p.id, p.name, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM capacity_plans cp WHERE cp.plan_id = p.id),
			(SELECT COUNT(*) FROM presentations pr WHERE pr.plan_id = p.id),
			EXISTS(SELECT 1 FROM plan_admins pa WHERE pa.plan_id = p.id AND pa.user_id = ?),
			p.jira_url, p.jira_user, p.jira_token, p.jira_jql, p.jira_sp_field, p.jira_insecure
		FROM plans p WHERE p.id = ?
	`, userID, id).Scan(
		&p.ID, &p.Name, &createdAt, &updatedAt, &p.CapacityPlanCount, &p.PresentationCount, &p.IsAdmin,
		&p.JiraURL, &p.JiraUser, &jiraToken, &p.JiraJQL, &p.JiraSPField, &p.JiraInsecure,
	)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "plan not found")
		return
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	p.JiraTokenSet = (jiraToken != "")
	p.JiraToken = ""

	// Load admins
	adminRows, err := h.DB.Query(`
		SELECT u.username FROM users u
		JOIN plan_admins pa ON u.id = pa.user_id
		WHERE pa.plan_id = ?
	`, id)
	if err == nil {
		for adminRows.Next() {
			var username string
			adminRows.Scan(&username)
			p.Admins = append(p.Admins, username)
		}
		adminRows.Close()
	}

	respond(w, 200, p)
}

func (h *PlansHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, id) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM plans WHERE id = ?`, id)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondErr(w, 404, "plan not found")
		return
	}
	respond(w, 200, map[string]bool{"deleted": true})
}

func (h *PlansHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, id) {
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
	_, err := h.DB.Exec(`UPDATE plans SET name = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, body.Name, id)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.GetByID(w, r, id)
}

func (h *PlansHandler) AddAdmin(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
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

	_, err = h.DB.Exec("INSERT OR IGNORE INTO plan_admins (plan_id, user_id) VALUES (?, ?)", planID, userID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]bool{"added": true})
}

func (h *PlansHandler) RemoveAdmin(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
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

	// Prevent removing last admin? Requirement says Admin owns plans.
	var count int
	h.DB.QueryRow("SELECT COUNT(*) FROM plan_admins WHERE plan_id = ?", planID).Scan(&count)
	if count <= 1 {
		respondErr(w, 400, "cannot remove the last admin")
		return
	}

	_, err = h.DB.Exec("DELETE FROM plan_admins WHERE plan_id = ? AND user_id = ?", planID, userID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, map[string]bool{"removed": true})
}
