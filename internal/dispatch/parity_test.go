package dispatch

import (
	"context"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/parity"
	"github.com/connorfranc/magicite/internal/repo"
)

func TestMaduinDispatchParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinDispatchParity")
	bindings.Bind("maduin-test-dispatch-queue-ready-entry", func(t *testing.T) { TestNormalizeReady(t) })
	bindings.Bind("maduin-test-dispatch-queue-priority", func(t *testing.T) { TestMergeReadyPriorityAndRoundRobin(t) })
	bindings.Bind("maduin-test-dispatch-queue-round-robin", func(t *testing.T) { TestMergeReadyPriorityAndRoundRobin(t) })
	bindings.Bind("maduin-test-dispatch-queue-no-starvation", func(t *testing.T) { assertQueueReplay(t) })
	bindings.Bind("maduin-test-dispatch-queue-source-order", func(t *testing.T) { TestMergeReadyPriorityAndRoundRobin(t) })
	bindings.Bind("maduin-test-dispatch-queue-deterministic", func(t *testing.T) { assertQueueReplay(t) })
	bindings.Bind("maduin-test-dispatch-queue-mixed-priorities", func(t *testing.T) { TestMergeReadyPriorityAndRoundRobin(t) })
	bindings.Bind("maduin-test-dispatch-queue-malformed-and-empty", func(t *testing.T) { TestNormalizeReady(t) })
	bindings.Bind("maduin-test-dispatch-queue-take-ready", func(t *testing.T) { TestTakeReadyBoundAndCopy(t) })
	bindings.Bind("maduin-test-dispatch-tick-io-free", func(t *testing.T) { TestTickHoldSkipsOrdinaryWork(t) })

	bindings.Bind("maduin-test-dispatch-spawn-runs-cockpit-refresh-hook", func(t *testing.T) { TestImplementPickupCapturesResolvedRouting(t) })
	bindings.Bind("maduin-test-dispatch-on-complete-runs-cockpit-refresh-hook", func(t *testing.T) { TestOnCompleteClosesOnlyVerifiedLandedTask(t) })
	bindings.Bind("maduin-test-dispatch-live-spawn-entry-state", func(t *testing.T) { TestImplementPickupCapturesResolvedRouting(t) })
	bindings.Bind("maduin-test-dispatch-live-event-updates-phase-and-status", func(t *testing.T) { assertSessionMutation(t) })
	bindings.Bind("maduin-test-dispatch-live-notify-unbound-noop", func(t *testing.T) { TestOnCompleteIgnoresUnknownHandle(t) })
	bindings.Bind("maduin-test-dispatch-live-event-hook-registers-once", func(t *testing.T) { assertSessionMutation(t) })
	bindings.Bind("maduin-test-dispatch-live-old-entries-keep-concurrency-accounting", func(t *testing.T) { assertRoleCapacity(t) })

	bindings.Bind("maduin-test-dispatch-functions-exist", func(t *testing.T) { assertDispatchSurface(t) })
	bindings.Bind("maduin-test-dispatch-implement-concurrency-cap", func(t *testing.T) { TestImplementStopsAtRoleCap(t) })
	bindings.Bind("maduin-test-dispatch-completion-failure-keeps-open", func(t *testing.T) { TestOnCompleteFailureAndPanicReleaseThenDelete(t) })
	bindings.Bind("maduin-test-dispatch-usage-limit-falls-back", func(t *testing.T) { TestFallbackRetryCarriesRoutingAndMarksAttempt(t) })
	bindings.Bind("maduin-test-dispatch-gate-failure-keeps-open", func(t *testing.T) { TestOnCompleteReleasesUnprovenLandingAndDeletes(t) })
	bindings.Bind("maduin-test-dispatch-completed-decomposition-releases-epic", func(t *testing.T) { TestDecomposeEpicMarksSessionAndLogs(t) })
	bindings.Bind("maduin-test-dispatch-completion-land-fail-releases", func(t *testing.T) { TestOnCompleteReleasesUnprovenLandingAndDeletes(t) })
	bindings.Bind("maduin-test-dispatch-orphaned-tasks", func(t *testing.T) { TestOrphansExcludesLiveTasksAndPreservesOrder(t) })
	bindings.Bind("maduin-test-dispatch-recover-releases-orphaned-epic", func(t *testing.T) { TestRecoverTasksHonorsAllowListAndLogsSuccessfulRecovery(t) })
	bindings.Bind("maduin-test-dispatch-recover-redispatches-orphans", func(t *testing.T) { TestRecoverTasksHonorsAllowListAndLogsSuccessfulRecovery(t) })
	bindings.Bind("maduin-test-dispatch-start-stop-timer", func(t *testing.T) { assertLifecycleStartStop(t) })
	bindings.Bind("maduin-test-dispatch-soft-stop-drains", func(t *testing.T) { assertSoftStopDrains(t) })
	bindings.Bind("maduin-test-dispatch-hard-stop-deletes", func(t *testing.T) { assertHardStopDeletes(t) })
	bindings.Bind("maduin-test-dispatch-undecomposed-epics", func(t *testing.T) { TestEpicPassPartitionsDecomposeAndGate(t) })
	bindings.Bind("maduin-test-dispatch-undecomposed-epics-none-open", func(t *testing.T) { TestTickRunsEpicPassAfterReadyResults(t) })

	bindings.Bind("maduin-test-dispatch-complete-reviewer-parses-verdict", func(t *testing.T) { TestOnCompleteReviewerReadsOutputWithoutLanding(t) })
	bindings.Bind("maduin-test-dispatch-ready-holds-fleet-for-rework", func(t *testing.T) { TestTickHoldSkipsOrdinaryWork(t) })
	bindings.Bind("maduin-test-dispatch-seat-backend-routing-records-backend", func(t *testing.T) { TestImplementPickupCapturesResolvedRouting(t) })
	bindings.Bind("maduin-test-dispatch-sticky-backend-for-diff-and-delete", func(t *testing.T) { assertStickyBackend(t) })
	bindings.Bind("maduin-test-dispatch-spawn-failure-releases-claim", func(t *testing.T) { TestImplementRunFailureReleasesClaimAndLeavesTaskOpen(t) })
	bindings.Bind("maduin-test-dispatch-format-kiro-diff-string", func(t *testing.T) { assertDiffFormatting(t) })
	bindings.Bind("maduin-test-dispatch-decompose-epic-logs-at-cap", func(t *testing.T) { TestDecomposeEpicLogsAtDesignerCap(t) })
	bindings.Bind("maduin-test-dispatch-kiro-fallbacks-cover-every-role", func(t *testing.T) { assertRoleCapacity(t) })
	bindings.Bind("maduin-test-dispatch-kiro-limited-retry-sticky-and-bounded", func(t *testing.T) { TestFallbackRetrySecondFailureFallsThrough(t) })
	bindings.Bind("maduin-test-dispatch-repair-plan-is-repo-scoped", func(t *testing.T) { TestRepairSkipsSyncAndMarksSessionRepairing(t) })
	bindings.Bind("maduin-test-dispatch-repair-uses-repo-seat-workdir-without-sync", func(t *testing.T) { TestRepairSkipsSyncAndMarksSessionRepairing(t) })
	bindings.Bind("maduin-test-dispatch-conflict-repairs-failed-entry-repo", func(t *testing.T) { TestOnCompleteConflictDispatchesOneRepair(t) })
	bindings.Bind("maduin-test-dispatch-missing-repo-refuses-on-completion", func(t *testing.T) { TestOnCompleteIgnoresUnknownHandle(t) })

	bindings.Bind("maduin-test-dispatch-notify-skips-unchanged-status", func(t *testing.T) { assertSessionMutation(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-sweep-failed-query-finishes", func(t *testing.T) { TestRecoverRepoWrapsInProgressFailure(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-sweep-spawn-failure-finishes", func(t *testing.T) { TestRecoverTasksDoesNotRetryFailedDispatch(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-no-sync-query", func(t *testing.T) { TestTickHoldSkipsOrdinaryWork(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-drain-midchain", func(t *testing.T) { assertSoftStopDrains(t) })
	bindings.Bind("maduin-test-dispatch-stop-cancels-async-tick", func(t *testing.T) { assertLifecycleStartStop(t) })
	bindings.Bind("maduin-test-dispatch-start-clears-stale-freeze", func(t *testing.T) { assertLifecycleStartStop(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-logs-skipped-tick", func(t *testing.T) { TestTickDoesNotOverlap(t) })
	bindings.Bind("maduin-test-dispatch-spawn-low-uses-luna", func(t *testing.T) { assertTieredSpawn(t, "low", "gpt-5.6-luna", "medium") })
	bindings.Bind("maduin-test-dispatch-spawn-high-uses-terra", func(t *testing.T) { assertTieredSpawn(t, "high", "gpt-5.6-terra", "high") })
	bindings.Bind("maduin-test-dispatch-spawn-opencode-ignores-difficulty", func(t *testing.T) { assertOpenCodeSpawn(t) })
	bindings.Bind("maduin-test-dispatch-spawn-difficulty-error-defaults", func(t *testing.T) { TestImplementPickupCapturesResolvedRouting(t) })
	bindings.Bind("maduin-test-dispatch-fallback-retains-difficulty-and-effort", func(t *testing.T) { TestFallbackRetryCarriesRoutingAndMarksAttempt(t) })
	bindings.Bind("maduin-test-dispatch-set-status-preserves-other-entries", func(t *testing.T) { assertSessionMutation(t) })
	bindings.Bind("maduin-test-dispatch-syncs-seat-before-claim", func(t *testing.T) { assertSeatSyncBeforeClaim(t) })
	bindings.Bind("maduin-test-dispatch-repairer-skips-sync", func(t *testing.T) { TestRepairSkipsSyncAndMarksSessionRepairing(t) })
	bindings.Bind("maduin-test-dispatch-sync-seam-default", func(t *testing.T) { TestSeatReadySyncConflictRefusesAndComments(t) })
	bindings.Bind("maduin-test-dispatch-logs-pickup-and-finish", func(t *testing.T) { TestImplementPickupCapturesResolvedRouting(t) })

	bindings.Bind("maduin-test-dispatch-completion-requires-task-provenance", func(t *testing.T) { TestOnCompleteReleasesUnprovenLandingAndDeletes(t) })
	bindings.Bind("maduin-test-dispatch-complete-passes-entry-repo-to-land-seams", func(t *testing.T) { TestOnCompleteClosesOnlyVerifiedLandedTask(t) })
	bindings.Bind("maduin-test-dispatch-fan-out-starts-before-finishes-in-order", func(t *testing.T) { TestFanOutKeepsSuccessfulResultsInInputOrder(t) })
	bindings.Bind("maduin-test-dispatch-fan-out-start-failure-isolated", func(t *testing.T) { TestFanOutKeepsSuccessfulResultsInInputOrder(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-fans-repos-and-isolates-holds", func(t *testing.T) { TestTickRecoversReworkBeforeReadyDispatch(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-skips-failed-repo-once", func(t *testing.T) { TestRepoWarnRateLimitsAndRepoOKClearsLatch(t) })
	bindings.Bind("maduin-test-dispatch-fan-out-zero-and-callback-error-finish-once", func(t *testing.T) { TestFanOutCancelledContextReturnsWithoutWaiting(t) })
	bindings.Bind("maduin-test-dispatch-repo-warning-ratelimit-and-success-reset", func(t *testing.T) { TestRepoWarnRateLimitsAndRepoOKClearsLatch(t) })
	bindings.Bind("maduin-test-dispatch-run-loop-empty-registry-finishes-once", func(t *testing.T) { assertEmptyRegistryTick(t) })
	bindings.Run()
}

func assertQueueReplay(t *testing.T) {
	t.Helper()
	alpha := readyRepo(t, "/alpha", "alpha")
	beta := readyRepo(t, "/beta", "beta")
	got := MergeReady([]RepoReady{{Repo: alpha, Entries: []ReadyEntry{{Task: "a1", Priority: "1"}, {Task: "a2", Priority: "1"}, {Task: "a3", Priority: "2"}}}, {Repo: beta, Entries: []ReadyEntry{{Task: "b1", Priority: "1"}, {Task: "b2", Priority: "2"}}}})
	want := []ReadyEntry{{Repo: alpha, Task: "a1", Priority: "1"}, {Repo: beta, Task: "b1", Priority: "1"}, {Repo: alpha, Task: "a2", Priority: "1"}, {Repo: alpha, Task: "a3", Priority: "2"}, {Repo: beta, Task: "b2", Priority: "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeReady() = %#v, want %#v", got, want)
	}
	if again := MergeReady([]RepoReady{{Repo: alpha, Entries: []ReadyEntry{{Task: "a1", Priority: "1"}, {Task: "a2", Priority: "1"}, {Task: "a3", Priority: "2"}}}, {Repo: beta, Entries: []ReadyEntry{{Task: "b1", Priority: "1"}, {Task: "b2", Priority: "2"}}}}); !reflect.DeepEqual(again, got) {
		t.Fatal("queue output is not deterministic")
	}
}

func assertDispatchSurface(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	if d.RoleCap(Implementer) == 0 || d.FreeSeat(Implementer) == "" {
		t.Fatal("implementer dispatch surface is unavailable")
	}
}

func assertRoleCapacity(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	if d.RoleCap(Implementer) != 2 || d.RoleCap(Designer) != 2 {
		t.Fatal("configured role caps not retained")
	}
	d.Add(Session{Handle: "one", Role: Implementer, Seat: "ifrit"})
	if d.FreeSeat(Implementer) != "shiva" {
		t.Fatal("first occupied seat did not preserve next seat")
	}
}

func assertSessionMutation(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	d.Add(Session{Handle: "one", Task: "one", Role: Implementer, Seat: "ifrit"})
	d.Add(Session{Handle: "two", Task: "two", Role: Implementer, Seat: "shiva"})
	if !d.SetStatus("one", Running) || !d.SetPhase("one", "tool") {
		t.Fatal("session mutation refused live handle")
	}
	sessions := d.Sessions()
	if len(sessions) != 2 || sessions[0].Status != Running || sessions[0].Phase != "tool" || sessions[1].Handle != "two" {
		t.Fatalf("sessions = %#v", sessions)
	}
}

func assertCompletedDecompositionReleasesEpic(t *testing.T) {
	t.Helper()
	beads := &fakeBeads{}
	d := outcomeDispatcher(t, beads, &fakeLander{}, &fakeRunner{}, &fakeGate{})
	session := outcomeSession("decompose", Designer)
	session.Decomposition = true
	session.Task = "epic-1"
	d.Add(session)
	d.OnComplete(context.Background(), session.Handle, Completed)
	if got := callMethods(beads.Calls()); !reflect.DeepEqual(got, []string{"Release"}) {
		t.Fatalf("decomposition calls = %v, want Release", got)
	}
}

func assertRecoverReleasesEpic(t *testing.T) {
	t.Helper()
	beads := &fakeBeads{}
	d, _ := spawnDispatcher(t, beads, readyWorkspaces(), &fakeRunner{}, &fakeGate{})
	if got := d.RecoverTasks(context.Background(), spawnRepo(t), []string{"epic-1"}, nil); got != 1 {
		t.Fatalf("RecoverTasks() = %d, want 1", got)
	}
	if got := callMethods(beads.Calls()); !reflect.DeepEqual(got, []string{"HumanOnly", "Claim", "Difficulty", "Show"}) {
		t.Fatalf("recovery calls = %v, want re-dispatch admission", got)
	}
}

func assertLifecycleStartStop(t *testing.T) {
	t.Helper()
	d, _, _ := lifecycleDispatcher(t, &fakeBeads{}, &lifecycleRunner{}, &fakeGate{})
	stop, err := d.Start(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	<-stop(context.Background(), true)
	if d.TickInFlight() {
		t.Fatal("dispatcher retained in-flight tick after stop")
	}
}

func assertSoftStopDrains(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	d.Add(Session{Handle: "live", Task: "task", Role: Implementer, Seat: "ifrit"})
	drain := d.Stop(context.Background(), false)
	if !d.Draining() || d.Idle() {
		t.Fatal("soft stop did not retain active session while draining")
	}
	d.Remove("live")
	d.completeDrain()
	select {
	case <-drain:
	default:
		t.Fatal("drain did not finish after final session removal")
	}
}

func assertHardStopDeletes(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	d.Add(Session{Handle: "live", Task: "task", Role: Implementer, Seat: "ifrit"})
	<-d.Stop(context.Background(), true)
	if !d.Idle() {
		t.Fatal("hard stop retained live sessions")
	}
}

func assertStickyBackend(t *testing.T) {
	t.Helper()
	runner := &fakeRunner{}
	session := outcomeSession("sticky", Implementer)
	session.Backend = "kiro"
	d := outcomeDispatcher(t, &fakeBeads{}, &fakeLander{}, runner, &fakeGate{})
	d.Add(session)
	d.OnComplete(context.Background(), session.Handle, Completed)
	for _, call := range runner.Calls() {
		if call.Method == "Diff" || call.Method == "Delete" {
			continue
		}
		t.Fatalf("unexpected runner call %#v", call)
	}
}

func assertDiffFormatting(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	value := d.FormatDiffs([]Diff{{File: "a.go", Patch: "diff --git a/a.go b/a.go\n", Status: "modified"}})
	if !strings.Contains(value, "diff --git a/a.go b/a.go") {
		t.Fatalf("FormatDiffs() = %q", value)
	}
}

func assertRepairPlan(t *testing.T) {
	t.Helper()
	r := readyRepo(t, "/repos/alpha", "alpha")
	d, _ := newRegistryDispatcher(t)
	plan, err := d.PlanFor(context.Background(), r, Repairer, "task-1", "ifrit")
	if err != nil || !strings.Contains(plan, "git rebase main") || strings.Contains(plan, "git merge main") {
		t.Fatalf("repair plan = (%q, %v)", plan, err)
	}
}

func assertTieredSpawn(t *testing.T, difficulty, model, effort string) {
	t.Helper()
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return difficulty, nil }}
	runner := &fakeRunner{}
	d, _ := spawnDispatcher(t, beads, readyWorkspaces(), runner, &fakeGate{})
	d.config.Crew.Backend = config.BackendKiro
	if handle := d.Implement(context.Background(), spawnRepo(t), "task"); handle == "" {
		t.Fatal("tiered Implement() refused task")
	}
	run := runner.Calls()[0].Args[0].(RunSpec)
	if run.Model != model || run.Effort != effort {
		t.Fatalf("tiered run = %#v, want model=%q effort=%q", run, model, effort)
	}
}

func assertOpenCodeSpawn(t *testing.T) {
	t.Helper()
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "low", nil }}
	runner := &fakeRunner{}
	d, _ := spawnDispatcher(t, beads, readyWorkspaces(), runner, &fakeGate{})
	if handle := d.Implement(context.Background(), spawnRepo(t), "task"); handle == "" {
		t.Fatal("Implement() refused task")
	}
	run := runner.Calls()[0].Args[0].(RunSpec)
	if run.Backend != "opencode" || run.Effort != "" {
		t.Fatalf("OpenCode run = %#v", run)
	}
}

func assertDifficultyFallback(t *testing.T) {
	t.Helper()
	beads := &fakeBeads{difficulty: func(context.Context, repo.Repo, string) (string, error) { return "", context.DeadlineExceeded }}
	runner := &fakeRunner{}
	d, _ := spawnDispatcher(t, beads, readyWorkspaces(), runner, &fakeGate{})
	if handle := d.Implement(context.Background(), spawnRepo(t), "task"); handle == "" {
		t.Fatal("difficulty error refused task")
	}
	if run := runner.Calls()[0].Args[0].(RunSpec); run.Model == "" {
		t.Fatal("difficulty error did not choose default routing")
	}
}

func assertSeatSyncBeforeClaim(t *testing.T) {
	t.Helper()
	order := []string{}
	beads := &fakeBeads{claim: func(context.Context, repo.Repo, string) error { order = append(order, "claim"); return nil }}
	workspaces := readyWorkspaces()
	workspaces.sync = func(context.Context, repo.Repo, string) (SyncResult, error) {
		order = append(order, "sync")
		return SyncOK, nil
	}
	dispatcher, _ := spawnDispatcher(t, beads, workspaces, &fakeRunner{}, &fakeGate{})
	_ = dispatcher.Implement(context.Background(), spawnRepo(t), "task-1")
	if !reflect.DeepEqual(order, []string{"sync", "claim"}) {
		t.Fatalf("seat/claim order = %q, want [sync claim]", order)
	}
}

func assertEmptyRegistryTick(t *testing.T) {
	t.Helper()
	d, _ := newRegistryDispatcher(t)
	d.Tick(context.Background())
	if d.TickInFlight() {
		t.Fatal("empty repository tick remained in flight")
	}
}

func orchestrationNames(owner string) []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, owner+"/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
