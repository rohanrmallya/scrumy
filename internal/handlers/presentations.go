package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"scrumy/internal/db"
	"scrumy/internal/models"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type PresentationsHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

func (h *PresentationsHandler) checkAdmin(r *http.Request, planID string) bool {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil {
		return false
	}
	return h.Auth.IsPlanAdmin(user.ID, planID)
}

func (h *PresentationsHandler) getPlanID(presID string) string {
	var planID string
	h.DB.QueryRow("SELECT plan_id FROM presentations WHERE id = ?", presID).Scan(&planID)
	return planID
}

func (h *PresentationsHandler) List(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	rows, err := h.DB.Query(`
		SELECT id, plan_id, type, template_id, title, status, sprint_name, created_at, updated_at
		FROM presentations WHERE plan_id=? ORDER BY created_at DESC
	`, planID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var list []models.Presentation
	for rows.Next() {
		p, err := scanPres(rows)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		list = append(list, p)
	}
	if list == nil {
		list = []models.Presentation{}
	}
	respond(w, 200, list)
}

func (h *PresentationsHandler) Create(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Type       string `json:"type"`
		TemplateID string `json:"template_id"`
		Title      string `json:"title"`
		SprintName string `json:"sprint_name"`
	}
	if err := decode(r, &body); err != nil || body.Type == "" {
		respondErr(w, 400, "type is required (intro|retro)")
		return
	}
	if body.TemplateID == "" {
		body.TemplateID = "default"
	}
	if body.Title == "" {
		body.Title = "Untitled Presentation"
	}
	id := uuid.NewString()
	_, err := h.DB.Exec(`
		INSERT INTO presentations (id, plan_id, type, template_id, title, sprint_name) VALUES (?, ?, ?, ?, ?, ?)
	`, id, planID, body.Type, body.TemplateID, body.Title, body.SprintName)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	// create empty content row
	h.DB.Exec(`INSERT INTO presentation_data (id, presentation_id, content) VALUES (?, ?, '{}')`, uuid.NewString(), id)
	h.getByID(w, r, id)
}

func (h *PresentationsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "presID")
	h.getByID(w, r, id)
}

func (h *PresentationsHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "presID")
	planID := h.getPlanID(id)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Title      string `json:"title"`
		TemplateID string `json:"template_id"`
		SprintName string `json:"sprint_name"`
		Content    any    `json:"content"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}
	contentJSON, _ := json.Marshal(body.Content)

	if body.TemplateID != "" {
		h.DB.Exec(`UPDATE presentations SET title=?, template_id=?, sprint_name=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			body.Title, body.TemplateID, body.SprintName, id)
	} else {
		h.DB.Exec(`UPDATE presentations SET title=?, sprint_name=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
			body.Title, body.SprintName, id)
	}

	h.DB.Exec(`
		INSERT INTO presentation_data (id, presentation_id, content, updated_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(presentation_id) DO UPDATE SET content=excluded.content, updated_at=CURRENT_TIMESTAMP
	`, uuid.NewString(), id, string(contentJSON))
	h.getByID(w, r, id)
}

func (h *PresentationsHandler) Publish(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "presID")
	planID := h.getPlanID(id)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	h.DB.Exec(`UPDATE presentations SET status='published', updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	h.getByID(w, r, id)
}

func (h *PresentationsHandler) Unpublish(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "presID")
	planID := h.getPlanID(id)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	h.DB.Exec(`UPDATE presentations SET status='draft', updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	h.getByID(w, r, id)
}

func (h *PresentationsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "presID")
	planID := h.getPlanID(id)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM presentations WHERE id=?`, id)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		respondErr(w, 404, "not found")
		return
	}
	respond(w, 200, map[string]bool{"deleted": true})
}

// AddRetroFeedback adds a learning/feedback item to a retro presentation live
func (h *PresentationsHandler) AddRetroFeedback(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "presID")

	var body struct {
		Item string `json:"item"`
	}
	if err := decode(r, &body); err != nil || body.Item == "" {
		respondErr(w, 400, "item is required")
		return
	}
	// Load existing content
	var contentStr string
	err := h.DB.QueryRow(`SELECT content FROM presentation_data WHERE presentation_id=?`, id).Scan(&contentStr)
	if err != nil {
		respondErr(w, 404, "not found")
		return
	}
	var content models.RetroContent
	json.Unmarshal([]byte(contentStr), &content)
	content.Feedback = append(content.Feedback, body.Item)
	updated, _ := json.Marshal(content)
	h.DB.Exec(`UPDATE presentation_data SET content=?, updated_at=CURRENT_TIMESTAMP WHERE presentation_id=?`, string(updated), id)
	h.getByID(w, r, id)
}

func (h *PresentationsHandler) getByID(w http.ResponseWriter, r *http.Request, id string) {
	row := h.DB.QueryRow(`SELECT id, plan_id, type, template_id, title, status, sprint_name, created_at, updated_at FROM presentations WHERE id=?`, id)
	p, err := scanPres(row)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "not found")
		return
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	// Load content
	var contentStr string
	h.DB.QueryRow(`SELECT content FROM presentation_data WHERE presentation_id=?`, id).Scan(&contentStr)
	if contentStr != "" {
		var raw any
		json.Unmarshal([]byte(contentStr), &raw)
		p.Content = raw
	}
	respond(w, 200, p)
}

type presScanner interface {
	Scan(dest ...any) error
}

func scanPres(row presScanner) (models.Presentation, error) {
	var p models.Presentation
	var createdAt, updatedAt string
	err := row.Scan(&p.ID, &p.PlanID, &p.Type, &p.TemplateID, &p.Title, &p.Status, &p.SprintName, &createdAt, &updatedAt)
	if err != nil {
		return p, err
	}
	p.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	p.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	return p, nil
}
