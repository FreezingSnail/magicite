package dispatch

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func TestOrphansExcludesLiveTasksAndPreservesOrder(t *testing.T) {
	dispatcher, _ := newRegistryDispatcher(t)
	dispatcher.Add(Session{Handle: "live", Task: "task-live"})

	got := dispatcher.Orphans([]string{"task-3", "task-live", "task-1", "task-2"})
	want := []string{"task-3", "task-1", "task-2"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Orphans() = %#v, want %#v", got, want)
	}
}

func TestRecoverTasksHonorsAllowListAndLogsSuccessfulRecovery(t *testing.T) {
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil }}
	dispatcher, logs := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, &fakeGate{})
	dispatcher.Add(Session{Handle: "live", Task: "task-live"})
	repository := spawnRepo(t)

	if got := dispatcher.RecoverTasks(context.Background(), repository,
		[]string{"task-1", "task-2", "task-live"}, []string{"task-2", "task-live"}); got != 1 {
		t.Fatalf("RecoverTasks() = %d, want 1", got)
	}
	if calls := beads.Calls(); len(calls) == 0 || calls[0].Method != "HumanOnly" || calls[0].Args[1] != "task-2" {
		t.Fatalf("bead calls = %#v, want task-2 dispatch", calls)
	}
	if len(*logs) != 2 || (*logs)[1].kind != logging.KindRecovery {
		t.Fatalf("logs = %#v, want pickup and recovery", *logs)
	}
	fields := (*logs)[1].fields
	if fields["task"] != "task-2" || fields["repo"] != repository.LogName() || fields["reason"] != orphanedInProgressReason {
		t.Fatalf("recovery fields = %#v", fields)
	}
}

func TestRecoverTasksDoesNotRetryFailedDispatch(t *testing.T) {
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil }}
	runner := &fakeRunner{run: func(context.Context, RunSpec) (string, error) { return "", errors.New("failed") }}
	dispatcher, _ := spawnDispatcher(t, beads, runnerWorkspaces(), runner, &fakeGate{})

	if got := dispatcher.RecoverTasks(context.Background(), spawnRepo(t), []string{"task-1", "task-1"}, nil); got != 0 {
		t.Fatalf("RecoverTasks() = %d, want 0", got)
	}
	if got := len(runner.Calls()); got != 1 {
		t.Fatalf("runner calls = %d, want one attempted dispatch", got)
	}
}

func TestRecoverRepoWrapsInProgressFailure(t *testing.T) {
	cause := errors.New("bd unavailable")
	beads := &fakeBeads{inProgress: func(context.Context, repo.Repo) ([]string, error) { return nil, cause }}
	dispatcher, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, &fakeGate{})
	repository := spawnRepo(t)

	count, err := dispatcher.RecoverRepo(context.Background(), repository)
	var recoverErr *RecoverError
	if count != 0 || !errors.As(err, &recoverErr) || !errors.Is(err, cause) || recoverErr.Repo != repository {
		t.Fatalf("RecoverRepo() = (%d, %v), want typed wrapped error", count, err)
	}
}

func runnerWorkspaces() *fakeWorkspaces {
	return readyWorkspaces()
}
