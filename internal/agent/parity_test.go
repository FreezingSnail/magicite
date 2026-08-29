package agent_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/agent"
	"github.com/FreezingSnail/magicite/internal/agent/backends"
	"github.com/FreezingSnail/magicite/internal/agent/kiro"
	"github.com/FreezingSnail/magicite/internal/agent/opencode"
	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/parity"
	"github.com/FreezingSnail/magicite/internal/testenv"
)

func TestMaduinAgentParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinAgentParity")

	bindings.Bind("maduin-test-session-parse-step-finish-stop", func(t *testing.T) {
		assertParseLine(t, `{"type":"step_finish","sessionID":"ses","part":{"reason":"stop"}}`, agent.StatusCompleted)
	})
	bindings.Bind("maduin-test-session-parse-step-finish-error", func(t *testing.T) {
		assertParseLine(t, `{"type":"step_finish","part":{"reason":"error"}}`, agent.StatusFailed)
	})
	bindings.Bind("maduin-test-session-parse-tool-use-error", func(t *testing.T) {
		assertParseLine(t, `{"type":"tool_use","part":{"state":{"status":"error"}}}`, agent.StatusFailed)
	})
	bindings.Bind("maduin-test-session-parse-tool-use-completed", func(t *testing.T) {
		assertParseLine(t, `{"type":"tool_use","part":{"state":{"status":"completed"}}}`, agent.StatusRunning)
	})
	bindings.Bind("maduin-test-session-parse-nonterminal", func(t *testing.T) {
		assertParseLine(t, `{"type":"step_start","properties":{"sessionID":"ses"}}`, agent.StatusRunning)
	})
	bindings.Bind("maduin-test-session-parse-garbage", func(t *testing.T) {
		if _, ok := opencode.ParseLine("{"); ok {
			t.Fatal("garbage parsed")
		}
	})
	bindings.Bind("maduin-test-session-usage-limit-line", func(t *testing.T) {
		if !agent.LimitedLine(`{"type":"session.error","error":{"message":"usage limit reached"}}`) {
			t.Fatal("limit line ignored")
		}
	})
	bindings.Bind("maduin-test-session-filter-retains-transcript", func(t *testing.T) { assertTranscript(t) })
	bindings.Bind("maduin-test-session-run-missing-cli", func(t *testing.T) { assertMissingCLI(t) })
	bindings.Bind("maduin-test-session-complete-p-unknown", func(t *testing.T) { assertUnknownRuntime(t, "complete") })
	bindings.Bind("maduin-test-session-diff-unknown", func(t *testing.T) { assertUnknownRuntime(t, "diff") })
	bindings.Bind("maduin-test-session-run-completes", func(t *testing.T) { assertRuntimeRoutes(t) })
	bindings.Bind("maduin-test-session-run-permission-denied", func(t *testing.T) { assertRuntimeFailure(t) })
	bindings.Bind("maduin-test-session-opencode-adapter-registered", func(t *testing.T) { assertBackendsRegistered(t) })
	bindings.Bind("maduin-test-session-opencode-command-and-ndjson-contract", func(t *testing.T) { assertOpenCodeArgs(t, "medium") })
	bindings.Bind("maduin-test-session-run-command-nil-effort-unchanged", func(t *testing.T) { assertOpenCodeArgs(t, "") })
	bindings.Bind("maduin-test-session-run-command-with-effort", func(t *testing.T) { assertOpenCodeArgs(t, "high") })
	bindings.Bind("maduin-test-session-run-command-effort-position", func(t *testing.T) { assertOpenCodeEffortPosition(t) })
	bindings.Bind("maduin-test-session-run-command-invalid-effort-omitted", func(t *testing.T) {
		if opencode.ValidEffort("bad effort") {
			t.Fatal("invalid effort accepted")
		}
	})
	bindings.Bind("maduin-test-session-opencode-effort-optional-arity", func(t *testing.T) { assertOpenCodeOptionalEffort(t) })
	bindings.Bind("maduin-test-session-completion-hook-runs-once", func(t *testing.T) { assertCompletionOnce(t) })
	bindings.Bind("maduin-test-session-diff-delete-cleans-registry", func(t *testing.T) { assertRuntimeDelete(t) })
	bindings.Bind("maduin-test-session-opencode-adapter-accepts-optional-effort", func(t *testing.T) {
		if !opencode.ValidEffort("experimental-v2") {
			t.Fatal("optional effort rejected")
		}
	})

	bindings.Bind("maduin-test-kiro-agent-files-cover-configured-roles", func(t *testing.T) { assertConfiguredKiroAgents(t) })
	bindings.Bind("maduin-test-kiro-agent-json-and-prompt-integrity", func(t *testing.T) { assertKiroAgentAndPlan(t) })
	bindings.Bind("maduin-test-kiro-run-command-nil-effort-unchanged", func(t *testing.T) { assertKiroArgs(t, "") })
	bindings.Bind("maduin-test-kiro-run-command-with-effort", func(t *testing.T) { assertKiroArgs(t, "high") })
	bindings.Bind("maduin-test-kiro-effort-valid-p-allowlist", func(t *testing.T) { assertKiroEffortAllowlist(t) })
	bindings.Bind("maduin-test-kiro-run-command-invalid-effort-omitted", func(t *testing.T) { assertKiroArgs(t, "not valid") })
	bindings.Bind("maduin-test-kiro-run-invalid-model-still-refuses", func(t *testing.T) {
		if kiro.ValidModel("provider/model") {
			t.Fatal("slash model accepted")
		}
	})
	bindings.Bind("maduin-test-kiro-run-uses-required-argv-and-workdir", func(t *testing.T) { assertKiroTrace(t, "medium") })
	bindings.Bind("maduin-test-kiro-run-refuses-invalid-agent-and-model-without-spawn", func(t *testing.T) { assertKiroRejectsWithoutSpawn(t) })
	bindings.Bind("maduin-test-kiro-sentinel-rejects-false-success-and-hooks-once", func(t *testing.T) { assertKiroFailureCallback(t) })
	bindings.Bind("maduin-test-kiro-diff-includes-unstaged-staged-and-untracked", func(t *testing.T) { assertKiroDiffUnknown(t) })
	bindings.Bind("maduin-test-kiro-worker-forbids-interactive-test-commands", func(t *testing.T) { assertKiroNoInteractive(t) })
	bindings.Bind("maduin-test-kiro-delete-kills-local-state-only", func(t *testing.T) { assertKiroDelete(t) })
	bindings.Bind("maduin-test-kiro-delete-terminates-descendants-before-parent", func(t *testing.T) { assertKiroDelete(t) })
	bindings.Bind("maduin-test-kiro-tui-is-interactive-command-line", func(t *testing.T) { assertKiroNoInteractive(t) })
	bindings.Bind("maduin-test-kiro-limit-patterns-cover-usage-and-credit", func(t *testing.T) { assertLimitPatterns(t) })
	bindings.Bind("maduin-test-kiro-completion-statuses-remain-local", func(t *testing.T) { assertKiroFailureCallback(t) })

	bindings.Bind("maduin-test-backend-register-get-and-reject-malformed", func(t *testing.T) { assertRegistry(t) })
	bindings.Bind("maduin-test-backend-resolve-priority-and-unknown-selection", func(t *testing.T) { assertBackendResolution(t) })
	bindings.Bind("maduin-test-backend-missing-executable-does-not-dispatch", func(t *testing.T) { assertMissingCLI(t) })
	bindings.Bind("maduin-test-backend-run-forwards-effort", func(t *testing.T) { assertEffortForwarding(t, "high") })
	bindings.Bind("maduin-test-backend-run-nil-effort", func(t *testing.T) { assertEffortForwarding(t, "") })
	bindings.Bind("maduin-test-backend-tui-forwards-effort", func(t *testing.T) { assertEffortForwarding(t, "medium") })
	bindings.Bind("maduin-test-backend-unknown-backend-still-nil", func(t *testing.T) { assertUnknownBackend(t) })
	bindings.Bind("maduin-test-backend-resolve-uses-concrete-config-precedence", func(t *testing.T) { assertBackendFallback(t) })
	bindings.Run()
}

func assertParseLine(t *testing.T, line string, want agent.Status) {
	t.Helper()
	event, ok := opencode.ParseLine(line)
	if want == agent.StatusRunning {
		if !ok || event.Terminal != "" {
			t.Fatalf("ParseLine(%q) = (%#v, %t), want nonterminal", line, event, ok)
		}
		return
	}
	if !ok || event.Terminal != want {
		t.Fatalf("ParseLine(%q) = (%#v, %t), want %q", line, event, ok, want)
	}
}

func assertTranscript(t *testing.T) {
	t.Helper()
	stream := "{\"type\":\"step_start\",\"sessionID\":\"ses\"}\n{" + "\"type\":\"step_finish\",\"part\":{\"reason\":\"stop\"}}"
	scanner := opencode.NewScanner()
	_, _ = scanner.Write([]byte(stream[:13]))
	_, _ = scanner.Write([]byte(stream[13:]))
	scanner.Flush()
	if scanner.Transcript() != stream || scanner.SessionID() != "ses" || scanner.Status() != agent.StatusCompleted {
		t.Fatalf("scanner = (%q, %q, %q)", scanner.Transcript(), scanner.SessionID(), scanner.Status())
	}
}

func assertMissingCLI(t *testing.T) {
	t.Helper()
	adapter := opencode.New(opencode.Options{Executable: "magicite-parity-no-such-agent"})
	if _, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: t.TempDir(), Plan: "plan"}); !errors.Is(err, agent.ErrExecutableMissing) {
		t.Fatalf("Run missing = %v", err)
	}
}

func runtimeWithAdapter(t *testing.T) (*agent.Runtime, *parityAdapter) {
	t.Helper()
	adapter := &parityAdapter{name: "parity", executable: os.Args[0], handle: "parity-1", status: agent.StatusCompleted, output: "output", diffs: []agent.FileDiff{{File: "a.go", Status: "modified"}}}
	registry := agent.NewRegistry()
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	return agent.NewRuntime(registry), adapter
}

func assertUnknownRuntime(t *testing.T, method string) {
	t.Helper()
	runtime, _ := runtimeWithAdapter(t)
	ctx := context.Background()
	var err error
	switch method {
	case "complete":
		_, err = runtime.Complete(ctx, "missing")
	case "diff":
		_, err = runtime.Diff(ctx, "missing")
	}
	if !errors.Is(err, agent.ErrUnknownHandle) {
		t.Fatalf("%s unknown = %v", method, err)
	}
}

func assertRuntimeRoutes(t *testing.T) {
	t.Helper()
	runtime, _ := runtimeWithAdapter(t)
	handle, err := runtime.Run(context.Background(), "parity", agent.RunSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if status, err := runtime.Complete(context.Background(), handle); err != nil || status != agent.StatusCompleted {
		t.Fatalf("Complete = %q, %v", status, err)
	}
	if output, err := runtime.Output(context.Background(), handle); err != nil || output != "output" {
		t.Fatalf("Output = %q, %v", output, err)
	}
	if diffs, err := runtime.Diff(context.Background(), handle); err != nil || len(diffs) != 1 {
		t.Fatalf("Diff = %#v, %v", diffs, err)
	}
}

func assertRuntimeFailure(t *testing.T) {
	t.Helper()
	runtime, adapter := runtimeWithAdapter(t)
	adapter.status = agent.StatusFailed
	handle, err := runtime.Run(context.Background(), "parity", agent.RunSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if status, _ := runtime.Complete(context.Background(), handle); status != agent.StatusFailed {
		t.Fatalf("status = %q", status)
	}
}

func assertBackendsRegistered(t *testing.T) {
	t.Helper()
	registry := agent.NewRegistry()
	if err := backends.Register(registry, config.Default()); err != nil {
		t.Fatal(err)
	}
	if got, want := registry.Names(), []string{"kiro", "opencode"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names = %v", got)
	}
}

func assertOpenCodeArgs(t *testing.T, effort string) {
	t.Helper()
	args := opencode.RunArgs("opencode", "/work", "provider/model", "worker", effort, "opencode-1", "plan")
	if args[0] != "opencode" || args[1] != "run" || args[2] != "--dir" || args[len(args)-1] != "plan" {
		t.Fatalf("args = %q", args)
	}
	if effort != "" && opencode.ValidEffort(effort) && !contains(args, effort) {
		t.Fatalf("effort missing: %q", args)
	}
}
func assertOpenCodeEffortPosition(t *testing.T) {
	t.Helper()
	args := opencode.RunArgs("opencode", "/work", "model", "worker", "high", "h", "plan")
	if index(args, "--variant") != 6 || index(args, "--agent") != 8 {
		t.Fatalf("args = %q", args)
	}
}
func assertOpenCodeOptionalEffort(t *testing.T) {
	t.Helper()
	without := opencode.RunArgs("opencode", "/work", "model", "", "", "h", "plan")
	with := opencode.RunArgs("opencode", "/work", "model", "", "low", "h", "plan")
	if len(with) != len(without)+2 {
		t.Fatalf("optional argv lengths = %d, %d", len(without), len(with))
	}
}

func assertCompletionOnce(t *testing.T) {
	t.Helper()
	runtime, adapter := runtimeWithAdapter(t)
	var mu sync.Mutex
	count := 0
	runtime.OnComplete(func(agent.Handle, agent.Status) { mu.Lock(); count++; mu.Unlock() })
	handle, err := runtime.Run(context.Background(), "parity", agent.RunSpec{})
	if err != nil {
		t.Fatal(err)
	}
	adapter.notify(handle, agent.StatusCompleted)
	adapter.notify(handle, agent.StatusCompleted)
	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("completion count = %d", count)
	}
}

func assertRuntimeDelete(t *testing.T) {
	t.Helper()
	runtime, adapter := runtimeWithAdapter(t)
	handle, err := runtime.Run(context.Background(), "parity", agent.RunSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.Delete(context.Background(), handle); err != nil || !adapter.deleted {
		t.Fatalf("Delete = %v, deleted=%t", err, adapter.deleted)
	}
	if _, err := runtime.Output(context.Background(), handle); !errors.Is(err, agent.ErrUnknownHandle) {
		t.Fatalf("Output deleted = %v", err)
	}
}

func assertConfiguredKiroAgents(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	for _, role := range []string{"slugineer-planner-concierge", "slugineer-planner-designer", "slugineer-worker", "slugineer-reviewer", "slugineer-repairer"} {
		if err := os.WriteFile(filepath.Join(dir, role+".json"), []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
		if !kiro.ValidAgent(dir, role) {
			t.Fatalf("agent %q absent", role)
		}
	}
}
func assertKiroAgentAndPlan(t *testing.T) {
	t.Helper()
	args := kiro.RunArgs("kiro", "model", "worker", "", "line one\nline two")
	if args[len(args)-1] != "line one\nline two" || !contains(args, "--agent") {
		t.Fatalf("args = %q", args)
	}
}
func assertKiroArgs(t *testing.T, effort string) {
	t.Helper()
	args := kiro.RunArgs("kiro", "model", "worker", effort, "plan")
	if args[1] != "chat" || !contains(args, "--no-interactive") || !contains(args, "--trust-all-tools") {
		t.Fatalf("args = %q", args)
	}
	if kiro.ValidEffort(effort) != contains(args, "--effort") {
		t.Fatalf("effort args = %q", args)
	}
}
func assertKiroEffortAllowlist(t *testing.T) {
	t.Helper()
	for _, value := range []string{"low", "medium", "high", "xhigh", "max"} {
		if !kiro.ValidEffort(value) {
			t.Fatalf("effort %q rejected", value)
		}
	}
	if kiro.ValidEffort("higher") {
		t.Fatal("unknown effort accepted")
	}
}

func kiroFixture(t *testing.T, scenario string) (*testenv.Agent, *kiro.Adapter, string) {
	t.Helper()
	env := testenv.New(t)
	fake := testenv.NewAgent(t, env, "kiro")
	fake.Scenario(scenario)
	agentsDir := filepath.Join(env.Root, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "worker.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	return fake, kiro.New(kiro.Options{Executable: env.Bin("kiro"), AgentsDir: agentsDir, Env: env.Env()}), env.Root
}
func waitKiro(t *testing.T, adapter *kiro.Adapter, handle agent.Handle, want agent.Status, done <-chan agent.Status) {
	t.Helper()
	select {
	case status := <-done:
		if status != want {
			t.Fatalf("callback status = %q, want %q", status, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Kiro completion timed out")
	}
	if status, err := adapter.Complete(context.Background(), handle); err != nil || status != want {
		t.Fatalf("Complete = %q, %v", status, err)
	}
}
func assertKiroTrace(t *testing.T, effort string) {
	t.Helper()
	fake, adapter, root := kiroFixture(t, "complete")
	done := make(chan agent.Status, 1)
	handle, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: root, Model: "model", Agent: "worker", Effort: effort, Plan: "task", Notify: func(_ agent.Handle, status agent.Status) { done <- status }})
	if err != nil {
		t.Fatal(err)
	}
	waitKiro(t, adapter, handle, agent.StatusCompleted, done)
	calls := fake.Calls()
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || !samePath(calls[0].Dir, resolvedRoot) || !contains(calls[0].Argv, "--effort") || !contains(calls[0].Argv, effort) {
		t.Fatalf("trace = %#v", calls)
	}
	_ = adapter.Delete(context.Background(), handle)
}
func assertKiroRejectsWithoutSpawn(t *testing.T) {
	t.Helper()
	fake, adapter, root := kiroFixture(t, "complete")
	if _, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: root, Model: "bad/model", Agent: "../worker", Plan: "task"}); err == nil {
		t.Fatal("invalid run accepted")
	}
	if calls := fake.Calls(); len(calls) != 0 {
		t.Fatalf("invalid run spawned %#v", calls)
	}
}
func assertKiroFailureCallback(t *testing.T) {
	t.Helper()
	_, adapter, root := kiroFixture(t, "failed")
	done := make(chan agent.Status, 1)
	handle, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: root, Model: "model", Agent: "worker", Plan: "task", Notify: func(_ agent.Handle, status agent.Status) { done <- status }})
	if err != nil {
		t.Fatal(err)
	}
	waitKiro(t, adapter, handle, agent.StatusFailed, done)
	_ = adapter.Delete(context.Background(), handle)
}
func assertKiroDiffUnknown(t *testing.T) {
	t.Helper()
	adapter := kiro.New(kiro.Options{})
	if _, err := adapter.Diff(context.Background(), "missing"); !errors.Is(err, agent.ErrUnknownHandle) {
		t.Fatalf("Diff missing = %v", err)
	}
}
func assertKiroNoInteractive(t *testing.T) {
	t.Helper()
	if contains(kiro.RunArgs("kiro", "model", "", "", "plan"), "--interactive") {
		t.Fatal("interactive flag present")
	}
}
func assertKiroDelete(t *testing.T) {
	t.Helper()
	_, adapter, root := kiroFixture(t, "complete")
	done := make(chan agent.Status, 1)
	handle, err := adapter.Run(context.Background(), agent.RunSpec{Workdir: root, Model: "model", Agent: "worker", Plan: "task", Notify: func(_ agent.Handle, status agent.Status) { done <- status }})
	if err != nil {
		t.Fatal(err)
	}
	waitKiro(t, adapter, handle, agent.StatusCompleted, done)
	if err := adapter.Delete(context.Background(), handle); err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Output(context.Background(), handle); !errors.Is(err, agent.ErrUnknownHandle) {
		t.Fatalf("Output deleted = %v", err)
	}
}
func assertLimitPatterns(t *testing.T) {
	t.Helper()
	for _, text := range []string{"usage limit reached", "credit limit", "429"} {
		if !agent.LimitedTail(text) {
			t.Fatalf("limit %q ignored", text)
		}
	}
}

func assertRegistry(t *testing.T) {
	t.Helper()
	registry := agent.NewRegistry()
	if err := registry.Register(nil); !errors.Is(err, agent.ErrInvalidAdapter) {
		t.Fatalf("Register nil = %v", err)
	}
	adapter := &parityAdapter{name: "parity", executable: os.Args[0]}
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Lookup("parity"); err != nil {
		t.Fatal(err)
	}
}
func assertBackendResolution(t *testing.T) {
	t.Helper()
	cfg := config.Default()
	cfg.Crew.Backend = config.BackendKiro
	got, err := config.Resolve(cfg, "implementer", config.DifficultyLow)
	if err != nil || got.Backend != config.BackendKiro || got.Model != "gpt-5.6-luna" {
		t.Fatalf("Resolve = %#v, %v", got, err)
	}
}
func assertEffortForwarding(t *testing.T, effort string) {
	t.Helper()
	runtime, adapter := runtimeWithAdapter(t)
	_, err := runtime.Run(context.Background(), "parity", agent.RunSpec{Effort: effort})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.spec.Effort != effort {
		t.Fatalf("effort = %q", adapter.spec.Effort)
	}
}
func assertUnknownBackend(t *testing.T) {
	t.Helper()
	runtime := agent.NewRuntime(agent.NewRegistry())
	if _, err := runtime.Run(context.Background(), "missing", agent.RunSpec{}); !errors.Is(err, agent.ErrUnknownBackend) {
		t.Fatalf("Run unknown = %v", err)
	}
}
func assertBackendFallback(t *testing.T) {
	t.Helper()
	cfg := config.Default()
	cfg.Crew.Backend = config.BackendKiro
	fallback, err := config.FallbackModel(cfg, "implementer")
	if err != nil || fallback != "gpt-5.6-terra" {
		t.Fatalf("FallbackModel = %q, %v", fallback, err)
	}
}

func contains(values []string, want string) bool { return index(values, want) >= 0 }
func index(values []string, want string) int {
	for i, value := range values {
		if value == want {
			return i
		}
	}
	return -1
}
func samePath(path, resolvedWant string) bool {
	resolved, err := filepath.EvalSymlinks(path)
	return err == nil && resolved == resolvedWant
}

type parityAdapter struct {
	name, executable string
	handle           agent.Handle
	status           agent.Status
	output           string
	diffs            []agent.FileDiff
	spec             agent.RunSpec
	notifier         agent.Notifier
	deleted          bool
}

func (a *parityAdapter) Name() string       { return a.name }
func (a *parityAdapter) Executable() string { return a.executable }
func (a *parityAdapter) Run(_ context.Context, spec agent.RunSpec) (agent.Handle, error) {
	a.spec = spec
	a.notifier = spec.Notify
	return a.handle, nil
}
func (a *parityAdapter) Complete(context.Context, agent.Handle) (agent.Status, error) {
	return a.status, nil
}
func (a *parityAdapter) Diff(context.Context, agent.Handle) ([]agent.FileDiff, error) {
	return a.diffs, nil
}
func (a *parityAdapter) Output(context.Context, agent.Handle) (string, error) { return a.output, nil }
func (a *parityAdapter) Delete(context.Context, agent.Handle) error           { a.deleted = true; return nil }
func (a *parityAdapter) UsageLimited(context.Context, agent.Handle) bool {
	return a.status == agent.StatusLimited
}
func (a *parityAdapter) notify(handle agent.Handle, status agent.Status) {
	if a.notifier != nil {
		a.notifier(handle, status)
	}
}
