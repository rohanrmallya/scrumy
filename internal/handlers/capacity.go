package handlers

import (
	"database/sql"
	"net/http"
	"scrumy/internal/calc"
	"scrumy/internal/db"
	"scrumy/internal/models"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type CapacityHandler struct {
	DB   *db.DB
	Auth *AuthHandler
}

func (h *CapacityHandler) checkAdmin(r *http.Request, planID string) bool {
	user := h.Auth.GetUserFromContext(r.Context())
	if user == nil {
		return false
	}
	return h.Auth.IsPlanAdmin(user.ID, planID)
}

func (h *CapacityHandler) getPlanID(cpID string) string {
	var planID string
	h.DB.QueryRow("SELECT plan_id FROM capacity_plans WHERE id = ?", cpID).Scan(&planID)
	return planID
}

// ─── Capacity Plans ───────────────────────────────────────────────────────────

func (h *CapacityHandler) List(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	rows, err := h.DB.Query(`
		SELECT id, plan_id, name, status, hours_per_sp, productive_hours, loading_factor, created_at, updated_at
		FROM capacity_plans WHERE plan_id = ? ORDER BY created_at DESC
	`, planID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	defer rows.Close()
	var cps []models.CapacityPlan
	for rows.Next() {
		cp, err := scanCP(rows)
		if err != nil {
			respondErr(w, 500, err.Error())
			return
		}
		cps = append(cps, cp)
	}
	if cps == nil {
		cps = []models.CapacityPlan{}
	}
	respond(w, 200, cps)
}

func (h *CapacityHandler) Create(w http.ResponseWriter, r *http.Request) {
	planID := chi.URLParam(r, "planID")
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name            string  `json:"name"`
		HoursPerSP      float64 `json:"hours_per_sp"`
		ProductiveHours float64 `json:"productive_hours"`
		LoadingFactor   float64 `json:"loading_factor"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}
	if body.Name == "" {
		respondErr(w, 400, "name is required")
		return
	}
	if body.HoursPerSP == 0 {
		body.HoursPerSP = 8
	}
	if body.ProductiveHours == 0 {
		body.ProductiveHours = 6
	}
	if body.LoadingFactor == 0 {
		body.LoadingFactor = 0.75
	}
	id := uuid.NewString()
	_, err := h.DB.Exec(`
		INSERT INTO capacity_plans (id, plan_id, name, hours_per_sp, productive_hours, loading_factor)
		VALUES (?, ?, ?, ?, ?, ?)
	`, id, planID, body.Name, body.HoursPerSP, body.ProductiveHours, body.LoadingFactor)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.getByID(w, r, id)
}

func (h *CapacityHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "cpID")
	h.getByID(w, r, id)
}

func (h *CapacityHandler) getByID(w http.ResponseWriter, r *http.Request, id string) {
	cp, err := h.loadFull(id)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "capacity plan not found")
		return
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	respond(w, 200, cp)
}

func (h *CapacityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "cpID")
	planID := h.getPlanID(id)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name            string  `json:"name"`
		Status          string  `json:"status"`
		HoursPerSP      float64 `json:"hours_per_sp"`
		ProductiveHours float64 `json:"productive_hours"`
		LoadingFactor   float64 `json:"loading_factor"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}
	_, err := h.DB.Exec(`
		UPDATE capacity_plans SET name=?, status=?, hours_per_sp=?, productive_hours=?, loading_factor=?, updated_at=CURRENT_TIMESTAMP
		WHERE id=?
	`, body.Name, body.Status, body.HoursPerSP, body.ProductiveHours, body.LoadingFactor, id)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.getByID(w, r, id)
}

func (h *CapacityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "cpID")
	planID := h.getPlanID(id)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	res, err := h.DB.Exec(`DELETE FROM capacity_plans WHERE id=?`, id)
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

func (h *CapacityHandler) Summary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "cpID")
	cp, err := h.loadFull(id)
	if err == sql.ErrNoRows {
		respondErr(w, 404, "not found")
		return
	}
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	summary := calc.ComputeSummary(&cp)
	respond(w, 200, summary)
}

// ─── Members ─────────────────────────────────────────────────────────────────

func (h *CapacityHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name           string  `json:"name"`
		Role           string  `json:"role"`
		UtilizationPct float64 `json:"utilization_pct"`
	}
	if err := decode(r, &body); err != nil || body.Name == "" {
		respondErr(w, 400, "name is required")
		return
	}
	if body.UtilizationPct == 0 {
		body.UtilizationPct = 100
	}
	id := uuid.NewString()
	_, err := h.DB.Exec(`
		INSERT INTO team_members (id, capacity_plan_id, name, role, utilization_pct, sort_order)
		VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),0)+1 FROM team_members WHERE capacity_plan_id=?))
	`, id, cpID, body.Name, body.Role, body.UtilizationPct, cpID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.getByID(w, r, cpID)
}

func (h *CapacityHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	mID := chi.URLParam(r, "memberID")
	var body struct {
		Name           string  `json:"name"`
		Role           string  `json:"role"`
		UtilizationPct float64 `json:"utilization_pct"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}
	_, err := h.DB.Exec(`UPDATE team_members SET name=?, role=?, utilization_pct=? WHERE id=? AND capacity_plan_id=?`,
		body.Name, body.Role, body.UtilizationPct, mID, cpID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.getByID(w, r, cpID)
}

func (h *CapacityHandler) DeleteMember(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	mID := chi.URLParam(r, "memberID")
	h.DB.Exec(`DELETE FROM team_members WHERE id=? AND capacity_plan_id=?`, mID, cpID)
	h.getByID(w, r, cpID)
}

// ─── Sprints ─────────────────────────────────────────────────────────────────

func (h *CapacityHandler) AddSprint(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	var body struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := decode(r, &body); err != nil || body.StartDate == "" || body.EndDate == "" {
		respondErr(w, 400, "start_date and end_date required")
		return
	}
	if body.Name == "" {
		// auto-name
		var count int
		h.DB.QueryRow(`SELECT COUNT(*) FROM sprints WHERE capacity_plan_id=?`, cpID).Scan(&count)
		body.Name = "Sprint " + itoa(count+1)
	}
	id := uuid.NewString()
	_, err := h.DB.Exec(`
		INSERT INTO sprints (id, capacity_plan_id, name, start_date, end_date, sort_order)
		VALUES (?, ?, ?, ?, ?, (SELECT COALESCE(MAX(sort_order),0)+1 FROM sprints WHERE capacity_plan_id=?))
	`, id, cpID, body.Name, body.StartDate, body.EndDate, cpID)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.getByID(w, r, cpID)
}

func (h *CapacityHandler) UpdateSprint(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	sID := chi.URLParam(r, "sprintID")
	var body struct {
		Name      string `json:"name"`
		StartDate string `json:"start_date"`
		EndDate   string `json:"end_date"`
	}
	if err := decode(r, &body); err != nil {
		respondErr(w, 400, "invalid body")
		return
	}
	h.DB.Exec(`UPDATE sprints SET name=?, start_date=?, end_date=? WHERE id=? AND capacity_plan_id=?`,
		body.Name, body.StartDate, body.EndDate, sID, cpID)
	h.getByID(w, r, cpID)
}

func (h *CapacityHandler) DeleteSprint(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	sID := chi.URLParam(r, "sprintID")
	h.DB.Exec(`DELETE FROM sprints WHERE id=? AND capacity_plan_id=?`, sID, cpID)
	h.getByID(w, r, cpID)
}

func (h *CapacityHandler) UpsertLeave(w http.ResponseWriter, r *http.Request) {
	cpID := chi.URLParam(r, "cpID")
	planID := h.getPlanID(cpID)
	if !h.checkAdmin(r, planID) {
		respondErr(w, http.StatusForbidden, "forbidden")
		return
	}

	sID := chi.URLParam(r, "sprintID")
	var body struct {
		MemberID string  `json:"member_id"`
		Leaves   float64 `json:"leaves"`
	}
	if err := decode(r, &body); err != nil || body.MemberID == "" {
		respondErr(w, 400, "member_id required")
		return
	}
	id := uuid.NewString()
	_, err := h.DB.Exec(`
		INSERT INTO sprint_leaves (id, sprint_id, member_id, leaves) VALUES (?, ?, ?, ?)
		ON CONFLICT(sprint_id, member_id) DO UPDATE SET leaves=excluded.leaves
	`, id, sID, body.MemberID, body.Leaves)
	if err != nil {
		respondErr(w, 500, err.Error())
		return
	}
	h.getByID(w, r, cpID)
}

// ─── Internal helpers ────────────────────────────────────────────────────────

func (h *CapacityHandler) loadFull(id string) (models.CapacityPlan, error) {
	row := h.DB.QueryRow(`SELECT id, plan_id, name, status, hours_per_sp, productive_hours, loading_factor, created_at, updated_at FROM capacity_plans WHERE id=?`, id)
	cp, err := scanCP(row)
	if err != nil {
		return cp, err
	}
	// members
	mRows, err := h.DB.Query(`SELECT id, capacity_plan_id, name, role, utilization_pct, sort_order FROM team_members WHERE capacity_plan_id=? ORDER BY sort_order`, id)
	if err != nil {
		return cp, err
	}
	for mRows.Next() {
		var m models.TeamMember
		mRows.Scan(&m.ID, &m.CapacityPlanID, &m.Name, &m.Role, &m.UtilizationPct, &m.SortOrder)
		cp.Members = append(cp.Members, m)
	}
	mRows.Close()

	// sprints
	sRows, err := h.DB.Query(`SELECT id, capacity_plan_id, name, start_date, end_date, sort_order FROM sprints WHERE capacity_plan_id=? ORDER BY sort_order`, id)
	if err != nil {
		return cp, err
	}
	for sRows.Next() {
		var s models.Sprint
		sRows.Scan(&s.ID, &s.CapacityPlanID, &s.Name, &s.StartDate, &s.EndDate, &s.SortOrder)
		cp.Sprints = append(cp.Sprints, s)
	}
	sRows.Close()

	for i := range cp.Sprints {
		s := &cp.Sprints[i]
		// leaves for this sprint
		lRows, _ := h.DB.Query(`SELECT id, sprint_id, member_id, leaves FROM sprint_leaves WHERE sprint_id=?`, s.ID)
		if lRows != nil {
			for lRows.Next() {
				var l models.SprintLeave
				lRows.Scan(&l.ID, &l.SprintID, &l.MemberID, &l.Leaves)
				s.Leaves = append(s.Leaves, l)
			}
			lRows.Close()
		}
	}
	return cp, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanCP(row scanner) (models.CapacityPlan, error) {
	var cp models.CapacityPlan
	var createdAt, updatedAt string
	err := row.Scan(&cp.ID, &cp.PlanID, &cp.Name, &cp.Status, &cp.HoursPerSP, &cp.ProductiveHours, &cp.LoadingFactor, &createdAt, &updatedAt)
	if err != nil {
		return cp, err
	}
	cp.CreatedAt, _ = time.Parse("2006-01-02T15:04:05Z", createdAt)
	cp.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05Z", updatedAt)
	return cp, nil
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
