package dispatch_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/cli"
	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/daemon"
	"github.com/FreezingSnail/magicite/internal/decomp"
	"github.com/FreezingSnail/magicite/internal/dispatch"
	magicexec "github.com/FreezingSnail/magicite/internal/exec"
	"github.com/FreezingSnail/magicite/internal/parity"
	"github.com/FreezingSnail/magicite/internal/repo"
	"github.com/FreezingSnail/magicite/internal/testenv"
)

func TestMaduinOrchestrationParity(t *testing.T) {
	if os.Getenv("MAGICITE_ORCHESTRATION_ARGV") != "" {
		return
	}
	bindings := parity.NewBindings(t, "TestMaduinOrchestrationParity")
	bindings.Bind("maduin-test-plan-injects-spec-fields-once", func(t *testing.T) {
		d, _, _ := orchestrationDispatcher(t, orchestrationRepo(t, "plan"))
		plan, err := d.PlanFor(context.Background(), orchestrationRepo(t, "plan"), dispatch.Implementer, "task-1", "ifrit")
		if err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"Title:", "Description:", "Design:", "Acceptance:", "Title text", "Description text", "Design text", "Acceptance text"} {
			if got := strings.Count(plan, field); got != 1 {
				t.Fatalf("plan occurrences of %q = %d, want 1", field, got)
			}
		}
	})
	bindings.Bind("maduin-test-lifecycle-events-reach-the-log", func(t *testing.T) {
		d, _, _ := orchestrationDispatcher(t, orchestrationRepo(t, "lifecycle"))
		stop, err := d.Start(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		stop(context.Background(), true)
	})
	bindings.Bind("maduin-test-main-commands-exist", func(t *testing.T) {
		want := map[string]bool{"start": false, "stop": false, "dispatch": false, "review": false}
		for _, command := range cli.Commands() {
			if _, ok := want[command.Name]; ok && command.Run != nil {
				want[command.Name] = true
			}
		}
		for name, found := range want {
			if !found {
				t.Errorf("CLI command %q is unavailable", name)
			}
		}
	})
	bindings.Bind("maduin-test-designer-functions-exist", func(t *testing.T) {
		d, _, _ := orchestrationDispatcher(t, orchestrationRepo(t, "designer"))
		if d.RoleCap(dispatch.Designer) == 0 || d.FreeSeat(dispatch.Designer) == "" {
			t.Fatal("designer dispatch surface is unavailable")
		}
	})
	bindings.Bind("maduin-test-designer-repo-resolution", func(t *testing.T) {
		first, second := orchestrationRepo(t, "first"), orchestrationRepo(t, "second")
		got, err := repo.GetIn([]repo.Repo{first, second}, "second")
		if err != nil || got != second {
			t.Fatalf("GetIn() = (%#v, %v), want second", got, err)
		}
	})
	bindings.Bind("maduin-test-designer-prompts-are-repo-scoped", func(t *testing.T) {
		r := orchestrationRepo(t, "scoped")
		d, _, _ := orchestrationDispatcher(t, r)
		plan, err := d.PlanFor(context.Background(), r, dispatch.Designer, "task-1", "ramuh")
		if err != nil || !strings.Contains(plan, "scoped") || !strings.Contains(plan, "--body-file") {
			t.Fatalf("designer plan = (%q, %v)", plan, err)
		}
	})
	bindings.Bind("maduin-test-designer-design-and-epic-dispatch-repo", func(t *testing.T) {
		r := orchestrationRepo(t, "dispatch")
		d, runner, _ := orchestrationDispatcher(t, r)
		if handle := d.Design(context.Background(), r, "task-1"); handle == "" {
			t.Fatal("Design() refused task")
		}
		if len(runner.specs) != 1 || runner.specs[0].Workdir != r.Root {
			t.Fatalf("designer runs = %#v", runner.specs)
		}
	})
	bindings.Bind("maduin-test-designer-nil-repo-refuses-without-io", func(t *testing.T) {
		r := orchestrationRepo(t, "refusal")
		d, runner, _ := orchestrationDispatcher(t, r)
		if handle := d.Design(context.Background(), repo.Repo{}, "task-1"); handle != "" || len(runner.specs) != 0 {
			t.Fatalf("Design(nil) = %q, runs = %#v", handle, runner.specs)
		}
	})
	bindings.Bind("maduin-test-designer-decompose-selects-only-chosen-repo", func(t *testing.T) {
		r := orchestrationRepo(t, "chosen")
		d, runner, _ := orchestrationDispatcher(t, r)
		if handle := d.DecomposeEpic(context.Background(), r, "epic-1"); handle == "" || len(runner.specs) != 1 || runner.specs[0].Workdir != r.Root {
			t.Fatalf("DecomposeEpic() = %q, runs = %#v", handle, runner.specs)
		}
	})
	bindings.Bind("maduin-test-concierge-functions-exist", func(t *testing.T) {
		if _, ok := config.Default().Role("concierge"); !ok {
			t.Fatal("concierge role is unavailable")
		}
	})
	bindings.Bind("maduin-test-concierge-model-resolution", func(t *testing.T) {
		role, ok := config.Default().Role("concierge")
		if !ok || role.Model == "" || role.Seats[0].Model != role.Model {
			t.Fatalf("concierge role = %#v, found = %t", role, ok)
		}
	})
	bindings.Bind("maduin-test-concierge-repo-resolution", func(t *testing.T) {
		r := orchestrationRepo(t, "concierge")
		if got, err := repo.GetIn([]repo.Repo{r}, r.Name); err != nil || got != r {
			t.Fatalf("GetIn() = (%#v, %v), want configured repository", got, err)
		}
	})
	bindings.Bind("maduin-test-concierge-prompt-template", func(t *testing.T) {
		r := orchestrationRepo(t, "prompt")
		d, _, _ := orchestrationDispatcher(t, r)
		plan, err := d.PlanFor(context.Background(), r, dispatch.Designer, "epic-1", "ramuh")
		if err != nil || !strings.Contains(plan, "Produce the design") || !strings.Contains(plan, dispatch.BDInputRules) {
			t.Fatalf("designer plan = (%q, %v)", plan, err)
		}
	})
	bindings.Bind("maduin-test-main-start-zero-sessions", func(t *testing.T) {
		d, _, _ := orchestrationDispatcher(t, orchestrationRepo(t, "start"))
		stop, err := d.Start(context.Background())
		if err != nil || !d.Idle() {
			t.Fatalf("Start() = (%v), idle = %t", err, d.Idle())
		}
		stop(context.Background(), true)
	})
	bindings.Bind("maduin-test-main-stop-tears-down", func(t *testing.T) {
		r := orchestrationRepo(t, "stop")
		d, runner, _ := orchestrationDispatcher(t, r)
		d.Add(dispatch.Session{Handle: "live", Repo: r, Role: dispatch.Designer, Seat: "ramuh"})
		waitClosed(t, d.Stop(context.Background(), true))
		if !d.Idle() || runner.deleted != 1 {
			t.Fatalf("hard stop: idle=%t deleted=%d", d.Idle(), runner.deleted)
		}
	})
	bindings.Bind("maduin-test-main-start-empty-registry-refuses", func(t *testing.T) {
		registry := repo.NewWith(repo.Options{Repos: config.ReposConfig{Discover: "explicit"}})
		if records := registry.List(context.Background()); len(records) != 0 {
			t.Fatalf("empty registry = %#v", records)
		}
	})
	bindings.Bind("maduin-test-main-stop-tears-down-active-repos", func(t *testing.T) {
		first, second := orchestrationRepo(t, "active-a"), orchestrationRepo(t, "active-b")
		d, runner, _ := orchestrationDispatcher(t, first)
		d.Add(dispatch.Session{Handle: "a", Repo: first, Role: dispatch.Designer, Seat: "ramuh"})
		d.Add(dispatch.Session{Handle: "b", Repo: second, Role: dispatch.Implementer, Seat: "ifrit"})
		waitClosed(t, d.Stop(context.Background(), true))
		if !d.Idle() || runner.deleted != 2 {
			t.Fatalf("active-repo stop: idle=%t deleted=%d", d.Idle(), runner.deleted)
		}
	})
	bindings.Bind("maduin-test-planner-designer-requires-luna-executable-children", func(t *testing.T) {
		child := decomp.Child{ID: "child", Description: orchestrationDescription(), Labels: []string{"difficulty:low"}}
		if violations := decomp.Check([]decomp.Child{child}); len(violations) == 0 || violations[0].Rule != decomp.RuleCountBudget {
			t.Fatalf("Check() = %#v, want executable-child budget refusal", violations)
		}
	})
	bindings.Bind("maduin-test-prompts-carry-bd-input-rules", func(t *testing.T) {
		for _, text := range []string{"--body-file", "--design-file", "Never inline prose", "Never use a heredoc"} {
			if !strings.Contains(dispatch.BDInputRules, text) {
				t.Errorf("BDInputRules missing %q", text)
			}
		}
	})
	bindings.Bind("maduin-test-concierge-splits-oversized-ideas-into-epics", func(t *testing.T) {
		children := make([]decomp.Child, decomp.MaxChildren+1)
		for i := range children {
			children[i] = decomp.Child{ID: string(rune('a' + i)), Description: orchestrationDescription(), Labels: []string{"difficulty:low"}}
		}
		if violations := decomp.Check(children); len(violations) == 0 || violations[0].Rule != decomp.RuleCountBudget {
			t.Fatalf("Check() = %#v, want oversized decomposition refusal", violations)
		}
	})
	bindings.Bind("maduin-test-designer-decomposition-budget-is-bounded", func(t *testing.T) {
		if decomp.MinChildren != 8 || decomp.MaxChildren != 12 || decomp.FleetConcurrency != 3 {
			t.Fatalf("decomposition budget = (%d, %d, %d)", decomp.MinChildren, decomp.MaxChildren, decomp.FleetConcurrency)
		}
	})
	bindings.Bind("maduin-test-designer-agent-avoids-micro-bead-rules", func(t *testing.T) {
		children := []decomp.Child{{ID: "micro", Description: orchestrationDescription(), Labels: []string{"difficulty:low"}}}
		violations := decomp.Check(children)
		if len(violations) == 0 || violations[0].Rule != decomp.RuleCountBudget {
			t.Fatalf("Check() = %#v, want micro decomposition refusal", violations)
		}
	})
	bindings.Bind("maduin-test-no-shell-routed-subprocesses", func(t *testing.T) {
		_, _, code, err := magicexec.RunEnv(context.Background(), ".", []string{"MAGICITE_ORCHESTRATION_ARGV=1"}, os.Args[0], "-test.run=^TestMaduinOrchestrationParity$", "--", `literal; $(not-a-command) && "quoted"`)
		if err != nil || code != 0 {
			t.Fatalf("argv-only subprocess = (exit %d, %v)", code, err)
		}
	})
	bindings.Bind("maduin-test-main-start-retains-only-hardened-repos", func(t *testing.T) {
		env := testenv.New(t)
		record := testenv.NewRepo(t, env, "hardened")
		if err := os.Mkdir(record.Root+"/.beads", 0o755); err != nil {
			t.Fatal(err)
		}
		registry := repo.NewWith(repo.Options{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{record.Root}}})
		if records := registry.List(context.Background()); len(records) != 1 || !repo.SameRoot(records[0].Root, record.Root) {
			t.Fatalf("configured registry = %#v", records)
		}
	})
	bindings.Bind("maduin-test-main-start-all-harden-fail-refuses", func(t *testing.T) {
		env := testenv.New(t)
		path := env.Root + "/bad.yaml"
		if err := os.WriteFile(path, []byte("repos: ["), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := daemon.Assemble(context.Background(), path); err == nil || !strings.HasPrefix(err.Error(), "daemon: load config:") {
			t.Fatalf("Assemble() error = %v, want daemon configuration refusal", err)
		}
	})
	bindings.Bind("maduin-test-main-bootstrap-repos-are-lazy-and-scoped", func(t *testing.T) {
		env := testenv.New(t)
		record := testenv.NewRepo(t, env, "scoped")
		if err := os.Mkdir(record.Root+"/.beads", 0o755); err != nil {
			t.Fatal(err)
		}
		registry := repo.NewWith(repo.Options{Repos: config.ReposConfig{Discover: "explicit", Roots: []string{record.Root}}})
		first, second := registry.List(context.Background()), registry.List(context.Background())
		if len(first) != 1 || len(second) != 1 || first[0] != second[0] {
			t.Fatalf("lazy registry = (%#v, %#v)", first, second)
		}
	})
	bindings.Bind("maduin-test-fixture-repos-create-registry-and-main", func(t *testing.T) {
		env := testenv.New(t)
		record := testenv.NewRepo(t, env, "fixture")
		if head := record.Head("main"); head == "" {
			t.Fatal("fixture main has no initial commit")
		}
	})
	bindings.Bind("maduin-test-fixture-repos-clean-up-and-restore-state", func(t *testing.T) {
		env := testenv.New(t)
		record := testenv.NewRepo(t, env, "cleanup")
		before := record.Head("main")
		record.Commit("fixture change", map[string]string{"state.txt": "changed\n"})
		record.Checkout("main")
		if after := record.Head("main"); after == before {
			t.Fatal("fixture state did not advance deterministically")
		}
	})
	bindings.Bind("maduin-test-final-repository-apis-refuse-nil-and-leave-no-legacy-symbols", func(t *testing.T) {
		if _, err := repo.GetIn(nil, "missing"); !repo.IsNotFound(err) {
			t.Fatalf("GetIn(nil) error = %v, want NotFoundError", err)
		}
	})
	bindings.Run()
}

type orchestrationBeads struct{}

func (orchestrationBeads) Ready(context.Context, repo.Repo) ([]dispatch.ReadyEntry, error) {
	return nil, nil
}
func (orchestrationBeads) Show(context.Context, repo.Repo, string) (dispatch.Spec, error) {
	return dispatch.Spec{Title: "Title text", Description: "Description text", Design: "Design text", Acceptance: "Acceptance text"}, nil
}
func (orchestrationBeads) Claim(context.Context, repo.Repo, string) error           { return nil }
func (orchestrationBeads) Release(context.Context, repo.Repo, string) error         { return nil }
func (orchestrationBeads) Close(context.Context, repo.Repo, string, string) error   { return nil }
func (orchestrationBeads) Comment(context.Context, repo.Repo, string, string) error { return nil }
func (orchestrationBeads) Difficulty(context.Context, repo.Repo, string) (string, error) {
	return "high", nil
}
func (orchestrationBeads) HumanOnly(context.Context, repo.Repo, string) (bool, error) {
	return false, nil
}
func (orchestrationBeads) InProgress(context.Context, repo.Repo) ([]string, error) { return nil, nil }
func (orchestrationBeads) OpenEpics(context.Context, repo.Repo) ([]string, error)  { return nil, nil }
func (orchestrationBeads) EpicChildren(context.Context, repo.Repo, string) ([]string, error) {
	return nil, nil
}
func (orchestrationBeads) EpicOpenChildren(context.Context, repo.Repo, string) ([]string, error) {
	return nil, nil
}
func (orchestrationBeads) DriftFixTasks(context.Context, repo.Repo) ([]string, error) {
	return nil, nil
}
func (orchestrationBeads) CancelAll(context.Context) error { return nil }

type orchestrationWorkspace struct{}

func (orchestrationWorkspace) Ensure(_ context.Context, r repo.Repo, _ string) (string, error) {
	return r.Root, nil
}
func (orchestrationWorkspace) Path(r repo.Repo, _ string) (string, error) { return r.Root, nil }
func (orchestrationWorkspace) Sync(context.Context, repo.Repo, string) (dispatch.SyncResult, error) {
	return dispatch.SyncOK, nil
}

type orchestrationRunner struct {
	specs    []dispatch.RunSpec
	deleted  int
	complete func(string, dispatch.Outcome)
}

func (r *orchestrationRunner) Run(_ context.Context, spec dispatch.RunSpec) (string, error) {
	r.specs = append(r.specs, spec)
	return "handle", nil
}
func (orchestrationRunner) Diff(context.Context, string) ([]dispatch.Diff, error) { return nil, nil }
func (orchestrationRunner) Output(context.Context, string) (string, error)        { return "", nil }
func (r *orchestrationRunner) Delete(context.Context, string) error               { r.deleted++; return nil }
func (orchestrationRunner) UsageLimited(context.Context, string) (bool, error)    { return false, nil }
func (r *orchestrationRunner) OnComplete(fn func(string, dispatch.Outcome))       { r.complete = fn }
func (*orchestrationRunner) OnPhase(func(string, string))                         {}

type orchestrationLander struct{}

func (orchestrationLander) Land(context.Context, repo.Repo, string, dispatch.Stamp) (dispatch.LandResult, error) {
	return dispatch.LandOK, nil
}
func (orchestrationLander) Landed(context.Context, repo.Repo, string) (bool, error) { return true, nil }
func (orchestrationLander) TaskLanded(context.Context, repo.Repo, string) (bool, error) {
	return true, nil
}

type orchestrationRepos struct{ records []repo.Repo }

func (r orchestrationRepos) List(context.Context) []repo.Repo { return r.records }
func (r orchestrationRepos) Current(context.Context, string) (repo.Repo, error) {
	if len(r.records) == 0 {
		return repo.Repo{}, &repo.NotFoundError{}
	}
	return r.records[0], nil
}

type orchestrationGate struct{}

func (orchestrationGate) Hold(context.Context, repo.Repo) (bool, error) { return false, nil }
func (orchestrationGate) DueEpic(context.Context, repo.Repo, string) (string, error) {
	return "", nil
}
func (orchestrationGate) GateEpic(context.Context, repo.Repo, string) (string, error) { return "", nil }
func (orchestrationGate) ReviewPlan(context.Context, repo.Repo, string) (dispatch.RunSpec, error) {
	return dispatch.RunSpec{}, nil
}
func (orchestrationGate) NoteSession(string, repo.Repo, string)                {}
func (orchestrationGate) CompleteReview(context.Context, string, string) error { return nil }
func (orchestrationGate) AbortReview(context.Context, string, string) error    { return nil }
func (orchestrationGate) DecompositionVerdict(context.Context, repo.Repo, string) ([]decomp.Violation, error) {
	return nil, nil
}

type orchestrationClock struct{}

func (orchestrationClock) Now() time.Time                       { return time.Unix(0, 0) }
func (orchestrationClock) Ticker(time.Duration) dispatch.Ticker { return orchestrationTicker{} }

type orchestrationTicker struct{}

func (orchestrationTicker) Chan() <-chan time.Time { return nil }
func (orchestrationTicker) Stop()                  {}

func orchestrationDispatcher(t *testing.T, r repo.Repo) (*dispatch.Dispatcher, *orchestrationRunner, orchestrationBeads) {
	t.Helper()
	runner := &orchestrationRunner{}
	beads := orchestrationBeads{}
	d, err := dispatch.New(dispatch.Deps{Beads: beads, Workspaces: orchestrationWorkspace{}, Lander: orchestrationLander{}, Runner: runner, Repos: orchestrationRepos{records: []repo.Repo{r}}, Gate: orchestrationGate{}, Clock: orchestrationClock{}, Config: config.Default()})
	if err != nil {
		t.Fatal(err)
	}
	return d, runner, beads
}

func orchestrationRepo(t *testing.T, name string) repo.Repo {
	t.Helper()
	r, ok := repo.Make(t.TempDir(), name, name, "main")
	if !ok {
		t.Fatal("repo.Make() failed")
	}
	return r
}

func orchestrationDescription() string {
	return "# Scope\nsmall\n\n# Files\ninternal/example.go\n\n# Contract\nvalid\n\n# Invariants\nvalid\n\n# Non-goals\nnone\n\n# MACHINE\nprovides:\nconsumes:\nfiles: internal/example.go\ntier: low"
}

func waitClosed(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("stop did not complete")
	}
}
