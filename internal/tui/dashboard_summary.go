package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

// DashboardSummaryInput contains exact daemon aggregates. FreeSeats and
// BusySeats are supplied separately so rendering never infers availability.
type DashboardSummaryInput struct {
	Repositories int
	Sessions     int
	StatusCounts []transport.StatusCount
	FreeSeats    int
	BusySeats    int
}

// RenderDashboardSummary renders daemon aggregate chips without reading model
// state. Status counts are copied and ordered by status before rendering.
func RenderDashboardSummary(input DashboardSummaryInput, width int) string {
	if width <= 0 {
		return ""
	}

	counts := append([]transport.StatusCount(nil), input.StatusCounts...)
	sort.SliceStable(counts, func(left, right int) bool {
		if counts[left].Status != counts[right].Status {
			return counts[left].Status < counts[right].Status
		}
		return counts[left].Count < counts[right].Count
	})

	lines := []string{
		"Summary",
		fmt.Sprintf("repositories: %d", input.Repositories),
		fmt.Sprintf("sessions: %d", input.Sessions),
		"beads: " + dashboardStatusCounts(counts),
		fmt.Sprintf("seats: free %d; busy %d", input.FreeSeats, input.BusySeats),
	}
	return fitDashboardLines(lines, width)
}

func dashboardStatusCounts(counts []transport.StatusCount) string {
	if len(counts) == 0 {
		return "0"
	}
	parts := make([]string, 0, len(counts))
	for _, count := range counts {
		status := count.Status
		if status == "" {
			status = "unknown"
		}
		parts = append(parts, fmt.Sprintf("%s %d", status, count.Count))
	}
	return strings.Join(parts, "; ")
}

func fitDashboardLines(lines []string, width int) string {
	for index := range lines {
		lines[index] = fit(lines[index], width)
	}
	return strings.Join(lines, "\n")
}
