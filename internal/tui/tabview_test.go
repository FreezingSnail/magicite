package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestKeepSelection(t *testing.T) {
	for _, test := range []struct {
		name     string
		keys     []string
		previous Selection
		want     Selection
	}{
		{"present after sort", []string{"two", "one", "three"}, Selection{Key: "one", Index: 0}, Selection{Key: "one", Index: 1}},
		{"removed chooses previous index", []string{"one", "three"}, Selection{Key: "two", Index: 1}, Selection{Key: "three", Index: 1}},
		{"removed last clamps", []string{"one", "two"}, Selection{Key: "three", Index: 5}, Selection{Key: "two", Index: 1}},
		{"empty previous chooses first", []string{"one", "two"}, Selection{Index: -1}, Selection{Key: "one", Index: 0}},
		{"empty list", nil, Selection{Key: "one", Index: 0}, Selection{Index: -1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := KeepSelection(test.keys, test.previous); got != test.want {
				t.Fatalf("KeepSelection(%#v, %#v) = %#v, want %#v", test.keys, test.previous, got, test.want)
			}
		})
	}
}

func TestTabRegistryRoutesFocusedViewAndPreservesResizeSelection(t *testing.T) {
	beads := stubTabView{title: "Beads", selected: Selection{Key: "bead-2", Index: 1}}
	seats := stubTabView{title: "Seats"}
	registry := NewTabRegistry(map[Tab]TabView{TabBeads: beads, TabSeats: seats})

	updated, command := registry.Update(TabBeads, tea.KeyMsg{Type: tea.KeyDown})
	if command != nil {
		t.Fatal("Update() command != nil")
	}
	if got := updated.view(TabBeads).(stubTabView); got.updates != 1 || got.selected != beads.selected {
		t.Fatalf("beads = %#v", got)
	}
	if got := updated.view(TabSeats).(stubTabView); got.updates != 0 {
		t.Fatalf("seats = %#v", got)
	}
	if got := updated.View(TabBeads, 20, 5); got != "Beads" {
		t.Fatalf("View() = %q", got)
	}
	if got := updated.view(TabBeads).(stubTabView).selected; got != beads.selected {
		t.Fatalf("resize changed selection: %#v", got)
	}
	if got := updated.View(TabEvents, 20, 5); got != EmptyTabPlaceholder {
		t.Fatalf("unregistered View() = %q", got)
	}
}

func TestTabRegistrySnapshotsEveryViewAndConcatenatesEnabledHints(t *testing.T) {
	dashboard := stubTabView{title: "Dashboard", hints: []Hint{{Key: "d", Label: "dashboard", Enabled: true}, {Key: "x", Label: "disabled"}}}
	beads := stubTabView{title: "Beads", hints: []Hint{{Key: "b", Label: "beads", Enabled: true}}}
	seats := stubTabView{title: "Seats", hints: []Hint{{Key: "s", Label: "seats", Enabled: true}}}
	registry := NewTabRegistry(map[Tab]TabView{TabDashboard: dashboard, TabBeads: beads, TabSeats: seats})

	snapshot := wire.SnapshotResult{Generation: 7}
	updated := registry.Snapshot(snapshot)
	for _, tab := range []Tab{TabDashboard, TabBeads, TabSeats} {
		if got := updated.view(tab).(stubTabView); got.snapshots != 1 || got.generation != snapshot.Generation {
			t.Fatalf("%s snapshot = %#v", tab, got)
		}
	}
	if got, want := updated.Hints(TabDashboard), []Hint{{Key: "d", Label: "dashboard", Enabled: true}}; !equalHints(got, want) {
		t.Fatalf("Hints() = %#v, want %#v", got, want)
	}
	if got, want := updated.AllHints(), []Hint{{Key: "d", Label: "dashboard", Enabled: true}, {Key: "b", Label: "beads", Enabled: true}, {Key: "s", Label: "seats", Enabled: true}}; !equalHints(got, want) {
		t.Fatalf("AllHints() = %#v, want %#v", got, want)
	}
}

type stubTabView struct {
	title     string
	hints     []Hint
	selected  Selection
	updates   int
	snapshots int
	generation uint64
}

func (view stubTabView) Title() string { return view.title }
func (view stubTabView) Update(tea.Msg) (TabView, tea.Cmd) {
	view.updates++
	return view, nil
}
func (view stubTabView) View(int, int) string { return view.title }
func (view stubTabView) Hints() []Hint { return view.hints }
func (view stubTabView) Snapshot(snapshot wire.SnapshotResult) TabView {
	view.snapshots++
	view.generation = snapshot.Generation
	return view
}

func equalHints(left, right []Hint) bool {
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
