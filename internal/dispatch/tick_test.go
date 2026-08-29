package dispatch

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

func TestRepoWarnRateLimitsAndRepoOKClearsLatch(t *testing.T) {
	dispatcher, logs := spawnDispatcher(t, &fakeBeads{}, readyWorkspaces(), &fakeRunner{}, &fakeGate{})
	repository := spawnRepo(t)

	dispatcher.RepoWarn(repository, "first")
	dispatcher.RepoWarn(repository, "second")
	if got := len(*logs); got != 1 {
		t.Fatalf("warnings = %d, want one", got)
	}
	dispatcher.clock.(*manualClock).Advance(repoWarnInterval)
	dispatcher.RepoWarn(repository, "third")
	dispatcher.RepoOK(repository)
	dispatcher.RepoWarn(repository, "fourth")
	if got := len(*logs); got != 3 {
		t.Fatalf("warnings = %d, want three", got)
	}
}

func TestTickRecoversReworkBeforeReadyDispatch(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{
		driftFixTasks: func(context.Context, repo.Repo) ([]string, error) { return []string{"rework"}, nil },
		inProgress:    func(context.Context, repo.Repo) ([]string, error) { return []string{"rework"}, nil },
		ready: func(context.Context, repo.Repo) ([]ReadyEntry, error) {
			return []ReadyEntry{{Task: "ordinary", Priority: "1"}}, nil
		},
		difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil },
	}
	deps := completeDeps()
	deps.Beads = beads
	deps.Workspaces = readyWorkspaces()
	deps.Runner = &fakeRunner{}
	deps.Repos = &fakeRepos{list: func(context.Context) []repo.Repo { return []repo.Repo{repository} }}
	deps.Gate = &fakeGate{}
	deps.Config = config.Default()
	logs := []logCall{}
	deps.Logger = func(level logging.Level, kind string, fields map[string]any) {
		logs = append(logs, logCall{level: level, kind: kind, fields: fields})
	}
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}

	dispatcher.Tick(context.Background())
	claims := []string{}
	for _, call := range beads.Calls() {
		if call.Method == "Claim" {
			claims = append(claims, call.Args[1].(string))
		}
	}
	if want := []string{"rework", "ordinary"}; !reflect.DeepEqual(claims, want) {
		t.Fatalf("claim order = %#v, want %#v", claims, want)
	}
	if logs[len(logs)-1].kind != "tick" || logs[len(logs)-1].fields["rework"] != 1 {
		t.Fatalf("last log = %#v, want tick with rework", logs[len(logs)-1])
	}
}

func TestTickHoldSkipsOrdinaryWork(t *testing.T) {
	repository := spawnRepo(t)
	beads := &fakeBeads{ready: func(context.Context, repo.Repo) ([]ReadyEntry, error) {
		return []ReadyEntry{{Task: "ordinary", Priority: "1"}}, nil
	}}
	deps := completeDeps()
	deps.Beads = beads
	deps.Repos = &fakeRepos{list: func(context.Context) []repo.Repo { return []repo.Repo{repository} }}
	deps.Gate = &fakeGate{hold: func(context.Context, repo.Repo) (bool, error) { return true, nil }}
	logs := []logCall{}
	deps.Logger = func(level logging.Level, kind string, fields map[string]any) {
		logs = append(logs, logCall{level: level, kind: kind, fields: fields})
	}
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}

	dispatcher.Tick(context.Background())
	for _, call := range beads.Calls() {
		if call.Method == "Claim" {
			t.Fatalf("held tick claimed ordinary work: %#v", call)
		}
	}
	if logs[len(logs)-2].kind != "fleet-hold" || logs[len(logs)-2].fields["held"] != 1 {
		t.Fatalf("logs = %#v, want fleet hold", logs)
	}
}

func TestTickDoesNotOverlap(t *testing.T) {
	repository := spawnRepo(t)
	started := make(chan struct{})
	release := make(chan struct{})
	beads := &fakeBeads{driftFixTasks: func(context.Context, repo.Repo) ([]string, error) {
		close(started)
		<-release
		return nil, nil
	}}
	deps := completeDeps()
	deps.Beads = beads
	deps.Repos = &fakeRepos{list: func(context.Context) []repo.Repo { return []repo.Repo{repository} }}
	logs := []logCall{}
	deps.Logger = func(level logging.Level, kind string, fields map[string]any) {
		logs = append(logs, logCall{level: level, kind: kind, fields: fields})
	}
	dispatcher, err := New(deps)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan struct{})
	go func() { dispatcher.Tick(context.Background()); close(done) }()
	<-started
	if !dispatcher.TickInFlight() {
		t.Fatal("TickInFlight() = false during tick")
	}
	dispatcher.Tick(context.Background())
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("first tick did not complete")
	}
	if dispatcher.TickInFlight() {
		t.Fatal("TickInFlight() = true after tick")
	}
	if logs[0].kind != "tick-skipped" || logs[0].fields["reason"] != "in-flight" {
		t.Fatalf("logs = %#v, want in-flight skip", logs)
	}
}
