package scheduler

import (
	"database/sql"
	"fmt"
	"log"
	"scrumy/internal/db"
	"strings"
	"sync"
	"time"
)

type RefreshFunc func(teamID, snapshotID, triggerType string) error

type Scheduler struct {
	db          *db.DB
	refreshFn   RefreshFunc
	interval    time.Duration
	stopCh      chan struct{}
	wg          sync.WaitGroup
	runningLock sync.Mutex
	running     bool
}

// CalculateNextRun returns the next execution time for a daily HH:MM schedule from the given reference time.
func CalculateNextRun(dailyTime string, from time.Time) time.Time {
	dailyTime = strings.TrimSpace(dailyTime)
	parts := strings.Split(dailyTime, ":")
	hour, min := 9, 0
	if len(parts) >= 2 {
		_, _ = fmt.Sscanf(parts[0], "%d", &hour)
		_, _ = fmt.Sscanf(parts[1], "%d", &min)
	}
	if hour < 0 || hour > 23 {
		hour = 9
	}
	if min < 0 || min > 59 {
		min = 0
	}
	targetToday := time.Date(from.Year(), from.Month(), from.Day(), hour, min, 0, 0, from.Location())
	if targetToday.After(from) {
		return targetToday
	}
	return targetToday.AddDate(0, 0, 1)
}

// IsSnapshotActive reports whether today falls within [startDate, endDate].
// Returns (isActive, status) where status is "active", "upcoming", or "expired".
func IsSnapshotActive(startDate, endDate string, now time.Time) (bool, string) {
	today := now.Format("2006-01-02")
	if startDate != "" && today < startDate {
		return false, "upcoming"
	}
	if endDate != "" && today > endDate {
		return false, "expired"
	}
	return true, "active"
}

func New(database *db.DB, refreshFn RefreshFunc, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Scheduler{
		db:        database,
		refreshFn: refreshFn,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.runningLock.Lock()
	if s.running {
		s.runningLock.Unlock()
		return
	}
	s.running = true
	s.runningLock.Unlock()

	s.wg.Add(1)
	go s.loop()
	log.Printf("⏰ Scrumy Auto-Refresh Scheduler started (poll interval: %v)\n", s.interval)
}

func (s *Scheduler) Stop() {
	s.runningLock.Lock()
	if !s.running {
		s.runningLock.Unlock()
		return
	}
	s.running = false
	close(s.stopCh)
	s.runningLock.Unlock()

	s.wg.Wait()
	log.Println("⏰ Scrumy Auto-Refresh Scheduler stopped")
}

func (s *Scheduler) loop() {
	defer s.wg.Done()

	// Initial check on startup
	s.checkDueJobs()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkDueJobs()
		}
	}
}

type jobCandidate struct {
	id              string
	teamID          string
	name            string
	startDate       string
	endDate         string
	autoRefreshTime string
	nextRunAt       sql.NullString
}

func (s *Scheduler) checkDueJobs() {
	now := time.Now()
	nowStr := now.Format("2006-01-02 15:04:05")

	rows, err := s.db.Query(`
		SELECT id, team_id, name, start_date, end_date, auto_refresh_time, next_run_at
		FROM jira_snapshots
		WHERE auto_refresh_enabled = 1
	`)
	if err != nil {
		log.Printf("⚠️ Scheduler error querying snapshots: %v\n", err)
		return
	}
	defer rows.Close()

	var candidates []jobCandidate
	for rows.Next() {
		var j jobCandidate
		if err := rows.Scan(&j.id, &j.teamID, &j.name, &j.startDate, &j.endDate, &j.autoRefreshTime, &j.nextRunAt); err == nil {
			candidates = append(candidates, j)
		}
	}

	for _, j := range candidates {
		// If next_run_at is unset or unparseable, initialize it
		if !j.nextRunAt.Valid || strings.TrimSpace(j.nextRunAt.String) == "" {
			nextRun := CalculateNextRun(j.autoRefreshTime, now)
			_, _ = s.db.Exec(`UPDATE jira_snapshots SET next_run_at = ? WHERE id = ?`,
				nextRun.Format("2006-01-02 15:04:05"), j.id)
			continue
		}

		nextRunTime, parseErr := time.ParseInLocation("2006-01-02 15:04:05", j.nextRunAt.String, now.Location())
		if parseErr != nil {
			nextRunTime, parseErr = time.Parse(time.RFC3339, j.nextRunAt.String)
		}
		if parseErr != nil {
			// Fallback: reset next_run_at
			nextRun := CalculateNextRun(j.autoRefreshTime, now)
			_, _ = s.db.Exec(`UPDATE jira_snapshots SET next_run_at = ? WHERE id = ?`,
				nextRun.Format("2006-01-02 15:04:05"), j.id)
			continue
		}

		// Check if job is due
		if now.Before(nextRunTime) {
			continue
		}

		log.Printf("⏰ Scheduler triggering auto-refresh for snapshot %q (id: %s, team: %s, due at: %s, now: %s)\n",
			j.name, j.id, j.teamID, j.nextRunAt.String, nowStr)

		if s.refreshFn != nil {
			if err := s.refreshFn(j.teamID, j.id, "scheduled"); err != nil {
				log.Printf("⚠️ Auto-refresh failed for snapshot %q (%s): %v\n", j.name, j.id, err)
			}
		}
	}
}
