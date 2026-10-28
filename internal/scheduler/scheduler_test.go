package scheduler

import (
	"testing"
	"time"
)

func TestCalculateNextRun(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")

	// Reference time: 2026-10-28 08:30:00 UTC
	refTime := time.Date(2026, 10, 28, 8, 30, 0, 0, loc)

	// Scheduled for 09:00 -> should run today at 09:00
	nextRun := CalculateNextRun("09:00", refTime)
	expectedToday := time.Date(2026, 10, 28, 9, 0, 0, 0, loc)
	if !nextRun.Equal(expectedToday) {
		t.Errorf("expected %v, got %v", expectedToday, nextRun)
	}

	// Scheduled for 08:00 (already passed today) -> should run tomorrow at 08:00
	nextRunTomorrow := CalculateNextRun("08:00", refTime)
	expectedTomorrow := time.Date(2026, 10, 29, 8, 0, 0, 0, loc)
	if !nextRunTomorrow.Equal(expectedTomorrow) {
		t.Errorf("expected %v, got %v", expectedTomorrow, nextRunTomorrow)
	}

	// Malformed input fallback (should not crash and default hour 9)
	nextRunFallback := CalculateNextRun("invalid", refTime)
	if nextRunFallback.Hour() != 9 {
		t.Errorf("expected fallback hour 9, got %d", nextRunFallback.Hour())
	}
}

func TestIsSnapshotActive(t *testing.T) {
	loc, _ := time.LoadLocation("UTC")
	today := time.Date(2026, 10, 28, 12, 0, 0, 0, loc)

	// Case 1: Today is within [start_date, end_date]
	active, status := IsSnapshotActive("2026-10-20", "2026-10-31", today)
	if !active || status != "active" {
		t.Errorf("expected active=true, status='active', got %v, %s", active, status)
	}

	// Case 2: Today is exact start_date
	active, status = IsSnapshotActive("2026-10-28", "2026-11-05", today)
	if !active || status != "active" {
		t.Errorf("expected active on start date, got %v, %s", active, status)
	}

	// Case 3: Today is exact end_date
	active, status = IsSnapshotActive("2026-10-20", "2026-10-28", today)
	if !active || status != "active" {
		t.Errorf("expected active on end date, got %v, %s", active, status)
	}

	// Case 4: Snapshot hasn't started yet (upcoming)
	active, status = IsSnapshotActive("2026-11-01", "2026-11-15", today)
	if active || status != "upcoming" {
		t.Errorf("expected active=false, status='upcoming', got %v, %s", active, status)
	}

	// Case 5: Snapshot has ended (expired)
	active, status = IsSnapshotActive("2026-10-01", "2026-10-25", today)
	if active || status != "expired" {
		t.Errorf("expected active=false, status='expired', got %v, %s", active, status)
	}
}
