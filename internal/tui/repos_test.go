package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestReposViewRendersSortedIdentityAndHealth(t *testing.T) {
	view := NewReposView()
	view.Snapshot(wire.SnapshotResult{
		Repositories: []wire.RepoResult{
			{Name: "zeta", Prefix: "z", Branch: "main", Path: "/not-rendered"},
			{Name: "alpha", Prefix: "a", Branch: "trunk"},
			{Name: "broken", Prefix: "b", Branch: "dev"},
		},
		RepositoryErrors: []wire.RepositoryError{{Repository: "broken", Error: "bd unavailable"}},
	})

	if view.Title() != "Repositories" {
		t.Fatalf("Title() = %q, want Repositories", view.Title())
	}
	if got, want := repoRowKeys(view.rows), []string{"alpha", "broken", "zeta"}; !equalStrings(got, want) {
		t.Fatalf("row keys = %#v, want %#v", got, want)
	}
	output := view.View(180, 8)
	for _, want := range []string{"Repository", "Prefix", "Branch", "Beads", "Health", "Detail", "alpha", "broken", "zeta", "ok (healthy)", "error (read failed)", "bd unavailable", "—"} {
		if !strings.Contains(output, want) {
			t.Errorf("View() missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "/not-rendered") || strings.Contains(output, "\x1b") {
		t.Fatalf("View() rendered path or terminal control: %q", output)
	}
}

func TestReposViewUsesPlaceholdersAndNeverCountsLocalBeads(t *testing.T) {
	view := NewReposView()
	view.Snapshot(wire.SnapshotResult{
		Repositories: []wire.RepoResult{{Name: "empty"}},
		Beads:        []wire.BeadResult{{Repo: "empty"}, {Repo: "empty"}},
	})
	if got, want := repoRowValues(view.rows[0]), []string{"empty", "—", "—", "—", "ok (healthy)", "—"}; !equalStrings(got, want) {
		t.Fatalf("row values = %#v, want %#v", got, want)
	}
}

func TestReposViewKeepsSelectionByRepositoryNameAcrossSnapshots(t *testing.T) {
	view := NewReposView()
	view.Snapshot(wire.SnapshotResult{Repositories: []wire.RepoResult{{Name: "zeta"}, {Name: "alpha"}}})
	view.Update(tea.KeyMsg{Type: tea.KeyDown})
	if repository, ok := view.SelectedRepo(); !ok || repository.Name != "zeta" {
		t.Fatalf("selected repository = %#v, %t; want zeta", repository, ok)
	}

	view.Snapshot(wire.SnapshotResult{Stale: true, Repositories: []wire.RepoResult{{Name: "zeta"}, {Name: "alpha"}, {Name: "new"}}})
	if repository, ok := view.SelectedRepo(); !ok || repository.Name != "zeta" {
		t.Fatalf("selection after replacement = %#v, %t; want zeta", repository, ok)
	}
	view.Update(tea.KeyMsg{Type: tea.KeyUp})
	if repository, ok := view.SelectedRepo(); !ok || repository.Name != "new" {
		t.Fatalf("selection after move = %#v, %t; want new", repository, ok)
	}
}

func TestReposViewSelectedErrorShowsSanitizedFullDetail(t *testing.T) {
	view := NewReposView()
	view.Snapshot(wire.SnapshotResult{
		Repositories:     []wire.RepoResult{{Name: "broken"}},
		RepositoryErrors: []wire.RepositoryError{{Repository: "broken", Error: "bd\x1b[31m failed\nagain"}},
	})
	output := view.View(120, 4)
	for _, want := range []string{"bd�[31m failed�again", "Detail: bd�[31m failed�again"} {
		if !strings.Contains(output, want) {
			t.Errorf("View() missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "\x1b") || strings.Contains(output, "\nagain") {
		t.Fatalf("View() retained unsanitized detail: %q", output)
	}
}

func TestReposViewEmptyAndNarrow(t *testing.T) {
	view := NewReposView()
	if got := view.View(40, 3); got != "No repositories configured" {
		t.Fatalf("empty View() = %q", got)
	}
	view.Snapshot(wire.SnapshotResult{Repositories: []wire.RepoResult{{Name: "repository"}}})
	output := view.View(12, 3)
	if strings.Contains(output, "Branch") || strings.Contains(output, "Health") {
		t.Fatalf("narrow View() retained trailing columns: %q", output)
	}
	if !strings.Contains(output, "Repo") {
		t.Fatalf("narrow View() lost identity column: %q", output)
	}
}
