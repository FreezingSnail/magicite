package parity

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/agent/kiro"
	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/dispatch"
	"github.com/connorfranc/magicite/internal/repo"
	"github.com/connorfranc/magicite/internal/testenv"
)

func TestDispatchParity(t *testing.T) {
	for _, test := range []struct {
		name       string
		invariants []string
		run        func(*testing.T, *dispatchFixture)
	}{
		{"ready-poll", []string{"maduin-test-dispatch-queue-ready-entry", "maduin-test-dispatch-queue-source-order"}, dispatchReadyPoll},
		{"claim-dispatch", []string{"maduin-test-dispatch-syncs-seat-before-claim"}, dispatchClaim},
		{"dispatch-refused-human", []string{"maduin-test-dispatch-functions-exist"}, dispatchHuman},
		{"dispatch-role-cap", []string{"maduin-test-dispatch-implement-concurrency-cap"}, dispatchRoleCap},
		{"dispatch-seat-conflict", []string{"maduin-test-dispatch-syncs-seat-before-claim"}, dispatchSeatConflict},
		{"dispatch-fallback-retry", []string{"maduin-test-dispatch-usage-limit-falls-back"}, dispatchFallback},
		{"tick-multirepo", []string{"maduin-test-dispatch-queue-source-order"}, dispatchMultiRepo},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newDispatchFixture(t, "repo")
			test.run(t, fixture)
			assertDispatchCounterparts(t, test.invariants...)
			entries := readTrace(t, fixture.env)
			AssertTrace(t, test.name, entries)
		})
	}
}

func dispatchReadyPoll(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.dispatcher.Tick(context.Background())
}

func dispatchClaim(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.seed(bead("task-1"))
	if handle := fixture.dispatcher.Implement(context.Background(), fixture.repos[0], "task-1"); handle == "" {
		t.Fatal("Implement() refused ready task")
	}
}

func dispatchHuman(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.seed(bead("task-1", "human"))
	if handle := fixture.dispatcher.Implement(context.Background(), fixture.repos[0], "task-1"); handle != "" {
		t.Fatalf("Implement() = %q, want refusal", handle)
	}
	if len(fixture.agent.Calls()) != 0 {
		t.Fatal("human task started an agent")
	}
}

func dispatchRoleCap(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.seed(bead("task-1"))
	fixture.dispatcher.Add(dispatch.Session{Handle: "busy", Role: dispatch.Implementer, Seat: "ifrit"})
	if handle := fixture.dispatcher.Implement(context.Background(), fixture.repos[0], "task-1"); handle != "" {
		t.Fatalf("Implement() = %q, want cap refusal", handle)
	}
}

func dispatchSeatConflict(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.seed(bead("task-1"))
	fixture.spaces.conflict = true
	if handle := fixture.dispatcher.Implement(context.Background(), fixture.repos[0], "task-1"); handle != "" {
		t.Fatalf("Implement() = %q, want conflict refusal", handle)
	}
}

func dispatchFallback(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.seed(bead("task-1"))
	fixture.agent.Scenario("limited")
	handle := fixture.dispatcher.Implement(context.Background(), fixture.repos[0], "task-1")
	if handle == "" {
		t.Fatal("Implement() refused ready task")
	}
	sessions := fixture.dispatcher.Sessions()
	if len(sessions) != 1 {
		t.Fatalf("sessions = %#v, want one", sessions)
	}
	if !fixture.dispatcher.FallbackRetry(context.Background(), sessions[0], handle, dispatch.Limited) {
		t.Fatal("FallbackRetry() = false")
	}
	if got, ok := fixture.beads.Bead("task-1"); !ok || got.Status != "in_progress" {
		t.Fatalf("task status = %q, want in_progress", got.Status)
	}
}

func dispatchMultiRepo(t *testing.T, fixture *dispatchFixture) {
	t.Helper()
	fixture.addRepo("alpha")
	fixture.addRepo("beta")
	fixture.order = newRepoOrder(fixture.repos)
	fixture.beadPort.order = fixture.order
	fixture.dispatcher.Tick(context.Background())
}

func assertDispatchCounterparts(t *testing.T, names ...string) {
	t.Helper()
	counterparts := OrchestrationCounterparts()
	for _, name := range names {
		want := "TestMaduinDispatchParity/" + name
		if got := counterparts[name]; got != want {
			t.Fatalf("counterpart[%q] = %q, want %q", name, got, want)
		}
	}
}

type dispatchFixture struct {
	t          *testing.T
	env        *testenv.Env
	beads      *testenv.BD
	agent      *testenv.Agent
	git        []*testenv.Repo
	repos      []repo.Repo
	spaces     *fixtureSpaces
	beadPort   *dispatchBeads
	dispatcher *dispatch.Dispatcher
	order      *repoOrder
}

func newDispatchFixture(t *testing.T, first string) *dispatchFixture {
	t.Helper()
	env := testenv.New(t)
	for _, variable := range env.Env() {
		key, value, ok := strings.Cut(variable, "=")
		if ok && key != "PATH" {
			t.Setenv(key, value)
		}
	}
	beads := testenv.NewBD(t, env)
	agentFake := testenv.NewAgent(t, env, "kiro")
	t.Setenv("PATH", env.BinDir+":"+os.Getenv("PATH"))
	for _, variable := range env.Env() {
		key, value, ok := strings.Cut(variable, "=")
		if ok && key != "PATH" {
			t.Setenv(key, value)
		}
	}
	fixture := &dispatchFixture{t: t, env: env, beads: beads, agent: agentFake}
	fixture.addRepo(first)

	agentsDir := filepath.Join(env.Root, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(agentsDir, "a.json"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	registry := agent.NewRegistry()
	if err := registry.Register(kiro.New(kiro.Options{Executable: "kiro", AgentsDir: agentsDir, Env: env.Env()})); err != nil {
		t.Fatal(err)
	}
	runtime := agent.NewRuntime(registry)
	completed := make(chan agent.Handle, 2)
	runtime.OnComplete(func(handle agent.Handle, _ agent.Status) { completed <- handle })
	fixture.spaces = &fixtureSpaces{fixture: fixture, paths: make(map[string]string)}
	fixture.beadPort = &dispatchBeads{fixture: fixture}
	cfg := config.Default()
	cfg.Crew.Backend = config.BackendKiro
	cfg.Fleet.Agent = "a"
	cfg.Fleet.KiroModel = "m"
	cfg.Fleet.KiroModelHigh = "m"
	cfg.Fleet.KiroFallback = "fallback"
	cfg.Fleet.KiroEffortHigh = "high"
	cfg.Fleet.Seats = []config.SeatConfig{{Name: "ifrit", Role: "implementer"}}
	d, err := dispatch.New(dispatch.Deps{
		Beads: fixture.beadPort, Workspaces: fixture.spaces,
		Lander: dispatchLander{}, Runner: dispatchRunner{runtime: runtime, completed: completed}, Repos: dispatchRepos{fixture: fixture},
		Gate: dispatch.PermissiveGate{}, Clock: parityClock{}, Config: cfg,
	})
	if err != nil {
		t.Fatal(err)
	}
	fixture.dispatcher = d
	if _, err := fixture.spaces.Ensure(context.Background(), fixture.repos[0], "ifrit"); err != nil {
		t.Fatal(err)
	}
	return fixture
}

func (f *dispatchFixture) addRepo(name string) {
	f.git = append(f.git, testenv.NewRepo(f.t, f.env, name))
	record, ok := repo.Make(f.git[len(f.git)-1].Root, name, name, "main")
	if !ok {
		f.t.Fatal("make fixture repository record")
	}
	f.repos = append(f.repos, record)
}

func (f *dispatchFixture) seed(items ...testenv.Bead) { f.beads.Seed(items...) }

func bead(id string, labels ...string) testenv.Bead {
	if len(labels) == 0 {
		labels = []string{"difficulty:high"}
	}
	return testenv.Bead{ID: id, Title: "task", Description: "description", Status: "open", Priority: 1, IssueType: "task", Labels: labels}
}

type dispatchBeads struct {
	fixture *dispatchFixture
	order   *repoOrder
}

func (b dispatchBeads) client(r repo.Repo) (*bd.Client, error) {
	if b.order != nil {
		b.order.wait(r)
	}
	client, err := bd.New("bd", r.Root)
	if err != nil {
		return nil, err
	}
	client.Env = b.fixture.env.Env()
	return client, nil
}

func (b dispatchBeads) Ready(ctx context.Context, r repo.Repo) ([]dispatch.ReadyEntry, error) {
	client, err := b.client(r)
	if err != nil {
		return nil, err
	}
	defer b.complete()
	items, err := client.Ready(ctx)
	return readyEntries(r, items), err
}
func (b dispatchBeads) Show(ctx context.Context, r repo.Repo, id string) (dispatch.Spec, error) {
	client, err := b.client(r)
	if err != nil {
		return dispatch.Spec{}, err
	}
	item, err := client.Show(ctx, id)
	return dispatch.Spec{Title: item.Title, Description: item.Description, Design: item.Design, Acceptance: item.AcceptanceCriteria}, err
}
func (b dispatchBeads) Claim(ctx context.Context, r repo.Repo, id string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Claim(ctx, id)
}
func (b dispatchBeads) Release(ctx context.Context, r repo.Repo, id string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Release(ctx, id)
}
func (b dispatchBeads) Close(ctx context.Context, r repo.Repo, id, text string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Close(ctx, id, text)
}
func (b dispatchBeads) Comment(ctx context.Context, r repo.Repo, id, text string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Comment(ctx, id, text)
}
func (b dispatchBeads) Difficulty(ctx context.Context, r repo.Repo, id string) (string, error) {
	labels, err := b.labels(ctx, r, id)
	return bd.DifficultyFromLabels(labels), err
}
func (b dispatchBeads) HumanOnly(ctx context.Context, r repo.Repo, id string) (bool, error) {
	labels, err := b.labels(ctx, r, id)
	return bd.IsHuman(labels), err
}
func (b dispatchBeads) InProgress(ctx context.Context, r repo.Repo) ([]string, error) {
	return b.query(ctx, r, bd.InProgressQuery())
}
func (b dispatchBeads) OpenEpics(ctx context.Context, r repo.Repo) ([]string, error) {
	return b.query(ctx, r, bd.OpenEpicsQuery())
}
func (b dispatchBeads) EpicChildren(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	return b.query(ctx, r, bd.EpicChildrenQuery(id))
}
func (b dispatchBeads) EpicOpenChildren(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	return b.query(ctx, r, bd.EpicOpenChildrenQuery(id))
}
func (b dispatchBeads) DriftFixTasks(ctx context.Context, r repo.Repo) ([]string, error) {
	return b.query(ctx, r, bd.DriftFixQuery())
}
func (dispatchBeads) CancelAll(context.Context) error { return nil }
func (b dispatchBeads) labels(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	client, err := b.client(r)
	if err != nil {
		return nil, err
	}
	return client.Labels(ctx, id)
}
func (b dispatchBeads) query(ctx context.Context, r repo.Repo, query string) ([]string, error) {
	client, err := b.client(r)
	if err != nil {
		return nil, err
	}
	defer b.complete()
	items, err := client.Query(ctx, query, false)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
	}
	return ids, nil
}
func (b dispatchBeads) complete() {
	if b.order != nil {
		b.order.done()
	}
}

func readyEntries(r repo.Repo, items []bd.Bead) []dispatch.ReadyEntry {
	entries := make([]dispatch.ReadyEntry, len(items))
	for i, item := range items {
		entries[i] = dispatch.ReadyEntry{Repo: r, Task: item.ID, Priority: fmt.Sprint(item.Priority)}
	}
	return entries
}

type fixtureSpaces struct {
	fixture  *dispatchFixture
	paths    map[string]string
	conflict bool
}

func (w *fixtureSpaces) Ensure(_ context.Context, r repo.Repo, seat string) (string, error) {
	key := r.Name + "\x00" + seat
	if path := w.paths[key]; path != "" {
		return path, nil
	}
	for _, candidate := range w.fixture.git {
		if !repo.SameRoot(candidate.Root, r.Root) {
			continue
		}
		candidate.Branch(seat, "main")
		path := candidate.Worktree(seat, seat)
		w.paths[key] = path
		return path, nil
	}
	return "", fmt.Errorf("unknown fixture repository %q", r.Root)
}

func (w *fixtureSpaces) Path(r repo.Repo, seat string) (string, error) {
	path := w.paths[r.Name+"\x00"+seat]
	if path == "" {
		return "", fmt.Errorf("seat %q not ensured", seat)
	}
	return path, nil
}

func (w *fixtureSpaces) Sync(context.Context, repo.Repo, string) (dispatch.SyncResult, error) {
	if w.conflict {
		return dispatch.SyncConflict, nil
	}
	return dispatch.SyncOK, nil
}

type dispatchRunner struct {
	runtime   *agent.Runtime
	completed <-chan agent.Handle
}

func (r dispatchRunner) Run(ctx context.Context, spec dispatch.RunSpec) (string, error) {
	handle, err := r.runtime.Run(ctx, spec.Backend, agent.RunSpec{Workdir: spec.Workdir, Model: spec.Model, Agent: spec.Agent, Effort: spec.Effort, Plan: spec.Plan})
	if err != nil {
		return "", err
	}
	if completed := <-r.completed; completed != handle {
		return "", fmt.Errorf("completion handle = %q, want %q", completed, handle)
	}
	return string(handle), nil
}
func (r dispatchRunner) Diff(ctx context.Context, handle string) ([]dispatch.Diff, error) {
	items, err := r.runtime.Diff(ctx, agent.Handle(handle))
	result := make([]dispatch.Diff, len(items))
	for i, item := range items {
		result[i] = dispatch.Diff{File: item.File, Patch: item.Patch, Status: item.Status, Additions: item.Additions, Deletions: item.Deletions}
	}
	return result, err
}
func (r dispatchRunner) Output(ctx context.Context, handle string) (string, error) {
	return r.runtime.Output(ctx, agent.Handle(handle))
}
func (r dispatchRunner) Delete(ctx context.Context, handle string) error {
	return r.runtime.Delete(ctx, agent.Handle(handle))
}
func (r dispatchRunner) UsageLimited(ctx context.Context, handle string) (bool, error) {
	return r.runtime.UsageLimited(ctx, agent.Handle(handle))
}

type dispatchLander struct{}

func (dispatchLander) Land(context.Context, repo.Repo, string, dispatch.Stamp) (dispatch.LandResult, error) {
	return dispatch.LandOK, nil
}
func (dispatchLander) Landed(context.Context, repo.Repo, string) (bool, error)     { return true, nil }
func (dispatchLander) TaskLanded(context.Context, repo.Repo, string) (bool, error) { return true, nil }

type dispatchRepos struct{ fixture *dispatchFixture }

func (r dispatchRepos) List(context.Context) []repo.Repo {
	return append([]repo.Repo(nil), r.fixture.repos...)
}
func (r dispatchRepos) Current(context.Context, string) (repo.Repo, error) {
	return repo.Repo{}, fmt.Errorf("not configured")
}

type parityClock struct{}

func (parityClock) Now() time.Time                       { return time.Unix(0, 0) }
func (parityClock) Ticker(time.Duration) dispatch.Ticker { return parityTicker{} }

type parityTicker struct{}

func (parityTicker) Chan() <-chan time.Time { return nil }
func (parityTicker) Stop()                  {}

type repoOrder struct {
	mu    sync.Mutex
	cond  *sync.Cond
	repos []repo.Repo
	next  int
}

func newRepoOrder(repos []repo.Repo) *repoOrder {
	order := &repoOrder{repos: append([]repo.Repo(nil), repos...)}
	order.cond = sync.NewCond(&order.mu)
	return order
}
func (o *repoOrder) wait(r repo.Repo) {
	o.mu.Lock()
	defer o.mu.Unlock()
	for o.repos[o.next] != r {
		o.cond.Wait()
	}
}
func (o *repoOrder) done() {
	o.mu.Lock()
	o.next = (o.next + 1) % len(o.repos)
	o.cond.Broadcast()
	o.mu.Unlock()
}
