package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"scrumy/internal/db"
	"scrumy/internal/jira"
	"scrumy/internal/models"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type JiraHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

func (h *JiraHandler) getClientForPlan(planID string) (*jira.Client, string, string, error) {
	var urlStr, user, token, jql, spField string
	err := h.DB.QueryRow(`
		SELECT jira_url, jira_user, jira_token, jira_jql, jira_sp_field 
		FROM plans WHERE id = ?
	`, planID).Scan(&urlStr, &user, &token, &jql, &spField)
	if err == sql.ErrNoRows {
		return nil, "", "", fmt.Errorf("plan not found")
	} else if err != nil {
		return nil, "", "", err
	}
	if urlStr == "" || user == "" || token == "" {
		return nil, "", "", fmt.Errorf("Jira integration is not fully configured")
	}
	return jira.NewClient(urlStr, user, token), jql, spField, nil
}

func (h *JiraHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		JiraURL     string `json:"jira_url"`
		JiraUser    string `json:"jira_user"`
		JiraToken   string `json:"jira_token"`
		JiraJQL     string `json:"jira_jql"`
		JiraSPField string `json:"jira_sp_field"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}

	var err error
	if body.JiraToken == "" {
		_, err = h.DB.Exec(`
			UPDATE plans 
			SET jira_url = ?, jira_user = ?, jira_jql = ?, jira_sp_field = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, body.JiraURL, body.JiraUser, body.JiraJQL, body.JiraSPField, planID)
	} else {
		_, err = h.DB.Exec(`
			UPDATE plans 
			SET jira_url = ?, jira_user = ?, jira_token = ?, jira_jql = ?, jira_sp_field = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, body.JiraURL, body.JiraUser, body.JiraToken, body.JiraJQL, body.JiraSPField, planID)
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	respond(w, 200, map[string]bool{"updated": true})
}

func (h *JiraHandler) TestConnection(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		JiraURL   string `json:"jira_url"`
		JiraUser  string `json:"jira_user"`
		JiraToken string `json:"jira_token"`
	}
	_ = decode(r, &body)

	// If token is omitted, try fetching the existing one from the DB
	if body.JiraToken == "" {
		_ = h.DB.QueryRow("SELECT jira_token FROM plans WHERE id = ?", planID).Scan(&body.JiraToken)
	}
	if body.JiraURL == "" {
		_ = h.DB.QueryRow("SELECT jira_url FROM plans WHERE id = ?", planID).Scan(&body.JiraURL)
	}
	if body.JiraUser == "" {
		_ = h.DB.QueryRow("SELECT jira_user FROM plans WHERE id = ?", planID).Scan(&body.JiraUser)
	}

	if body.JiraURL == "" || body.JiraUser == "" || body.JiraToken == "" {
		respondErr(w, 400, "Missing configuration (URL, User, or Token)")
		return
	}

	client := jira.NewClient(body.JiraURL, body.JiraUser, body.JiraToken)
	if err := client.TestConnection(); err != nil {
		respondErr(w, 400, err.Error())
		return
	}

	respond(w, 200, map[string]bool{"ok": true})
}

func (h *JiraHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"` // YYYY-MM-DD
		EndDate   string `json:"end_date"`   // YYYY-MM-DD
	}
	if err := decode(r, &body); err != nil || body.Name == "" || body.StartDate == "" || body.EndDate == "" {
		respondErr(w, 400, "name, start_date, and end_date are required")
		return
	}

	client, jql, spField, err := h.getClientForPlan(planID)
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
	_, err = h.DB.Exec(`
		INSERT INTO jira_snapshots (id, plan_id, name, start_date, end_date, data)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, planID, body.Name, body.StartDate, body.EndDate, string(jsonData))
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	h.GetSnapshotByID(w, r, planID, id)
}

func (h *JiraHandler) ListSnapshots(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")

	rows, err := h.DB.Query(`
		SELECT id, plan_id, name, start_date, end_date, created_at, updated_at
		FROM jira_snapshots 
		WHERE plan_id = ? 
		ORDER BY created_at DESC
	`, planID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var list []models.JiraSnapshot
	for rows.Next() {
		var s models.JiraSnapshot
		var createdAt, updatedAt string
		err := rows.Scan(&s.ID, &s.PlanID, &s.Name, &s.StartDate, &s.EndDate, &createdAt, &updatedAt)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		s.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
		s.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
		list = append(list, s)
	}
	if list == nil {
		list = []models.JiraSnapshot{}
	}
	respond(w, 200, list)
}

func (h *JiraHandler) GetSnapshot(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	id := chi.URLParam(r, "snapshotID")
	h.GetSnapshotByID(w, r, planID, id)
}

func (h *JiraHandler) GetSnapshotByID(w http.ResponseWriter, r *http.Request, planID, id string) {
	var s models.JiraSnapshot
	var dataStr string
	var createdAt, updatedAt string
	err := h.DB.QueryRow(`
		SELECT id, plan_id, name, start_date, end_date, data, created_at, updated_at
		FROM jira_snapshots 
		WHERE id = ? AND plan_id = ?
	`, id, planID).Scan(&s.ID, &s.PlanID, &s.Name, &s.StartDate, &s.EndDate, &dataStr, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "snapshot not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	s.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	s.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)

	var parsedData models.JiraSnapshotData
	if err := json.Unmarshal([]byte(dataStr), &parsedData); err == nil {
		s.Data = parsedData
	} else {
		s.Data = map[string]any{}
	}

	respond(w, 200, s)
}

func (h *JiraHandler) RefreshSnapshot(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	id := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var name, startDate, endDate string
	err := h.DB.QueryRow(`
		SELECT name, start_date, end_date FROM jira_snapshots 
		WHERE id = ? AND plan_id = ?
	`, id, planID).Scan(&name, &startDate, &endDate)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "snapshot not found")
		return
	} else if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	client, jql, spField, err := h.getClientForPlan(planID)
	if err != nil {
		respondErr(w, 400, err.Error())
		return
	}

	data, err := client.FetchRetroData(jql, startDate, endDate, spField)
	if err != nil {
		respondErr(w, 500, fmt.Sprintf("failed to refresh data from Jira: %v", err))
		return
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	_, err = h.DB.Exec(`
		UPDATE jira_snapshots 
		SET data = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND plan_id = ?
	`, string(jsonData), id, planID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}

	h.GetSnapshotByID(w, r, planID, id)
}

func (h *JiraHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	id := chi.URLParam(r, "snapshotID")
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil || !h.Auth.IsPlanAdmin(user.ID, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM jira_snapshots WHERE id = ? AND plan_id = ?`, id, planID)
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
