package wire

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestSnapshotResultJSONRoundTrip(t *testing.T) {
	capturedAt := time.Date(2026, time.September, 7, 22, 8, 38, 0, time.UTC)
	createdAt := capturedAt.Add(-2 * time.Hour)
	updatedAt := capturedAt.Add(-time.Hour)
	startedAt := capturedAt.Add(-30 * time.Minute)
	closedAt := capturedAt.Add(-time.Minute)
	deferredUntil := capturedAt.Add(time.Hour)
	description := "implement snapshot"
	design := "use typed records"
	acceptance := "all fields round trip"
	assignee := "shiva"
	owner := "FreezingSnail"
	createdBy := "FreezingSnail"
	parent := "magicite-qik"
	closeReason := "done"
	dispatchReason := "seat available"
	reviewReason := "not an epic"

	want := SnapshotResult{
		ModelVersion: 1,
		Generation:   9,
		CapturedAt:   capturedAt,
		Cursor:       42,
		Fresh:        true,
		Stale:        false,
		Runtime: StatusResult{
			Version:        "0.1.0",
			Schema:         Schema,
			Running:        true,
			Draining:       false,
			Repos:          1,
			ImplementerCap: 2,
			Sessions: []SessionResult{{
				Handle:        "session-1",
				Repo:          "magicite",
				Task:          "magicite-qik.1",
				Role:          "implementer",
				Seat:          "shiva",
				Backend:       "kiro",
				Model:         "gpt-5.6-terra",
				Status:        "running",
				Phase:         "implementing",
				UptimeSeconds: 60,
			}},
		},
		Repositories: []RepoResult{{Name: "magicite", Path: "/code/magicite", Prefix: "magicite", Branch: "main"}},
		Seats:        []SeatResult{{Name: "shiva", Role: "implementer", Repo: "magicite", Worktree: "/work/shiva", Task: "magicite-qik.1", Busy: true}},
		Sessions:     []SessionResult{{Handle: "session-1", Repo: "magicite", Task: "magicite-qik.1", Role: "implementer", Seat: "shiva", Backend: "kiro", Model: "gpt-5.6-terra", Status: "running", Phase: "implementing", UptimeSeconds: 60}},
		Beads: []BeadResult{{
			ID:                 "magicite-qik.1",
			Repo:               "magicite",
			Title:              "Add snapshot wire model",
			Description:        &description,
			Design:             &design,
			AcceptanceCriteria: &acceptance,
			Status:             "in_progress",
			Priority:           1,
			IssueType:          "task",
			Assignee:           &assignee,
			Owner:              &owner,
			Parent:             &parent,
			CreatedAt:          &createdAt,
			CreatedBy:          &createdBy,
			UpdatedAt:          &updatedAt,
			StartedAt:          &startedAt,
			ClosedAt:           &closedAt,
			DeferredUntil:      &deferredUntil,
			CloseReason:        &closeReason,
			DependencyCount:    1,
			DependentCount:     2,
			CommentCount:       3,
			Dependencies:       []DependencyResult{{ID: "magicite-qik", Title: "Daemon snapshot read model", Status: "open", Type: "parent-child"}},
			Labels:             []string{"staged", "difficulty:high"},
			Staged:             true,
			Dispatch:           Eligibility{Eligible: true, Reason: &dispatchReason},
			Review:             Eligibility{Eligible: false, Reason: &reviewReason},
		}},
		StatusCounts:     []StatusCount{{Status: "open", Count: 4}},
		RepositoryErrors: []RepositoryError{{Repository: "other", Error: "bd unavailable"}},
	}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var got SnapshotResult
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("snapshot round trip = %#v, want %#v", got, want)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model_version", "generation", "captured_at", "cursor", "fresh", "stale", "runtime", "repositories", "seats", "sessions", "beads", "status_counts", "repository_errors"} {
		if _, ok := fields[name]; !ok {
			t.Errorf("snapshot JSON missing %q", name)
		}
	}
}

func TestSnapshotResultJSONCollectionsAndNullability(t *testing.T) {
	empty := SnapshotResult{
		Repositories:     []RepoResult{},
		Seats:            []SeatResult{},
		Sessions:         []SessionResult{},
		Beads:            []BeadResult{},
		StatusCounts:     []StatusCount{},
		RepositoryErrors: []RepositoryError{},
	}
	encoded, err := json.Marshal(empty)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"repositories", "seats", "sessions", "beads", "status_counts", "repository_errors"} {
		if got := string(fields[name]); got != "[]" {
			t.Errorf("empty %s = %s, want []", name, got)
		}
	}

	var bead BeadResult
	if err := json.Unmarshal([]byte(`{"description":null,"dependencies":[],"labels":[]}`), &bead); err != nil {
		t.Fatal(err)
	}
	if bead.Description != nil {
		t.Errorf("description = %q, want nil", *bead.Description)
	}
	if bead.Dependencies == nil || bead.Labels == nil {
		t.Errorf("empty bead arrays = dependencies %#v, labels %#v, want nonnil", bead.Dependencies, bead.Labels)
	}
	var omitted, explicit SnapshotResult
	if err := json.Unmarshal([]byte(`{}`), &omitted); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(`{"beads":[]}`), &explicit); err != nil {
		t.Fatal(err)
	}
	if omitted.Beads != nil || explicit.Beads == nil {
		t.Errorf("omitted and explicit empty beads must differ: omitted %#v, explicit %#v", omitted.Beads, explicit.Beads)
	}
}

func TestLegacyPayloadJSONUnchanged(t *testing.T) {
	task := TaskResult{ID: "magicite-1", Repo: "magicite", Title: "task", Status: "open", Difficulty: "low", Priority: 1, Labels: []string{"staged"}}
	encoded, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"id":"magicite-1","repo":"magicite","title":"task","status":"open","difficulty":"low","priority":1,"labels":["staged"]}`
	if got := string(encoded); got != want {
		t.Errorf("task JSON = %s, want %s", got, want)
	}
}
