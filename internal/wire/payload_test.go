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

func TestMetricsResultJSONRoundTrip(t *testing.T) {
	startedAt := time.Date(2026, time.September, 8, 4, 5, 6, 0, time.UTC)
	sampledAt := startedAt.Add(time.Minute)
	want := MetricsResult{
		StartedAt:     startedAt,
		UptimeSeconds: 60,
		Lifecycle: []MetricsCount{
			{Key: "pickup", Count: 1},
			{Key: "complete", Count: 2},
			{Key: "land", Count: 3},
			{Key: "close", Count: 4},
			{Key: "review", Count: 5},
			{Key: "verdict", Count: 6},
			{Key: "recovery", Count: 7},
			{Key: "warn", Count: 8},
			{Key: "error", Count: 9},
		},
		Land: []MetricsCount{
			{Key: "ok", Count: 10},
			{Key: "conflict", Count: 11},
			{Key: "gate_failed", Count: 12},
			{Key: "failed", Count: 13},
		},
		Sessions:       SessionGauges{Active: 1, Peak: 2, Completed: 3, Failed: 4},
		Roles:          []RoleDuration{{Role: "implementer", Sessions: 5, TotalSeconds: 360}},
		Queue:          []RepoQueueDepth{{Repo: "magicite", Depth: 6}},
		QueueTotal:     6,
		QueueSampledAt: &sampledAt,
		Bus:            BusMetrics{Published: 14, Dropped: 15},
	}

	encoded, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	const expected = `{"started_at":"2026-09-08T04:05:06Z","uptime_seconds":60,"lifecycle":[{"key":"pickup","count":1},{"key":"complete","count":2},{"key":"land","count":3},{"key":"close","count":4},{"key":"review","count":5},{"key":"verdict","count":6},{"key":"recovery","count":7},{"key":"warn","count":8},{"key":"error","count":9}],"land":[{"key":"ok","count":10},{"key":"conflict","count":11},{"key":"gate_failed","count":12},{"key":"failed","count":13}],"sessions":{"active":1,"peak":2,"completed":3,"failed":4},"roles":[{"role":"implementer","sessions":5,"total_seconds":360}],"queue":[{"repo":"magicite","depth":6}],"queue_total":6,"queue_sampled_at":"2026-09-08T04:06:06Z","bus":{"published":14,"dropped":15}}`
	if got := string(encoded); got != expected {
		t.Errorf("metrics JSON = %s, want %s", got, expected)
	}

	var got MetricsResult
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("metrics round trip = %#v, want %#v", got, want)
	}
}

func TestMetricsResultJSONZeroCollectionsAndSample(t *testing.T) {
	encoded, err := json.Marshal(MetricsResult{
		Lifecycle: []MetricsCount{},
		Land:      []MetricsCount{},
		Roles:     []RoleDuration{},
		Queue:     []RepoQueueDepth{},
	})
	if err != nil {
		t.Fatal(err)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"lifecycle", "land", "roles", "queue"} {
		if got := string(fields[name]); got != "[]" {
			t.Errorf("empty %s = %s, want []", name, got)
		}
	}
	if got := string(fields["queue_sampled_at"]); got != "null" {
		t.Errorf("nil queue_sampled_at = %s, want null", got)
	}
	if got := string(fields["sessions"]); got != `{"active":0,"peak":0,"completed":0,"failed":0}` {
		t.Errorf("zero sessions = %s", got)
	}
	if got := string(fields["bus"]); got != `{"published":0,"dropped":0}` {
		t.Errorf("zero bus = %s", got)
	}
}
