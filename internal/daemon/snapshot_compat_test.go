package daemon

import (
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestSnapshotCompatibilityPreservesCompleteClosedBead(t *testing.T) {
	repository, ok := repo.Make("/snapshot-compat", "compat", "compat", "main")
	if !ok {
		t.Fatal("repo.Make() failed")
	}
	closed := snapshotCompatibilityBead()
	open := bd.Bead{ID: "compat-open", Title: "open", Status: "open", Labels: []string{}, Dependencies: []bd.Dependency{}}
	facts := workflowFacts{
		Dispatch: workflowFact{Known: true, Eligible: true, Reason: "ready"},
		Review:   workflowFact{Known: true, Reason: "closed"},
	}

	views := []wire.BeadResult{beadView(repository, closed, facts), beadView(repository, open, workflowFacts{})}
	if got := []string{views[0].ID, views[1].ID}; !reflect.DeepEqual(got, []string{"compat-closed", "compat-open"}) {
		t.Fatalf("bead view order = %#v", got)
	}

	captured := time.Date(2026, time.September, 7, 22, 0, 0, 0, time.UTC)
	description := "complete closed record"
	design := "retain every source detail"
	acceptance := "transport preserves contract"
	assignee := "shiva"
	owner := "FreezingSnail"
	parent := "magicite-qik"
	createdBy := "creator"
	closeReason := "implemented"
	dispatchReason := "ready"
	reviewReason := "closed"
	want := wire.BeadResult{
		ID:                 "compat-closed",
		Repo:               "compat",
		Title:              "closed snapshot compatibility",
		Description:        &description,
		Design:             &design,
		AcceptanceCriteria: &acceptance,
		Status:             "closed",
		Priority:           1,
		IssueType:          "task",
		Assignee:           &assignee,
		Owner:              &owner,
		Parent:             &parent,
		CreatedAt:          pointerTime(captured),
		CreatedBy:          &createdBy,
		UpdatedAt:          pointerTime(captured.Add(time.Minute)),
		StartedAt:          pointerTime(captured.Add(2 * time.Minute)),
		ClosedAt:           pointerTime(captured.Add(3 * time.Minute)),
		DeferredUntil:      pointerTime(captured.Add(4 * time.Minute)),
		CloseReason:        &closeReason,
		DependencyCount:    2,
		DependentCount:     3,
		CommentCount:       4,
		Dependencies: []wire.DependencyResult{
			{ID: "compat-parent", Title: "parent", Status: "closed", Type: "parent-child"},
			{ID: "compat-blocker", Title: "blocker", Status: "open", Type: "blocks"},
		},
		Labels:   []string{"staged", "difficulty:high", "release:2026"},
		Staged:   true,
		Dispatch: wire.Eligibility{Eligible: true, Reason: &dispatchReason},
		Review:   wire.Eligibility{Reason: &reviewReason},
	}
	if !reflect.DeepEqual(views[0], want) {
		t.Fatalf("closed bead view = %#v, want %#v", views[0], want)
	}
	if views[1].Labels == nil || views[1].Dependencies == nil || views[1].Description != nil || views[1].ClosedAt != nil {
		t.Fatalf("open bead optional values and arrays = %#v", views[1])
	}
}

func snapshotCompatibilityBead() bd.Bead {
	return bd.Bead{
		ID:                 "compat-closed",
		Title:              "closed snapshot compatibility",
		Description:        "complete closed record",
		Design:             "retain every source detail",
		AcceptanceCriteria: "transport preserves contract",
		Status:             "closed",
		Priority:           1,
		IssueType:          "task",
		Assignee:           "shiva",
		Owner:              "FreezingSnail",
		Parent:             "magicite-qik",
		CreatedAt:          "2026-09-07T22:00:00Z",
		CreatedBy:          "creator",
		UpdatedAt:          "2026-09-07T22:01:00Z",
		StartedAt:          "2026-09-07T22:02:00Z",
		ClosedAt:           "2026-09-07T22:03:00Z",
		DeferredUntil:      "2026-09-07T22:04:00Z",
		CloseReason:        "implemented",
		DependencyCount:    2,
		DependentCount:     3,
		CommentCount:       4,
		Labels:             []string{"staged", "difficulty:high", "release:2026"},
		Dependencies: []bd.Dependency{
			{ID: "compat-parent", Title: "parent", Status: "closed", DependencyType: "parent-child"},
			{DependsOnID: "compat-blocker", Title: "blocker", Status: "open", Type: "blocks"},
		},
	}
}

func pointerTime(value time.Time) *time.Time { return &value }
