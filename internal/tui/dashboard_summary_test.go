package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

func TestRenderDashboardSummaryShowsExactAggregates(t *testing.T) {
	input := DashboardSummaryInput{
		Repositories: 2,
		Sessions:     3,
		StatusCounts: []transport.StatusCount{{Status: "open", Count: 4}, {Status: "closed", Count: 1}, {Status: "in_progress", Count: 2}},
		FreeSeats:    1,
		BusySeats:    2,
	}
	original := append([]transport.StatusCount(nil), input.StatusCounts...)

	got := RenderDashboardSummary(input, 80)
	want := strings.Join([]string{
		"Summary",
		"repositories: 2",
		"sessions: 3",
		"beads: closed 1; in_progress 2; open 4",
		"seats: free 1; busy 2",
	}, "\n")
	if got != want {
		t.Fatalf("RenderDashboardSummary() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(input.StatusCounts, original) {
		t.Fatalf("RenderDashboardSummary() mutated input: %#v", input.StatusCounts)
	}
}

func TestRenderDashboardSummaryHandlesEmptyCountsAndWidth(t *testing.T) {
	got := RenderDashboardSummary(DashboardSummaryInput{}, 12)
	want := "Summary\nrepositorie…\nsessions: 0\nbeads: 0\nseats: free…"
	if got != want {
		t.Fatalf("RenderDashboardSummary() = %q, want %q", got, want)
	}
	for _, line := range strings.Split(got, "\n") {
		if length := len([]rune(line)); length > 12 {
			t.Fatalf("line width = %d, want <= 12: %q", length, line)
		}
	}
	if got := RenderDashboardSummary(DashboardSummaryInput{}, 0); got != "" {
		t.Fatalf("RenderDashboardSummary(zero width) = %q, want empty", got)
	}
}
