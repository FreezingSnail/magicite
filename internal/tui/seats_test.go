package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestSeatsViewRendersSortedSeatsAndJoinedSessions(t *testing.T) {
	view := NewSeatsView()
	view.Snapshot(wire.SnapshotResult{
		Seats: []wire.SeatResult{
			{Name: "zeta", Role: "reviewer", Repo: "magicite", Task: "review-2", Busy: true, Worktree: "/work/zeta"},
			{Name: "alpha", Role: "implementer"},
			{Name: "beta", Role: "implementer", Repo: "magicite", Task: "build-1", Busy: true},
		},
		Sessions: []wire.SessionResult{{
			Handle: "session-beta", Seat: "beta", Backend: "kiro", Model: "gpt", UptimeSeconds: 61,
		}},
	})

	if view.Title() != "Seats" {
		t.Fatalf("Title() = %q, want Seats", view.Title())
	}
	if got := seatRowKeys(view.rows); !equalStrings(got, []string{"alpha", "beta", "zeta"}) {
		t.Fatalf("row keys = %#v", got)
	}
	output := view.View(220, 5)
	for _, want := range []string{"Seat", "Role", "Repository", "State", "Task", "Session", "Backend", "Model", "Uptime", "Worktree", "busy (assigned)", "session-beta", "1m1s", "—"} {
		if !strings.Contains(output, want) {
			t.Errorf("View() missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "\x1b") {
		t.Fatalf("View() contains terminal control: %q", output)
	}
}

func TestSeatsViewUsesIdlePlaceholdersAndOptionalWorktreeColumn(t *testing.T) {
	view := NewSeatsView()
	view.Snapshot(wire.SnapshotResult{Seats: []wire.SeatResult{{Name: "idle", Role: "implementer"}}})
	if got := seatRowValues(view.rows[0], view.worktree); !equalStrings(got, []string{"idle", "implementer", "—", "idle (available)", "—", "—", "—", "—", "—"}) {
		t.Fatalf("idle row = %#v", got)
	}
	if strings.Contains(view.View(120, 3), "Worktree") {
		t.Fatal("empty worktree values enabled Worktree column")
	}

	view.Snapshot(wire.SnapshotResult{Seats: []wire.SeatResult{{Name: "idle", Worktree: "/work/idle"}}})
	if !view.worktree || !strings.Contains(view.View(220, 3), "Worktree") {
		t.Fatal("daemon worktree did not enable Worktree column")
	}
	if !strings.Contains(view.View(220, 3), "/work/idle") {
		t.Fatal("worktree value missing")
	}
}

func TestSeatsViewKeepsSelectionBySeatNameAcrossSnapshots(t *testing.T) {
	view := NewSeatsView()
	view.Snapshot(wire.SnapshotResult{Seats: []wire.SeatResult{
		{Name: "b", Role: "reviewer"},
		{Name: "a", Role: "implementer"},
	}})
	view.Update(tea.KeyMsg{Type: tea.KeyDown})
	if seat, ok := view.SelectedSeat(); !ok || seat.Name != "b" {
		t.Fatalf("selected seat = %#v, %t; want b", seat, ok)
	}

	view.Snapshot(wire.SnapshotResult{Seats: []wire.SeatResult{
		{Name: "b", Role: "implementer"},
		{Name: "a", Role: "reviewer"},
		{Name: "c", Role: "reviewer"},
	}})
	if seat, ok := view.SelectedSeat(); !ok || seat.Name != "b" {
		t.Fatalf("selection after replacement = %#v, %t; want b", seat, ok)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyDown})
	if seat, ok := view.SelectedSeat(); !ok || seat.Name != "a" {
		t.Fatalf("selection after up = %#v, %t; want a", seat, ok)
	}
}

func TestSeatsViewEmptyAndNarrow(t *testing.T) {
	view := NewSeatsView()
	if got := view.View(40, 3); got != "No seats configured" {
		t.Fatalf("empty View() = %q", got)
	}
	view.Snapshot(wire.SnapshotResult{Seats: []wire.SeatResult{{Name: "seat", Role: "role"}}})
	output := view.View(12, 3)
	if strings.Contains(output, "Repository") || strings.Contains(output, "State") {
		t.Fatalf("narrow View() retained trailing columns: %q", output)
	}
	if !strings.Contains(output, "Seat") {
		t.Fatalf("narrow View() lost identity column: %q", output)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
