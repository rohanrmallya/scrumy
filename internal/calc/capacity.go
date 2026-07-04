package calc

import (
	"scrumy/internal/models"
	"time"
)

// countWeekdays counts weekdays (Mon–Fri) between start and end inclusive
func countWeekdays(startStr, endStr string) float64 {
	layout := "2006-01-02"
	start, err := time.Parse(layout, startStr)
	if err != nil {
		return 0
	}
	end, err := time.Parse(layout, endStr)
	if err != nil {
		return 0
	}
	count := 0.0
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Weekday() != time.Saturday && d.Weekday() != time.Sunday {
			count++
		}
	}
	return count
}

// ComputeSummary calculates the capacity summary for a capacity plan
func ComputeSummary(cp *models.CapacityPlan) models.CapacitySummary {
	// Build a map of leaves by sprintID -> memberID -> leaves
	leaveMap := map[string]map[string]float64{}
	for _, s := range cp.Sprints {
		leaveMap[s.ID] = map[string]float64{}
		for _, l := range s.Leaves {
			leaveMap[s.ID][l.MemberID] = l.Leaves
		}
	}

	// Build member utilization map
	utilMap := map[string]float64{}
	for _, m := range cp.Members {
		utilMap[m.ID] = m.UtilizationPct / 100.0
	}

	// Total utilization sum across all members
	totalUtil := 0.0
	for _, m := range cp.Members {
		totalUtil += m.UtilizationPct / 100.0
	}

	var summaries []models.SprintSummary
	var totals models.SprintSummary
	totals.SprintName = "TOTAL"

	for _, sprint := range cp.Sprints {
		weekdays := countWeekdays(sprint.StartDate, sprint.EndDate)
		gross := weekdays * totalUtil

		// Sum adjusted leaves
		leavesAdj := 0.0
		for memberID, util := range utilMap {
			l := leaveMap[sprint.ID][memberID]
			leavesAdj += l * util
		}

		net := gross - leavesAdj
		loaded := net * cp.LoadingFactor
		// Target SP = (loaded person days * productive hours) / hours per SP
		targetSP := 0.0
		if cp.HoursPerSP > 0 {
			targetSP = loaded * cp.ProductiveHours / cp.HoursPerSP
		}
		thin := targetSP * 0.75
		stretch := targetSP * 1.25

		ss := models.SprintSummary{
			SprintID:         sprint.ID,
			SprintName:       sprint.Name,
			GrossPersonDays:  round2(gross),
			Leaves:           round2(leavesAdj),
			NetPersonDays:    round2(net),
			LoadedPersonDays: round2(loaded),
			TargetSP:         round2(targetSP),
			ThinTarget:       round2(thin),
			StretchTarget:    round2(stretch),
		}
		summaries = append(summaries, ss)

		totals.GrossPersonDays += gross
		totals.Leaves += leavesAdj
		totals.NetPersonDays += net
		totals.LoadedPersonDays += loaded
		totals.TargetSP += targetSP
		totals.ThinTarget += thin
		totals.StretchTarget += stretch
	}

	totals.GrossPersonDays = round2(totals.GrossPersonDays)
	totals.Leaves = round2(totals.Leaves)
	totals.NetPersonDays = round2(totals.NetPersonDays)
	totals.LoadedPersonDays = round2(totals.LoadedPersonDays)
	totals.TargetSP = round2(totals.TargetSP)
	totals.ThinTarget = round2(totals.ThinTarget)
	totals.StretchTarget = round2(totals.StretchTarget)

	return models.CapacitySummary{
		Sprints: summaries,
		Totals:  totals,
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
