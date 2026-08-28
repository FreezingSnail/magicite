package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/agent/backends"
	"github.com/connorfranc/magicite/internal/bd"
	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/dispatch"
	"github.com/connorfranc/magicite/internal/gate"
	"github.com/connorfranc/magicite/internal/land"
	"github.com/connorfranc/magicite/internal/logging"
	"github.com/connorfranc/magicite/internal/repo"
	"github.com/connorfranc/magicite/internal/server"
	"github.com/connorfranc/magicite/internal/stamp"
	"github.com/connorfranc/magicite/internal/state"
	"github.com/connorfranc/magicite/internal/version"
	"github.com/connorfranc/magicite/internal/worktree"
)

// Assembly contains an assembled daemon before it begins serving.
type Assembly struct {
	Core   server.Core
	Router *server.Router
	Bus    *server.Bus
	State  *state.Store
	Socket string
}

// Assemble builds all production ports from cfgPath.
func Assemble(ctx context.Context, cfgPath string) (*Assembly, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("daemon: load config: %w", err)
	}
	bus := server.NewBus(1024)
	logging.Configure(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: busWriter{bus: bus}})
	log := logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: busWriter{bus: bus}})
	repos := repo.NewWith(repo.Options{Repos: cfg.Repos, Log: log.Event})
	records := repos.List(ctx)
	beads, err := newBeads(records, log)
	if err != nil {
		return nil, fmt.Errorf("daemon: build bd client: %w", err)
	}
	gitRunner := worktree.ExecRunner()
	workspaces, err := worktree.New(worktree.Options{WorkspacePath: cfg.Workspaces.Path, Runner: gitRunner, Log: log.Event})
	if err != nil {
		return nil, fmt.Errorf("daemon: build workspaces: %w", err)
	}
	lander, err := land.New(land.Options{Workspace: landWorkspace{manager: workspaces}, Runner: gitRunner, Log: func(level, message string) {
		log.Event(logging.Info, "land", map[string]any{"level": level, "message": message})
	}})
	if err != nil {
		return nil, fmt.Errorf("daemon: build land pipeline: %w", err)
	}
	runtime, err := backends.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("daemon: build agent runtime: %w", err)
	}
	store := state.Default()
	qualityGate, err := gate.New(gate.Deps{
		Beads:  gateBeadsAdapter{beads: beads},
		Git:    gateGit{runner: gitRunner},
		Repos:  newGateRepos(records),
		Config: gate.Config{Enabled: cfg.Reviewer.Enabled, Model: cfg.Reviewer.Model, Agent: cfg.Reviewer.Agent, MaxRetries: cfg.Reviewer.MaxRetries},
		Log:    *log,
	})
	if err != nil {
		return nil, fmt.Errorf("daemon: build gate: %w", err)
	}
	dispatcher, err := dispatch.New(dispatch.Deps{Beads: beads, Workspaces: dispatchWorkspace{manager: workspaces}, Lander: landAdapter{pipeline: lander}, Runner: runnerAdapter{runtime: runtime}, Repos: repos, Gate: qualityGate, Clock: wallClock{}, Config: cfg, Logger: func(level logging.Level, kind string, fields map[string]any) { log.Event(level, kind, fields) }})
	if err != nil {
		return nil, fmt.Errorf("daemon: build dispatcher: %w", err)
	}
	core, err := NewCore(Deps{Config: cfg, Log: *log, Dispatcher: dispatcher, Beads: beads, Repos: repos, Gate: qualityGate, Bus: bus, Version: version.Info()})
	if err != nil {
		return nil, fmt.Errorf("daemon: build core: %w", err)
	}
	router := server.NewRouter(*log)
	if err := server.RegisterRead(router, core); err != nil {
		return nil, fmt.Errorf("daemon: register read commands: %w", err)
	}
	if err := server.RegisterControl(router, core); err != nil {
		return nil, fmt.Errorf("daemon: register control commands: %w", err)
	}
	return &Assembly{Core: core, Router: router, Bus: bus, State: store, Socket: server.SocketPath(cfg)}, nil
}

// Run assembles and serves the daemon until ctx is cancelled.
func Run(ctx context.Context, cfgPath string) error {
	assembly, err := Assemble(ctx, cfgPath)
	if err != nil {
		return err
	}
	return server.Serve(ctx, server.Deps{Router: assembly.Router, Bus: assembly.Bus, Socket: assembly.Socket})
}

type busWriter struct{ bus *server.Bus }

func (w busWriter) Write(data []byte) (int, error) {
	if w.bus == nil {
		return len(data), nil
	}
	var record struct {
		Level  string                     `json:"level"`
		Kind   string                     `json:"kind"`
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(data, &record); err != nil {
		return len(data), nil
	}
	fields := make(map[string]string, len(record.Fields))
	for key, value := range record.Fields {
		var text string
		if json.Unmarshal(value, &text) != nil {
			text = string(value)
		}
		fields[key] = text
	}
	w.bus.Log(record.Level, record.Kind, fields)
	return len(data), nil
}

type wallClock struct{}

func (wallClock) Now() time.Time { return time.Now() }
func (wallClock) Ticker(d time.Duration) dispatch.Ticker {
	return wallTicker{Ticker: time.NewTicker(d)}
}

type wallTicker struct{ *time.Ticker }

func (t wallTicker) Chan() <-chan time.Time { return t.C }

type runnerAdapter struct {
	runtime *agent.Runtime
}

func (r runnerAdapter) Run(ctx context.Context, spec dispatch.RunSpec) (string, error) {
	handle, err := r.runtime.Run(ctx, spec.Backend, agent.RunSpec{Workdir: spec.Workdir, Model: spec.Model, Agent: spec.Agent, Effort: spec.Effort, Plan: spec.Plan})
	return string(handle), err
}
func (r runnerAdapter) Diff(ctx context.Context, handle string) ([]dispatch.Diff, error) {
	files, err := r.runtime.Diff(ctx, agent.Handle(handle))
	if err != nil {
		return nil, err
	}
	result := make([]dispatch.Diff, len(files))
	for i, file := range files {
		result[i] = dispatch.Diff{File: file.File, Patch: file.Patch, Status: file.Status, Additions: file.Additions, Deletions: file.Deletions}
	}
	return result, nil
}
func (r runnerAdapter) Output(ctx context.Context, handle string) (string, error) {
	return r.runtime.Output(ctx, agent.Handle(handle))
}
func (r runnerAdapter) Delete(ctx context.Context, handle string) error {
	return r.runtime.Delete(ctx, agent.Handle(handle))
}

type dispatchWorkspace struct{ manager *worktree.Manager }

func (w dispatchWorkspace) Ensure(ctx context.Context, r repo.Repo, seat string) (string, error) {
	return w.manager.Ensure(ctx, repository{Repo: r}, seat)
}
func (w dispatchWorkspace) Path(r repo.Repo, seat string) (string, error) {
	return w.manager.Path(repository{Repo: r}, seat)
}
func (w dispatchWorkspace) Sync(ctx context.Context, r repo.Repo, seat string) (dispatch.SyncResult, error) {
	result, err := w.manager.Sync(ctx, repository{Repo: r}, seat)
	switch result {
	case worktree.SyncConflict:
		return dispatch.SyncConflict, err
	case worktree.SyncSynced:
		return dispatch.SyncOK, err
	default:
		return dispatch.SyncConflict, err
	}
}

type landWorkspace struct{ manager *worktree.Manager }

func (w landWorkspace) Branch(r land.Repo, seat string) (string, error) {
	return w.manager.Branch(r, seat)
}
func (w landWorkspace) Path(r land.Repo, seat string) (string, error) { return w.manager.Path(r, seat) }

type repository struct{ repo.Repo }

func (r repository) Name() string        { return r.Repo.Name }
func (r repository) Root() string        { return r.Repo.Root }
func (r repository) Integration() string { return r.Repo.Branch }
func (r runnerAdapter) UsageLimited(ctx context.Context, handle string) (bool, error) {
	return r.runtime.UsageLimited(ctx, agent.Handle(handle))
}
func (r runnerAdapter) OnComplete(callback func(string, dispatch.Outcome)) {
	r.runtime.OnComplete(func(handle agent.Handle, status agent.Status) {
		outcome := dispatch.Limited
		if status == agent.StatusCompleted {
			outcome = dispatch.Completed
		}
		callback(string(handle), outcome)
	})
}
func (runnerAdapter) OnPhase(func(string, string)) {}

type gateGit struct{ runner worktree.Runner }

func (g gateGit) Output(ctx context.Context, r repo.Repo, args ...string) (int, string, error) {
	return g.runner.Git(ctx, r.Root, args...)
}

type gateRepos map[string]repo.Repo

func newGateRepos(records []repo.Repo) gateRepos {
	result := make(gateRepos, len(records))
	for _, record := range records {
		result[record.Name] = record
	}
	return result
}

func (r gateRepos) Get(name string) (repo.Repo, bool) {
	record, ok := r[name]
	return record, ok
}

type gateBeadsAdapter struct{ beads *beadsAdapter }

func (b gateBeadsAdapter) Show(ctx context.Context, r repo.Repo, id string) (bd.Bead, error) {
	client, err := b.beads.client(r)
	if err != nil {
		return bd.Bead{}, err
	}
	return client.Show(ctx, id)
}
func (b gateBeadsAdapter) Labels(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	client, err := b.beads.client(r)
	if err != nil {
		return nil, err
	}
	return client.Labels(ctx, id)
}
func (b gateBeadsAdapter) EpicChildren(ctx context.Context, r repo.Repo, id string) ([]bd.Bead, error) {
	client, err := b.beads.client(r)
	if err != nil {
		return nil, err
	}
	return client.Query(ctx, "parent:"+id, true)
}
func (b gateBeadsAdapter) Query(ctx context.Context, r repo.Repo, query string) ([]bd.Bead, error) {
	client, err := b.beads.client(r)
	if err != nil {
		return nil, err
	}
	return client.Query(ctx, query, true)
}
func (b gateBeadsAdapter) Create(ctx context.Context, r repo.Repo, request bd.CreateRequest) (string, error) {
	client, err := b.beads.client(r)
	if err != nil {
		return "", err
	}
	return client.Create(ctx, request)
}
func (b gateBeadsAdapter) Comment(ctx context.Context, r repo.Repo, id, text string) error {
	client, err := b.beads.client(r)
	if err != nil {
		return err
	}
	return client.Comment(ctx, id, text)
}
func (b gateBeadsAdapter) Close(ctx context.Context, r repo.Repo, id, reason string) error {
	client, err := b.beads.client(r)
	if err != nil {
		return err
	}
	return client.Close(ctx, id, reason)
}

var _ gate.Git = gateGit{}
var _ gate.Repos = gateRepos{}
var _ gate.Beads = gateBeadsAdapter{}

type landAdapter struct{ pipeline *land.Pipeline }

func (l landAdapter) Land(ctx context.Context, r repo.Repo, seat string, value dispatch.Stamp) (dispatch.LandResult, error) {
	outcome, err := l.pipeline.LandBranch(ctx, repository{Repo: r}, seat, &stamp.Stamp{Model: value.Model, Backend: value.Backend, Difficulty: value.Difficulty, Effort: value.Effort, Agent: value.Agent, Repo: value.Repo, Seat: value.Seat, Task: value.Task, Harness: value.Harness, HarnessRev: value.HarnessRev})
	switch outcome {
	case land.OutcomeLanded:
		return dispatch.LandOK, err
	case land.OutcomeConflict:
		return dispatch.LandConflict, err
	case land.OutcomeGateFailed:
		return dispatch.LandGateFailed, err
	default:
		return dispatch.LandFailed, err
	}
}
func (l landAdapter) Landed(ctx context.Context, r repo.Repo, seat string) (bool, error) {
	return l.pipeline.Landed(ctx, repository{Repo: r}, seat)
}
func (l landAdapter) TaskLanded(ctx context.Context, r repo.Repo, task string) (bool, error) {
	return l.pipeline.TaskLanded(ctx, repository{Repo: r}, task)
}

type beadsAdapter struct {
	clients map[string]*bd.Client
}

func newBeads(records []repo.Repo, log *logging.Logger) (*beadsAdapter, error) {
	clients := make(map[string]*bd.Client, len(records))
	for _, record := range records {
		client, err := bd.New("", record.Root)
		if err != nil {
			return nil, err
		}
		client.Log = log
		clients[record.Root] = client
	}
	return &beadsAdapter{clients: clients}, nil
}
func (b *beadsAdapter) client(r repo.Repo) (*bd.Client, error) {
	client := b.clients[r.Root]
	if client == nil {
		return nil, fmt.Errorf("repository not configured: %s", r.Name)
	}
	return client, nil
}
func (b *beadsAdapter) Ready(ctx context.Context, r repo.Repo) ([]dispatch.ReadyEntry, error) {
	client, err := b.client(r)
	if err != nil {
		return nil, err
	}
	beads, err := client.Ready(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]dispatch.ReadyEntry, 0, len(beads))
	for _, bead := range beads {
		result = append(result, dispatch.ReadyEntry{Repo: r, Task: bead.ID, Priority: strconv.Itoa(bead.Priority)})
	}
	return result, nil
}
func (b *beadsAdapter) Show(ctx context.Context, r repo.Repo, id string) (dispatch.Spec, error) {
	client, err := b.client(r)
	if err != nil {
		return dispatch.Spec{}, err
	}
	bead, err := client.Show(ctx, id)
	return dispatch.Spec{Title: bead.Title, Description: bead.Description, Design: bead.Design, Acceptance: bead.AcceptanceCriteria}, err
}
func (b *beadsAdapter) Claim(ctx context.Context, r repo.Repo, id string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Claim(ctx, id)
}
func (b *beadsAdapter) Release(ctx context.Context, r repo.Repo, id string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Release(ctx, id)
}
func (b *beadsAdapter) Close(ctx context.Context, r repo.Repo, id, reason string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Close(ctx, id, reason)
}
func (b *beadsAdapter) Comment(ctx context.Context, r repo.Repo, id, text string) error {
	client, err := b.client(r)
	if err != nil {
		return err
	}
	return client.Comment(ctx, id, text)
}
func (b *beadsAdapter) Difficulty(ctx context.Context, r repo.Repo, id string) (string, error) {
	labels, err := b.labels(ctx, r, id)
	if err != nil {
		return "", err
	}
	for _, label := range labels {
		if value, ok := strings.CutPrefix(label, "difficulty:"); ok {
			return value, nil
		}
	}
	return config.DifficultyHigh, nil
}
func (b *beadsAdapter) HumanOnly(ctx context.Context, r repo.Repo, id string) (bool, error) {
	labels, err := b.labels(ctx, r, id)
	if err != nil {
		return false, err
	}
	for _, label := range labels {
		if label == "human" || label == "human-only" {
			return true, nil
		}
	}
	return false, nil
}
func (b *beadsAdapter) InProgress(ctx context.Context, r repo.Repo) ([]string, error) {
	return b.ids(ctx, r, "status:in_progress")
}
func (b *beadsAdapter) OpenEpics(ctx context.Context, r repo.Repo) ([]string, error) {
	return b.ids(ctx, r, "type:epic")
}
func (b *beadsAdapter) EpicChildren(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	return b.ids(ctx, r, "parent:"+id)
}
func (b *beadsAdapter) EpicOpenChildren(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	return b.ids(ctx, r, "parent:"+id+" status:open")
}
func (b *beadsAdapter) DriftFixTasks(ctx context.Context, r repo.Repo) ([]string, error) {
	return b.ids(ctx, r, "label:drift-fix")
}
func (b *beadsAdapter) CancelAll(context.Context) error { return nil }
func (b *beadsAdapter) Query(ctx context.Context, r repo.Repo, query string) ([]bd.Bead, error) {
	client, err := b.client(r)
	if err != nil {
		return nil, err
	}
	return client.Query(ctx, query, true)
}
func (b *beadsAdapter) labels(ctx context.Context, r repo.Repo, id string) ([]string, error) {
	client, err := b.client(r)
	if err != nil {
		return nil, err
	}
	return client.Labels(ctx, id)
}
func (b *beadsAdapter) ids(ctx context.Context, r repo.Repo, query string) ([]string, error) {
	beads, err := b.Query(ctx, r, query)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(beads))
	for _, bead := range beads {
		if bead.Status != "closed" {
			ids = append(ids, bead.ID)
		}
	}
	return ids, nil
}

var _ dispatch.Beads = (*beadsAdapter)(nil)
