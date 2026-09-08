package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestBeadsViewRendersAllSnapshotBeadsInRepositoryIDOrder(t *testing.T) {
	view := NewBeadsView()
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{
		{ID: "z-2", Repo: "zeta", IssueType: "task", Status: "closed", Priority: 2, Staged: true, Title: "Closed\x1b title"},
		{ID: "a-2", Repo: "alpha", IssueType: "epic", Status: "deferred", Priority: 1, Title: "Deferred"},
		{ID: "a-1", Repo: "alpha", IssueType: "bug", Status: "open", Priority: 0, Title: "Open"},
	}})

	if view.Title() != "Beads" {
		t.Fatalf("Title() = %q, want Beads", view.Title())
	}
	if got, want := view.filteredKeys(), []string{"a-1", "a-2", "z-2"}; !equalStrings(got, want) {
		t.Fatalf("row keys = %#v, want %#v", got, want)
	}
	output := view.View(180, 8)
	for _, want := range []string{"Beads: 3/3", "ID", "Type", "State", "Priority", "Staged", "Title", "Repository", "closed", "deferred", "true", "false", "Closed� title"} {
		if !strings.Contains(output, want) {
			t.Errorf("View() missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "\x1b") {
		t.Fatalf("View() retained terminal control: %q", output)
	}
}

func TestBeadsViewUsesPlaceholdersForEmptyStringFields(t *testing.T) {
	values := beadRowValues(wire.BeadResult{})
	if got, want := values, []string{"—", "—", "—", "0", "false", "—", "—"}; !equalStrings(got, want) {
		t.Fatalf("row values = %#v, want %#v", got, want)
	}
}

func TestBeadsViewFiltersExactFieldsAndFreeText(t *testing.T) {
	view := NewBeadsView()
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{
		{ID: "alpha-1", Repo: "alpha", Status: "open", IssueType: "task", Staged: true, Labels: []string{"urgent", "ui"}, Title: "Fix Béads"},
		{ID: "beta-2", Repo: "beta", Status: "closed", IssueType: "epic", Labels: []string{"urgent"}, Title: "Other"},
	}})

	view.setQuery("repo:alpha state:open type:task label:ui staged:true béads")
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "alpha-1" {
		t.Fatalf("filtered selection = %#v, %t; want alpha-1", bead, ok)
	}
	view.setQuery("label:urgent staged:false")
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "beta-2" {
		t.Fatalf("exact field selection = %#v, %t; want beta-2", bead, ok)
	}
	view.setQuery("ALPHA-1")
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "alpha-1" {
		t.Fatalf("ID search = %#v, %t; want alpha-1", bead, ok)
	}
	view.setQuery("missing:value")
	if _, ok := view.SelectedBead(); ok {
		t.Fatal("unknown field matched a row")
	}
	if output := view.View(100, 4); !strings.Contains(output, "Filter error: unknown field missing") || !strings.Contains(output, "No beads match filter") {
		t.Fatalf("unknown field output = %q", output)
	}
}

func TestBeadsViewInputAndSelectionSurviveFilterAndSnapshot(t *testing.T) {
	view := NewBeadsView()
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{
		{ID: "a", Repo: "repo", Title: "Alpha"},
		{ID: "b", Repo: "repo", Title: "Beta"},
		{ID: "c", Repo: "repo", Title: "Gamma"},
	}})
	view.Update(tea.KeyMsg{Type: tea.KeyDown})
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "b" {
		t.Fatalf("down selection = %#v, %t; want b", bead, ok)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	view.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Beta")})
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "b" {
		t.Fatalf("filtered selection = %#v, %t; want b", bead, ok)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if view.query != "" || view.focused {
		t.Fatalf("escape query/focus = %q/%t", view.query, view.focused)
	}
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{
		{ID: "a", Repo: "repo", Title: "Alpha"},
		{ID: "b", Repo: "repo", Title: "New Beta"},
		{ID: "d", Repo: "repo", Title: "Delta"},
	}})
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "b" || bead.Title != "New Beta" {
		t.Fatalf("snapshot selection = %#v, %t; want replacement b", bead, ok)
	}
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{{ID: "a", Repo: "repo"}}})
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "a" {
		t.Fatalf("fallback selection = %#v, %t; want a", bead, ok)
	}
}

func TestBeadsViewEmptyAndNarrow(t *testing.T) {
	view := NewBeadsView()
	if got := view.View(40, 3); got != "No beads in snapshot" {
		t.Fatalf("empty View() = %q", got)
	}
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{{ID: "bead", Repo: "repository", Title: "title"}}})
	view.setQuery("missing")
	if got := view.View(40, 3); !strings.Contains(got, "No beads match filter") {
		t.Fatalf("empty filter View() = %q", got)
	}
	view.setQuery("")
	output := view.View(12, 3)
	if !strings.Contains(output, "ID") || strings.Contains(output, "State") {
		t.Fatalf("narrow View() = %q", output)
	}
}
