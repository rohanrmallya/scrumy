package calc

import (
	"scrumy/internal/models"
	"testing"
)

func TestComputeUserDeltas(t *testing.T) {
	oldEntries := []models.JiraLeaderboardEntry{
		{AuthorName: "Alice", HoursLogged: 10.0},
		{AuthorName: "Bob", HoursLogged: 15.0},
		{AuthorName: "Charlie", HoursLogged: 5.0},
	}

	newEntries := []models.JiraLeaderboardEntry{
		{AuthorName: "Alice", HoursLogged: 15.5}, // +5.5
		{AuthorName: "Bob", HoursLogged: 15.0},   // 0.0
		{AuthorName: "Charlie", HoursLogged: 4.0}, // -1.0
		{AuthorName: "Diana", HoursLogged: 8.0},  // +8.0 (new)
	}

	deltas := ComputeUserDeltas(oldEntries, newEntries)

	if len(deltas) != 4 {
		t.Fatalf("expected 4 deltas, got %d", len(deltas))
	}

	// First should be Diana (+8.0)
	if deltas[0].AuthorName != "Diana" || deltas[0].HoursDelta != 8.0 || deltas[0].PrevHoursLogged != 0.0 || deltas[0].NewHoursLogged != 8.0 {
		t.Errorf("expected Diana +8.0, got %+v", deltas[0])
	}

	// Second should be Alice (+5.5)
	if deltas[1].AuthorName != "Alice" || deltas[1].HoursDelta != 5.5 || deltas[1].PrevHoursLogged != 10.0 || deltas[1].NewHoursLogged != 15.5 {
		t.Errorf("expected Alice +5.5, got %+v", deltas[1])
	}

	// Third should be Bob (0.0)
	if deltas[2].AuthorName != "Bob" || deltas[2].HoursDelta != 0.0 || deltas[2].PrevHoursLogged != 15.0 || deltas[2].NewHoursLogged != 15.0 {
		t.Errorf("expected Bob 0.0, got %+v", deltas[2])
	}

	// Fourth should be Charlie (-1.0)
	if deltas[3].AuthorName != "Charlie" || deltas[3].HoursDelta != -1.0 || deltas[3].PrevHoursLogged != 5.0 || deltas[3].NewHoursLogged != 4.0 {
		t.Errorf("expected Charlie -1.0, got %+v", deltas[3])
	}
}
