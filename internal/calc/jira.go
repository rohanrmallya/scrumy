package calc

import (
	"math"
	"scrumy/internal/models"
	"sort"
)

// ComputeUserDeltas calculates the change in logged hours for each contributor between two leaderboard states.
func ComputeUserDeltas(oldEntries, newEntries []models.JiraLeaderboardEntry) []models.JiraUserRefreshDelta {
	oldMap := make(map[string]float64)
	for _, e := range oldEntries {
		oldMap[e.AuthorName] = e.HoursLogged
	}

	newMap := make(map[string]float64)
	for _, e := range newEntries {
		newMap[e.AuthorName] = e.HoursLogged
	}

	authors := make(map[string]bool)
	for a := range oldMap {
		authors[a] = true
	}
	for a := range newMap {
		authors[a] = true
	}

	var deltas []models.JiraUserRefreshDelta
	for author := range authors {
		prev := oldMap[author]
		curr := newMap[author]
		delta := math.Round((curr-prev)*100) / 100
		prev = math.Round(prev*100) / 100
		curr = math.Round(curr*100) / 100

		deltas = append(deltas, models.JiraUserRefreshDelta{
			AuthorName:      author,
			PrevHoursLogged: prev,
			NewHoursLogged:  curr,
			HoursDelta:      delta,
		})
	}

	// Sort by highest delta first, then by new hours logged descending, then by name alphabetically
	sort.Slice(deltas, func(i, j int) bool {
		if deltas[i].HoursDelta != deltas[j].HoursDelta {
			return deltas[i].HoursDelta > deltas[j].HoursDelta
		}
		if deltas[i].NewHoursLogged != deltas[j].NewHoursLogged {
			return deltas[i].NewHoursLogged > deltas[j].NewHoursLogged
		}
		return deltas[i].AuthorName < deltas[j].AuthorName
	})

	if deltas == nil {
		deltas = []models.JiraUserRefreshDelta{}
	}
	return deltas
}
