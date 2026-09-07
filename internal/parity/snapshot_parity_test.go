package parity

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

var snapshotParitySocketSequence atomic.Uint64

func TestSnapshotParityTransportPreservesClosedRecordsAndLegacyCalls(t *testing.T) {
	want := snapshotParityResult()
	legacy := []wire.TaskResult{{ID: "compat-open", Repo: "alpha", Title: "open", Status: "open", Difficulty: "high", Priority: 2, Labels: []string{"difficulty:high"}}}
	socket := snapshotParitySocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	requests := make(chan wire.Request, 2)
	go func() {
		defer close(requests)
		for range 2 {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			request, err := wire.NewDecoder(conn).Request()
			if err == nil {
				requests <- request
				var result any = legacy
				if request.Command == "snapshot" {
					result = want
				}
				encoded, marshalErr := json.Marshal(result)
				if marshalErr == nil {
					_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: encoded})
				}
			}
			_ = conn.Close()
		}
	}()

	transport := client.New(client.Options{Socket: socket})
	got, err := transport.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Snapshot() = %#v, want %#v", got, want)
	}
	if got.Beads[0].Status != "closed" || got.Beads[0].ClosedAt == nil || got.Beads[0].Description == nil || got.Beads[0].Design == nil || got.Beads[0].AcceptanceCriteria == nil {
		t.Fatalf("closed bead details = %#v", got.Beads[0])
	}
	if !reflect.DeepEqual(got.Beads[0].Labels, []string{"staged", "difficulty:high", "release:2026"}) || !reflect.DeepEqual(got.Beads[0].Dependencies, []wire.DependencyResult{{ID: "compat-parent", Title: "parent", Status: "closed", Type: "parent-child"}, {ID: "compat-blocker", Title: "blocker", Status: "open", Type: "blocks"}}) {
		t.Fatalf("closed bead arrays = %#v", got.Beads[0])
	}
	if ids := []string{got.Beads[0].ID, got.Beads[1].ID}; !reflect.DeepEqual(ids, []string{"compat-closed", "compat-open"}) {
		t.Fatalf("normalized bead order = %#v", ids)
	}

	var legacyGot []wire.TaskResult
	if err := transport.Call(context.Background(), "tasks", wire.TasksParams{All: true}, &legacyGot); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacyGot, legacy) {
		t.Fatalf("legacy tasks Call() = %#v, want %#v", legacyGot, legacy)
	}

	first := <-requests
	second := <-requests
	if first.Command != "snapshot" || first.Params != nil || second.Command != "tasks" {
		t.Fatalf("requests = %#v, %#v", first, second)
	}
	var params wire.TasksParams
	if err := json.Unmarshal(second.Params, &params); err != nil || !params.All {
		t.Fatalf("legacy tasks params = %s, %v", second.Params, err)
	}
}

func snapshotParityResult() wire.SnapshotResult {
	captured := time.Date(2026, time.September, 7, 22, 10, 0, 0, time.UTC)
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
	return wire.SnapshotResult{
		ModelVersion: 1,
		Generation:   7,
		CapturedAt:   captured,
		Cursor:       11,
		Fresh:        true,
		Runtime:      wire.StatusResult{Version: "v1", Sessions: []wire.SessionResult{}},
		Repositories: []wire.RepoResult{{Name: "alpha"}},
		Seats:        []wire.SeatResult{},
		Sessions:     []wire.SessionResult{},
		Beads: []wire.BeadResult{
			{
				ID:                 "compat-closed",
				Repo:               "alpha",
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
				CreatedAt:          snapshotParityTime(captured.Add(-10 * time.Minute)),
				CreatedBy:          &createdBy,
				UpdatedAt:          snapshotParityTime(captured.Add(-9 * time.Minute)),
				StartedAt:          snapshotParityTime(captured.Add(-8 * time.Minute)),
				ClosedAt:           snapshotParityTime(captured.Add(-7 * time.Minute)),
				DeferredUntil:      snapshotParityTime(captured.Add(time.Hour)),
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
			},
			{ID: "compat-open", Repo: "alpha", Title: "open", Status: "open", Priority: 2, IssueType: "task", Dependencies: []wire.DependencyResult{}, Labels: []string{}, Dispatch: wire.Eligibility{Reason: snapshotParityString("workflow facts unavailable")}, Review: wire.Eligibility{Reason: snapshotParityString("workflow facts unavailable")}},
		},
		StatusCounts:     []wire.StatusCount{{Status: "closed", Count: 1}, {Status: "open", Count: 1}},
		RepositoryErrors: []wire.RepositoryError{},
	}
}

func snapshotParitySocket(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(fmt.Sprintf(".snapshot-parity-%d.sock", snapshotParitySocketSequence.Add(1)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func snapshotParityTime(value time.Time) *time.Time { return &value }

func snapshotParityString(value string) *string { return &value }
