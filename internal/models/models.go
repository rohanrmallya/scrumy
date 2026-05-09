package models

import "time"

type Plan struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	// Aggregated counts for UI
	CapacityPlanCount   int `json:"capacity_plan_count,omitempty"`
	PresentationCount   int `json:"presentation_count,omitempty"`
}

type CapacityPlan struct {
	ID               string       `json:"id"`
	PlanID           string       `json:"plan_id"`
	Name             string       `json:"name"`
	Status           string       `json:"status"` // draft | active | archived
	HoursPerSP       float64      `json:"hours_per_sp"`
	ProductiveHours  float64      `json:"productive_hours"`
	LoadingFactor    float64      `json:"loading_factor"`
	CreatedAt        time.Time    `json:"created_at"`
	UpdatedAt        time.Time    `json:"updated_at"`
	Members          []TeamMember `json:"members,omitempty"`
	Sprints          []Sprint     `json:"sprints,omitempty"`
}

type TeamMember struct {
	ID              string  `json:"id"`
	CapacityPlanID  string  `json:"capacity_plan_id"`
	Name            string  `json:"name"`
	Role            string  `json:"role"`
	UtilizationPct  float64 `json:"utilization_pct"`
	SortOrder       int     `json:"sort_order"`
}

type Sprint struct {
	ID              string        `json:"id"`
	CapacityPlanID  string        `json:"capacity_plan_id"`
	Name            string        `json:"name"`
	StartDate       string        `json:"start_date"` // YYYY-MM-DD
	EndDate         string        `json:"end_date"`   // YYYY-MM-DD
	SortOrder       int           `json:"sort_order"`
	Leaves          []SprintLeave `json:"leaves,omitempty"`
}

type SprintLeave struct {
	ID       string  `json:"id"`
	SprintID string  `json:"sprint_id"`
	MemberID string  `json:"member_id"`
	Leaves   float64 `json:"leaves"`
}

// SprintSummary holds calculated values for one sprint
type SprintSummary struct {
	SprintID         string  `json:"sprint_id"`
	SprintName       string  `json:"sprint_name"`
	GrossPersonDays  float64 `json:"gross_person_days"`
	Leaves           float64 `json:"leaves"`
	NetPersonDays    float64 `json:"net_person_days"`
	LoadedPersonDays float64 `json:"loaded_person_days"`
	TargetSP         float64 `json:"target_sp"`
	ThinTarget       float64 `json:"thin_target"`
	StretchTarget    float64 `json:"stretch_target"`
}

type CapacitySummary struct {
	Sprints []SprintSummary `json:"sprints"`
	Totals  SprintSummary   `json:"totals"`
}

type Presentation struct {
	ID         string    `json:"id"`
	PlanID     string    `json:"plan_id"`
	Type       string    `json:"type"` // intro | retro
	Title      string    `json:"title"`
	Status     string    `json:"status"` // draft | published
	SprintName string    `json:"sprint_name"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Content    any       `json:"content,omitempty"`
}
type Learning struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type Change struct {
	Title   string   `json:"title"`
	Content string   `json:"content"`
	Tags    []string `json:"tags"`
}

type Contributor struct {
	Name         string `json:"name"`
	Contribution string `json:"contribution"`
}

// IntroContent is the JSON structure for a Scrum Intro presentation
type IntroContent struct {
	Learnings    []Learning    `json:"learnings"`
	Changes      []Change      `json:"changes"`
	PreviousData PreviousData  `json:"previous_data"`
	Epics        []Epic        `json:"epics"`
	Contributors []Contributor `json:"contributors"`
}

// RetroContent is the JSON structure for a Sprint Retro presentation
type RetroContent struct {
	PreviousData PreviousData  `json:"previous_data"`
	Feedback     []string      `json:"feedback"`
	Contributors []Contributor `json:"contributors"`
}

type PreviousData struct {
	TotalSPDelivered    float64 `json:"total_sp_delivered"`
	TotalHoursLogged    float64 `json:"total_hours_logged"`
	TotalWorkLogs       int     `json:"total_work_logs"`
	AvgHoursPerSP       float64 `json:"avg_hours_per_sp"`
	PlannedSP           float64 `json:"planned_sp"`
	ExecutedSP          float64 `json:"executed_sp"`
	Spillovers          int     `json:"spillovers"`
	TotalEpicsDelivered int     `json:"total_epics_delivered"`
}

type Epic struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Summary     string  `json:"summary"`
	WhyNeeded   string  `json:"why_needed"`
	WhenDoing   string  `json:"when_doing"`
	Audience    string  `json:"audience"`
	TotalSP     float64 `json:"total_sp"`
}
