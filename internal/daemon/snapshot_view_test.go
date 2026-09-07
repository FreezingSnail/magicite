package daemon

import (
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func TestBeadViewPreservesCompleteBead(t *testing.T) {
	repository := snapshotViewRepo(t)
	bead := bd.Bead{
		ID:                 "magicite-1",
		Title:              "complete task",
		Description:        "description",
		Design:             "design",
		AcceptanceCriteria: "acceptance",
		Status:             "open",
		Priority:           1,
		IssueType:          "task",
		Assignee:           "agent",
		Owner:              "owner",
		Parent:             "magicite",
		CreatedAt:          "2026-09-07T20:00:00Z",
		CreatedBy:          "creator",
		UpdatedAt:          "2026-09-07T20:01:00Z",
		StartedAt:          "2026-09-07T20:02:00Z",
		ClosedAt:           "2026-09-07T20:03:00Z",
		DeferredUntil:      "2026-09-08T00:00:00Z",
		CloseReason:        "implemented",
		DependencyCount:    2,
		DependentCount:     3,
		CommentCount:       4,
		Labels:             []string{"staged", "difficulty:high"},
		Dependencies: []bd.Dependency{
			{ID: "magicite-0", Title: "dependency", Status: "closed", DependencyType: "blocks"},
			{DependsOnID: "magicite-parent", Type: "parent-child"},
		},
	}
	facts := workflowFacts{
		Dispatch: workflowFact{Known: true, Eligible: true, Reason: "ready"},
		Review:   workflowFact{Known: true, Reason: "not an epic"},
	}

	got := beadView(repository, bead, facts)
	if got.ID != bead.ID || got.Repo != repository.Name || got.Title != bead.Title || got.Status != bead.Status || got.Priority != bead.Priority || got.IssueType != bead.IssueType {
		t.Fatalf("beadView() identity = %#v", got)
	}
	assertSnapshotViewString(t, "Description", got.Description, bead.Description)
	assertSnapshotViewString(t, "Design", got.Design, bead.Design)
	assertSnapshotViewString(t, "AcceptanceCriteria", got.AcceptanceCriteria, bead.AcceptanceCriteria)
	assertSnapshotViewString(t, "Assignee", got.Assignee, bead.Assignee)
	assertSnapshotViewString(t, "Owner", got.Owner, bead.Owner)
	assertSnapshotViewString(t, "Parent", got.Parent, bead.Parent)
	assertSnapshotViewString(t, "CreatedBy", got.CreatedBy, bead.CreatedBy)
	assertSnapshotViewString(t, "CloseReason", got.CloseReason, bead.CloseReason)
	assertSnapshotViewTime(t, "CreatedAt", got.CreatedAt, bead.CreatedAt)
	assertSnapshotViewTime(t, "UpdatedAt", got.UpdatedAt, bead.UpdatedAt)
	assertSnapshotViewTime(t, "StartedAt", got.StartedAt, bead.StartedAt)
	assertSnapshotViewTime(t, "ClosedAt", got.ClosedAt, bead.ClosedAt)
	assertSnapshotViewTime(t, "DeferredUntil", got.DeferredUntil, bead.DeferredUntil)
	if got.DependencyCount != bead.DependencyCount || got.DependentCount != bead.DependentCount || got.CommentCount != bead.CommentCount {
		t.Fatalf("beadView() counts = %#v", got)
	}
	wantDependencies := []struct{ id, title, status, typ string }{{"magicite-0", "dependency", "closed", "blocks"}, {"magicite-parent", "", "", "parent-child"}}
	for index, want := range wantDependencies {
		if dependency := got.Dependencies[index]; dependency.ID != want.id || dependency.Title != want.title || dependency.Status != want.status || dependency.Type != want.typ {
			t.Fatalf("dependency[%d] = %#v, want %#v", index, dependency, want)
		}
	}
	if !got.Staged || !got.Dispatch.Eligible || got.Dispatch.Reason == nil || *got.Dispatch.Reason != "ready" || got.Review.Eligible || got.Review.Reason == nil || *got.Review.Reason != "not an epic" {
		t.Fatalf("beadView() workflow = dispatch %#v review %#v", got.Dispatch, got.Review)
	}
}

func TestBeadViewNullableMetadataAndExactStagedLabel(t *testing.T) {
	bead := bd.Bead{Labels: []string{" staged", "STAGED", "staged "}, Dependencies: nil}
	got := beadView(snapshotViewRepo(t), bead, workflowFacts{})

	if got.Description != nil || got.Design != nil || got.AcceptanceCriteria != nil || got.Assignee != nil || got.Owner != nil || got.Parent != nil || got.CreatedAt != nil || got.UpdatedAt != nil || got.StartedAt != nil || got.ClosedAt != nil || got.DeferredUntil != nil || got.CloseReason != nil {
		t.Fatalf("beadView() nullable metadata = %#v", got)
	}
	if got.Staged {
		t.Fatalf("beadView() Staged = true for inexact labels")
	}
	if got.Labels == nil || got.Dependencies == nil {
		t.Fatalf("beadView() collections = labels %#v dependencies %#v, want nonnil", got.Labels, got.Dependencies)
	}
}

func TestBeadViewReturnsIndependentSlices(t *testing.T) {
	bead := bd.Bead{Labels: []string{"staged"}, Dependencies: []bd.Dependency{{ID: "dependency", Title: "original"}}}
	got := beadView(snapshotViewRepo(t), bead, workflowFacts{})
	got.Labels[0] = "changed"
	got.Dependencies[0].Title = "changed"

	if !reflect.DeepEqual(bead.Labels, []string{"staged"}) || bead.Dependencies[0].Title != "original" {
		t.Fatalf("bead mutated = %#v", bead)
	}
	second := beadView(snapshotViewRepo(t), bead, workflowFacts{})
	if second.Labels[0] != "staged" || second.Dependencies[0].Title != "original" {
		t.Fatalf("second beadView() = %#v", second)
	}
}

func TestEligibility(t *testing.T) {
	tests := []struct {
		name   string
		fact   workflowFact
		ok     bool
		reason string
	}{
		{name: "eligible", fact: workflowFact{Known: true, Eligible: true}, ok: true},
		{name: "eligible with reason", fact: workflowFact{Known: true, Eligible: true, Reason: "ready"}, ok: true, reason: "ready"},
		{name: "ineligible", fact: workflowFact{Known: true}, reason: ineligibleEligibilityReason},
		{name: "ineligible with reason", fact: workflowFact{Known: true, Reason: "deferred"}, reason: "deferred"},
		{name: "unknown", fact: workflowFact{}, reason: unknownEligibilityReason},
		{name: "unknown with reason", fact: workflowFact{Reason: "ready query failed"}, reason: "ready query failed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := eligibility(test.fact)
			if got.Eligible != test.ok {
				t.Fatalf("eligibility() Eligible = %t, want %t", got.Eligible, test.ok)
			}
			if test.reason == "" {
				if got.Reason != nil {
					t.Fatalf("eligibility() Reason = %q, want nil", *got.Reason)
				}
				return
			}
			if got.Reason == nil || *got.Reason != test.reason {
				t.Fatalf("eligibility() Reason = %#v, want %q", got.Reason, test.reason)
			}
		})
	}
}

func snapshotViewRepo(t *testing.T) repo.Repo {
	t.Helper()
	repository, ok := repo.Make("/snapshot-view", "snapshot", "snapshot", "main")
	if !ok {
		t.Fatal("repo.Make() failed")
	}
	return repository
}

func assertSnapshotViewString(t *testing.T, name string, got *string, want string) {
	t.Helper()
	if got == nil || *got != want {
		t.Fatalf("%s = %#v, want %q", name, got, want)
	}
}

func assertSnapshotViewTime(t *testing.T, name string, got *time.Time, raw string) {
	t.Helper()
	want, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if got == nil || !got.Equal(want) {
		t.Fatalf("%s = %#v, want %s", name, got, want)
	}
}
