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
	DB *db.DB
}

func (h *PlansHandler) List(w http.ResponseWriter, r *http.Request) {
	rows, err := h.DB.Query(`
		SELECT p.id, p.name, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM capacity_plans cp WHERE cp.plan_id = p.id) as capacity_count,
			(SELECT COUNT(*) FROM presentations pr WHERE pr.plan_id = p.id) as presentation_count
		FROM plans p ORDER BY p.created_at DESC
	`)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()

	var plans []models.Plan
	for rows.Next() {
		var p models.Plan
		var createdAt, updatedAt string
		if err := rows.Scan(&p.ID, &p.Name, &createdAt, &updatedAt, &p.CapacityPlanCount, &p.PresentationCount); err != nil {
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
	var body struct {
		Name string `json:"name"`
	}
	if err := decode(r, &body); err != nil || body.Name == "" {
		respondErr(w, 400, "name is required")
		return
	}
	id := uuid.NewString()
	_, err := h.DB.Exec(`INSERT INTO plans (id, name) VALUES (?, ?)`, id, body.Name)
	if err != nil {
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
	var p models.Plan
	var createdAt, updatedAt string
	err := h.DB.QueryRow(`
		SELECT p.id, p.name, p.created_at, p.updated_at,
			(SELECT COUNT(*) FROM capacity_plans cp WHERE cp.plan_id = p.id),
			(SELECT COUNT(*) FROM presentations pr WHERE pr.plan_id = p.id)
		FROM plans p WHERE p.id = ?
	`, id).Scan(&p.ID, &p.Name, &createdAt, &updatedAt, &p.CapacityPlanCount, &p.PresentationCount)
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
	respond(w, 200, p)
}

func (h *PlansHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "planID")
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
