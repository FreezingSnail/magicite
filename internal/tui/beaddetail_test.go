package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestBeadDetailRendersSanitizedSnapshotSections(t *testing.T) {
	created := time.Date(2026, time.September, 7, 20, 3, 0, 0, time.UTC)
	body := "body\x1b text with wide 界 rune"
	design := "design"
	acceptance := "acceptance"
	assignee := "agent"
	owner := "owner"
	parent := "parent-1"
	creator := "creator"
	closeReason := "done"
	reason := "blocked\x1b by daemon"
	detail := NewBeadDetail(wire.BeadResult{
		ID: "bead-1", Repo: "repo", IssueType: "task", Status: "closed", Priority: 2, Staged: true, Title: "title\x1b",
		Description: &body, Design: &design, AcceptanceCriteria: &acceptance, Labels: []string{"z", "a"}, Assignee: &assignee, Owner: &owner, Parent: &parent,
		CreatedAt: &created, UpdatedAt: &created, StartedAt: &created, ClosedAt: &created, DeferredUntil: &created, CreatedBy: &creator, CloseReason: &closeReason,
		DependencyCount: 1, DependentCount: 2, CommentCount: 3,
		Dependencies: []wire.DependencyResult{{ID: "blocker", Status: "open", Title: "blocks this", Type: "blocks"}, {ID: "child", Status: "closed", Type: "parent-child"}},
		Dispatch:     wire.Eligibility{Reason: &reason}, Review: wire.Eligibility{Eligible: true},
	})
	output := detail.content
	for _, want := range []string{
		"Identity", "ID: bead-1", "Title", "title�", "Body", "body� text", "Design", "Acceptance criteria", "Labels", "a, z",
		"Assignment", "Assignee: agent", "Owner: owner", "Parent", "Timestamps", "Deferred until", "Depends on", "blocker [open]", "Blocks", "child [closed]",
		"Counts", "Children: 2", "Comments: 3", "Close and comment metadata", "Close reason: done", "Eligibility", "Dispatch: ineligible — blocked� by daemon", "Review: eligible",
	} {
		if !strings.Contains(output, want) {
			t.Errorf("View() missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "\x1b") {
		t.Fatalf("View() retained terminal control: %q", output)
	}
}

func TestBeadDetailOmitsAbsentSectionsAndUnknownEligibility(t *testing.T) {
	detail := NewBeadDetail(wire.BeadResult{ID: "bead-1"})
	output := detail.content
	for _, absent := range []string{"Body\n", "Design\n", "Acceptance criteria", "Labels\n", "Assignment", "Parent\n", "Timestamps", "Deferred until", "Depends on\n", "Blocks\n", "Close and comment metadata"} {
		if strings.Contains(output, absent) {
			t.Errorf("View() retained absent section %q: %q", absent, output)
		}
	}
	if !strings.Contains(output, "Dispatch: unknown") || !strings.Contains(output, "Review: unknown") {
		t.Fatalf("View() eligibility = %q", output)
	}
}

func TestBeadDetailScrollCloseRefreshAndRemoval(t *testing.T) {
	body := strings.Repeat("scrolling body ", 80)
	detail := NewBeadDetail(wire.BeadResult{ID: "bead-1", Description: &body})
	detail.View(20, 4)
	detail.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	offset := detail.viewport.YOffset
	if offset == 0 {
		t.Fatal("page down did not scroll")
	}
	updated := "updated " + body
	detail.Refresh(wire.BeadResult{ID: "bead-1", Description: &updated})
	if detail.viewport.YOffset != offset {
		t.Fatalf("Refresh() offset = %d, want %d", detail.viewport.YOffset, offset)
	}
	detail.Refresh(wire.BeadResult{})
	if got := detail.View(20, 4); !strings.Contains(got, "Bead removed") {
		t.Fatalf("removed View() = %q", got)
	}
	detail.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if detail.IsOpen() {
		t.Fatal("Esc did not close detail")
	}
}

func TestBeadsViewOpensAndClosesDetailWithoutChangingSelection(t *testing.T) {
	view := NewBeadsView()
	view.Snapshot(wire.SnapshotResult{Beads: []wire.BeadResult{{ID: "a"}, {ID: "b", Description: stringPointer("detail")}}})
	view.Update(tea.KeyMsg{Type: tea.KeyDown})
	view.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if view.detail == nil || !view.detail.IsOpen() || !strings.Contains(view.View(40, 20), "detail") {
		t.Fatalf("Enter detail = %#v, view %q", view.detail, view.View(40, 20))
	}
	view.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if view.detail.IsOpen() {
		t.Fatal("Esc did not return focus to table")
	}
	if bead, ok := view.SelectedBead(); !ok || bead.ID != "b" {
		t.Fatalf("selection after close = %#v, %t", bead, ok)
	}
}

func stringPointer(value string) *string { return &value }
