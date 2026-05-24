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
	Admins              []string `json:"admins,omitempty"` // List of user IDs who are admins
	IsAdmin             bool     `json:"is_admin,omitempty"` // Whether the current requester is an admin

	// Jira integration settings
	JiraURL      string `json:"jira_url"`
	JiraUser     string `json:"jira_user"`
	JiraToken    string `json:"jira_token,omitempty"`
	JiraJQL      string `json:"jira_jql"`
	JiraSPField  string `json:"jira_sp_field"`
	JiraTokenSet bool   `json:"jira_token_set"`
	JiraInsecure bool   `json:"jira_insecure"`
}

type JiraSnapshot struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"plan_id"`
	Name        string    `json:"name"`
	StartDate   string    `json:"start_date"` // YYYY-MM-DD
	EndDate     string    `json:"end_date"`   // YYYY-MM-DD
	AllWorklogs bool      `json:"all_worklogs"`
	Data        any       `json:"data"`       // parsed JiraSnapshotData (or raw JSON bytes)
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type JiraIssue struct {
	Key                       string  `json:"key"`
	Summary                   string  `json:"summary"`
	Status                    string  `json:"status"`
	StoryPoints               float64 `json:"story_points"`
	TimeSpentHours            float64 `json:"time_spent_hours"`
	StatusCategoryChangedDate string  `json:"status_category_changed_date"`
}

type JiraTotals struct {
	TotalStoryPoints float64 `json:"total_story_points"`
	TotalHoursLogged float64 `json:"total_hours_logged"`
	TotalWorkLogs    int     `json:"total_work_logs"`
	AvgHoursPerSP    float64 `json:"avg_hours_per_sp"`
}

type JiraWorklogItem struct {
	IssueKey     string  `json:"issue_key"`
	IssueSummary string  `json:"issue_summary"`
	HoursLogged  float64 `json:"hours_logged"`
}

type JiraLeaderboardEntry struct {
	AuthorName  string            `json:"author_name"`
	HoursLogged float64           `json:"hours_logged"`
	Percentage  float64           `json:"percentage"`
	Worklogs    []JiraWorklogItem `json:"worklogs"`
}

type JiraSnapshotData struct {
	Issues      []JiraIssue            `json:"issues"`
	Totals      JiraTotals             `json:"totals"`
	Leaderboard []JiraLeaderboardEntry `json:"leaderboard"`
}

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"` // admin | user
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
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
	TemplateID string    `json:"template_id"` // default | minimalist
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
	ClosingText  string        `json:"closing_text"`
}

// RetroContent is the JSON structure for a Sprint Retro presentation
type RetroContent struct {
	PreviousData PreviousData  `json:"previous_data"`
	Feedback     []string      `json:"feedback"`
	Contributors []Contributor `json:"contributors"`
	ClosingText  string        `json:"closing_text"`
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
