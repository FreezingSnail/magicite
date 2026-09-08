package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestEventsViewBoundsHistoryAndShowsDroppedOldest(t *testing.T) {
	view := NewEventsView(2)
	view.Append(testEvent(1, "one"))
	view.Append(testEvent(2, "two"))
	view.Append(testEvent(3, "three"))

	if view.Dropped != 1 {
		t.Fatalf("Dropped = %d, want 1", view.Dropped)
	}
	rows := view.visibleRows()
	if len(rows) != 2 || rows[0].event.Seq != 2 || rows[1].event.Seq != 3 {
		t.Fatalf("rows = %#v, want sequences 2, 3", rows)
	}
	got := view.View(120, 5)
	if !strings.Contains(got, "dropped-oldest:1") || strings.Contains(got, " one") {
		t.Fatalf("View() = %q", got)
	}
}

func TestEventsViewFiltersExactFieldsAndFreeText(t *testing.T) {
	view := NewEventsView(4)
	view.Append(wire.Event{Seq: 1, Level: "info", Kind: wire.KindPickup, Repo: "magicite", Task: "one", Seat: "ifrit", Fields: map[string]string{"summary": "prepare release"}})
	view.Append(wire.Event{Seq: 2, Level: "warn", Kind: wire.KindLand, Repo: "other", Task: "two", Seat: "ramuh", Fields: map[string]string{"summary": "land blocked"}})

	for _, test := range []struct {
		filter string
		want   uint64
	}{
		{"level:info", 1}, {"repo:other", 2}, {"task:one", 1}, {"seat:ramuh", 2}, {"blocked", 2}, {"level:info prepare", 1}, {"level:INFO", 0},
	} {
		t.Run(test.filter, func(t *testing.T) {
			view.SetFilter(test.filter)
			rows := view.visibleRows()
			if test.want == 0 {
				if len(rows) != 0 {
					t.Fatalf("filter %q rows = %#v, want none", test.filter, rows)
				}
				return
			}
			if len(rows) != 1 || rows[0].event.Seq != test.want {
				t.Fatalf("filter %q rows = %#v", test.filter, rows)
			}
		})
	}
}

func TestEventsViewFollowPinsSelectionAfterManualMovement(t *testing.T) {
	view := NewEventsView(4)
	view.Append(testEvent(1, "one"))
	view.Append(testEvent(2, "two"))
	if !view.Follow || view.selection.Key != "event:2" {
		t.Fatalf("initial follow = %v, selection = %#v", view.Follow, view.selection)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyDown})
	if view.Follow || view.selection.Key != "event:2" {
		t.Fatalf("manual movement follow = %v, selection = %#v", view.Follow, view.selection)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyUp})
	view.Append(testEvent(3, "three"))
	if view.selection.Key != "event:1" {
		t.Fatalf("pinned selection = %#v, want event:1", view.selection)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("f")})
	if !view.Follow || view.selection.Key != "event:3" {
		t.Fatalf("follow selection = %#v", view.selection)
	}
}

func TestEventsViewMarksRangesAndSortsExpandedFields(t *testing.T) {
	view := NewEventsView(3)
	view.Append(wire.Event{Seq: 4, Kind: wire.KindWarn, Fields: map[string]string{"zebra": "last", "alpha": "first", "summary": "notice\x1b"}})
	view.Mark(EventMark{Kind: EventMarkMissedRange, From: 5, To: 7, Detail: "retry\nstream"})
	view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	mark := view.View(120, 12)
	for _, want := range []string{"missed-range 5-7", "from: 5", "to: 7", "notice: missed-range"} {
		if !strings.Contains(mark, want) {
			t.Fatalf("marker View() missing %q: %q", want, mark)
		}
	}
	view.Update(tea.KeyMsg{Type: tea.KeyUp})
	got := view.View(120, 12)
	if !strings.Contains(got, "alpha: first") || !strings.Contains(got, "zebra: last") || strings.Index(got, "alpha: first") > strings.Index(got, "zebra: last") || strings.Contains(got, "\x1b") || strings.Contains(got, "\nstream") {
		t.Fatalf("View() = %q", got)
	}
}

func TestEventsViewSnapshotDoesNotChangeLocalHistory(t *testing.T) {
	view := NewEventsView(1)
	view.Append(testEvent(9, "local"))
	if got := view.Snapshot(wire.SnapshotResult{Generation: 22}); got != view {
		t.Fatalf("Snapshot() = %#v, want original view", got)
	}
	if rows := view.visibleRows(); len(rows) != 1 || rows[0].event.Seq != 9 {
		t.Fatalf("snapshot changed rows: %#v", rows)
	}
}

func testEvent(sequence uint64, summary string) wire.Event {
	return wire.Event{Seq: sequence, Time: time.Date(2026, time.September, 8, 1, 2, int(sequence), 0, time.UTC), Level: "info", Kind: wire.KindComplete, Fields: map[string]string{"summary": summary}}
}
