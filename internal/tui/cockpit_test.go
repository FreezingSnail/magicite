package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestRegisterTabsAssemblesRealViewsAndPreservesSelection(t *testing.T) {
	registry := RegisterTabs(nil, &cockpitAPI{})
	snapshot := cockpitFixture(1)
	registry = registrySnapshot(registry, snapshot)

	for _, test := range []struct {
		tab  Tab
		want string
	}{
		{TabBeads, "magicite-400.10"},
		{TabSeats, "ifrit"},
		{TabRepositories, "magicite"},
		{TabEvents, "Events  follow:on"},
	} {
		if got := registry.View(test.tab, 100, 20); !strings.Contains(got, test.want) {
			t.Fatalf("%s render missing %q: %q", test.tab, test.want, got)
		}
	}

	beads := registry.view(TabBeads).(*cockpitBeadsView).beads
	seats := registry.view(TabSeats).(*SeatsView)
	repos := registry.view(TabRepositories).(*ReposView)
	events := registry.view(TabEvents).(*EventsView)
	_, _ = registry.Update(TabBeads, tea.KeyMsg{Type: tea.KeyDown})
	seats.Update(tea.KeyMsg{Type: tea.KeyDown})
	repos.Update(tea.KeyMsg{Type: tea.KeyDown})
	events.Append(wire.Event{Seq: 1, Kind: wire.KindComplete, Fields: map[string]string{"summary": "first"}})
	events.Append(wire.Event{Seq: 2, Kind: wire.KindComplete, Fields: map[string]string{"summary": "second"}})
	events.Update(tea.KeyMsg{Type: tea.KeyUp})

	replacement := cockpitFixture(2)
	replacement.Beads = []wire.BeadResult{replacement.Beads[1], replacement.Beads[0]}
	replacement.Seats = []wire.SeatResult{replacement.Seats[1], replacement.Seats[0]}
	replacement.Repositories = []wire.RepoResult{replacement.Repositories[1], replacement.Repositories[0]}
	registry = registrySnapshot(registry, replacement)
	if beads.selection.Key != "magicite-400.11" || seats.selection.Key != "odin" || repos.selection.Key != "tools" || events.selection.Key != "event:1" {
		t.Fatalf("selections after replacement = beads:%#v seats:%#v repos:%#v events:%#v", beads.selection, seats.selection, repos.selection, events.selection)
	}

	_, _ = registry.Update(TabBeads, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	_, _ = registry.Update(TabBeads, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("400.11")})
	if beads.selection.Key != "magicite-400.11" {
		t.Fatalf("bead filter lost selection: %#v", beads.selection)
	}
	for _, tab := range []Tab{TabBeads, TabSeats, TabRepositories, TabEvents} {
		if got := registry.View(tab, 23, 7); got == "" || strings.Contains(got, "\x1b[") {
			t.Fatalf("%s narrow render = %q", tab, got)
		}
	}
}

func TestCockpitGoldensAreTextAccessible(t *testing.T) {
	registry := RegisterTabs(nil, &cockpitAPI{})
	registry = registrySnapshot(registry, cockpitFixture(1))
	events := cockpitEvents(registry)
	events.Append(wire.Event{Seq: 1, Time: time.Date(2026, time.September, 8, 1, 2, 3, 0, time.UTC), Level: "info", Kind: wire.KindComplete, Repo: "magicite", Task: "magicite-400.10", Seat: "ifrit", Fields: map[string]string{"summary": "worker \x1b[31m界 complete"}})
	for _, test := range []struct {
		tab  Tab
		name string
	}{
		{TabBeads, "beads"},
		{TabSeats, "seats"},
		{TabRepositories, "repositories"},
		{TabEvents, "events"},
	} {
		for _, width := range []int{100, 28} {
			name := test.name + map[bool]string{true: "-wide.golden", false: "-narrow.golden"}[width == 100]
			got := registry.View(test.tab, width, 12)
			if strings.Contains(got, "\x1b[") {
				t.Fatalf("%s uses ANSI: %q", name, got)
			}
			assertCockpitGolden(t, name, got)
		}
	}
}

func TestCockpitExplicitStatesRemainReadable(t *testing.T) {
	registry := RegisterTabs(nil, &cockpitAPI{})
	if got := registry.View(TabBeads, 80, 8); !strings.Contains(got, "No beads in snapshot") {
		t.Fatalf("empty beads = %q", got)
	}
	if got := registry.View(TabSeats, 80, 8); !strings.Contains(got, "No seats configured") {
		t.Fatalf("empty seats = %q", got)
	}
	if got := registry.View(TabRepositories, 80, 8); !strings.Contains(got, "No repositories configured") {
		t.Fatalf("empty repositories = %q", got)
	}
	snapshot := cockpitFixture(1)
	snapshot.RepositoryErrors = []wire.RepositoryError{{Repository: "magicite", Error: "\x1b[31mread failed"}}
	registry = registrySnapshot(registry, snapshot)
	if got := registry.View(TabRepositories, 100, 8); !strings.Contains(got, "error (read failed)") || strings.Contains(got, "\x1b[") {
		t.Fatalf("repository error = %q", got)
	}

	clock := time.Date(2026, time.September, 8, 2, 0, 0, 0, time.UTC)
	layout := NewDashboardLayout(100, 20, true)
	model := NewModel(ModelOptions{Now: func() time.Time { return clock }})
	if got := layout.Render(model.State(), clock, DashboardRefreshIdle, ""); !strings.Contains(got, "connection: loading") {
		t.Fatalf("loading dashboard = %q", got)
	}
	for _, test := range []struct {
		message tea.Msg
		want    string
	}{
		{SnapshotMsg{Snapshot: transport.Snapshot{ModelVersion: wire.Schema, Generation: 1, Fresh: true}}, "connection: connected"},
		{SnapshotMsg{Snapshot: transport.Snapshot{ModelVersion: wire.Schema, Generation: 2, Stale: true}}, "snapshot: stale"},
		{SnapshotMsg{Error: &transport.Error{Code: transport.ErrorUnavailable}}, "connection: offline"},
	} {
		next, _ := model.Update(test.message)
		model = next.(Model)
		if got := layout.Render(model.State(), clock, DashboardRefreshIdle, ""); !strings.Contains(got, test.want) {
			t.Fatalf("state %T missing %q: %q", test.message, test.want, got)
		}
	}
}

func TestCockpitSocketSmokeSnapshotEventHardStopAndResnapshot(t *testing.T) {
	server := newCockpitSocket(t, cockpitFixture(1))
	defer server.Close()
	api := transport.NewRPC(client.New(client.Options{Socket: server.socket, Timeout: time.Second}))
	program := NewProgram(ProgramOptions{
		API: api, Actions: api, Stream: transport.NewEventStream(client.New(client.Options{Socket: server.socket, Timeout: time.Second})),
		Now: func() time.Time { return time.Date(2026, time.September, 8, 3, 0, 0, 0, time.UTC) }, Cadence: -1, Width: 100, Height: 24, NoColor: true,
	})
	defer program.Stop()

	command := program.Init()
	server.Await(t, "snapshot")
	program = programUpdate(t, program, command())
	server.Await(t, "subscribe")
	server.ReleaseEvent()
	command = program.nextMessage()
	program = programUpdate(t, program, command())
	program = programUpdate(t, program, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("2")})
	program = programUpdate(t, program, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	_, command = program.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	if command == nil {
		t.Fatal("hard-stop confirmation returned no command")
	}
	program = programUpdate(t, program, command())
	server.Await(t, "stop")
	if got := server.hardStops.Load(); got != 1 {
		t.Fatalf("hard stops = %d, want 1", got)
	}
	server.Await(t, "snapshot")
	program = programUpdate(t, program, program.nextMessage()())
	program = programUpdate(t, program, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	if got := program.View(); !strings.Contains(got, "streame") || !strings.Contains(got, "Events") {
		t.Fatalf("events view = %q", got)
	}
}

func registrySnapshot(registry *TabRegistry, snapshot wire.SnapshotResult) *TabRegistry {
	next := registry.Snapshot(snapshot)
	return &next
}

func assertCockpitGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", "cockpit", name)
	if os.Getenv("UPDATE") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != got+"\n" {
		t.Fatalf("golden %s mismatch\nwant:\n%s\ngot:\n%s", name, want, got)
	}
}

type cockpitAPI struct{}

func (*cockpitAPI) Dispatch(context.Context, wire.DispatchParams) (wire.DispatchResult, error) {
	return wire.DispatchResult{}, nil
}
func (*cockpitAPI) Start(context.Context) (wire.StatusResult, error) { return wire.StatusResult{}, nil }
func (*cockpitAPI) Stop(context.Context, wire.StopParams) (wire.StopResult, error) {
	return wire.StopResult{}, nil
}
func (*cockpitAPI) Review(context.Context, wire.ReviewParams) (wire.ReviewResult, error) {
	return wire.ReviewResult{}, nil
}

func cockpitFixture(generation uint64) wire.SnapshotResult {
	return wire.SnapshotResult{
		ModelVersion: wire.Schema, Generation: generation, Cursor: 1, Fresh: true,
		Runtime:      wire.StatusResult{Running: true, Sessions: []wire.SessionResult{}},
		Repositories: []wire.RepoResult{{Name: "magicite", Prefix: "magicite\x1b[31m界", Branch: "main"}, {Name: "tools", Prefix: "tools", Branch: "dev"}},
		Seats:        []wire.SeatResult{{Name: "ifrit", Role: "implementer界", Repo: "magicite", Task: "magicite-400.10", Busy: true}, {Name: "odin", Role: "reviewer"}},
		Sessions:     []wire.SessionResult{{Seat: "ifrit", Handle: "ifrit", Backend: "kiro", Model: "terra", UptimeSeconds: 65}},
		Beads: []wire.BeadResult{
			{ID: "magicite-400.10", Repo: "magicite", Title: "Cockpit \x1b[31m界", Status: "open", Priority: 1, IssueType: "task", Staged: true, Dispatch: wire.Eligibility{Eligible: true}},
			{ID: "magicite-400.11", Repo: "tools", Title: "Wide 界界界 fixture", Status: "deferred", Priority: 2, IssueType: "task"},
		},
		StatusCounts: []wire.StatusCount{{Status: "open", Count: 1}}, RepositoryErrors: []wire.RepositoryError{},
	}
}

type cockpitSocket struct {
	socket       string
	link         string
	listener     net.Listener
	done         chan struct{}
	releaseEvent chan struct{}
	requests     chan struct{}
	observed     map[string]int
	mu           sync.Mutex
	hardStops    atomic.Int32
	once         sync.Once
	eventOnce    sync.Once
	wait         sync.WaitGroup
	snapshot     wire.SnapshotResult
}

var cockpitSocketSequence atomic.Uint64

func newCockpitSocket(t *testing.T, snapshot wire.SnapshotResult) *cockpitSocket {
	t.Helper()
	link := fmt.Sprintf(".tui-cockpit-%d-%d", os.Getpid(), cockpitSocketSequence.Add(1))
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	server := &cockpitSocket{socket: filepath.Join(link, "s"), link: link, done: make(chan struct{}), releaseEvent: make(chan struct{}), requests: make(chan struct{}, 16), observed: make(map[string]int), snapshot: snapshot}
	listener, err := net.Listen("unix", server.socket)
	if err != nil {
		t.Fatal(err)
	}
	server.listener = listener
	server.wait.Add(1)
	go func() {
		defer server.wait.Done()
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			server.wait.Add(1)
			go func() {
				defer server.wait.Done()
				defer conn.Close()
				server.handle(conn)
			}()
		}
	}()
	return server
}

func (server *cockpitSocket) handle(conn net.Conn) {
	request, err := wire.NewDecoder(conn).Request()
	if err != nil {
		return
	}
	server.mu.Lock()
	server.observed[request.Command]++
	server.mu.Unlock()
	server.requests <- struct{}{}
	switch request.Command {
	case "snapshot":
		payload, _ := json.Marshal(server.snapshot)
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: payload})
	case "metrics":
		payload, _ := json.Marshal(wire.MetricsResult{Lifecycle: []wire.MetricsCount{}, Land: []wire.MetricsCount{}, Roles: []wire.RoleDuration{}, Queue: []wire.RepoQueueDepth{}})
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: payload})
	case "subscribe":
		<-server.releaseEvent
		_ = wire.NewEncoder(conn).Encode(wire.Event{Schema: wire.Schema, Seq: 2, Kind: wire.KindComplete, Fields: map[string]string{"summary": "streamed event"}})
		<-server.done
	case "stop":
		var params wire.StopParams
		_ = json.Unmarshal(request.Params, &params)
		if params.Hard {
			server.hardStops.Add(1)
		}
		payload, _ := json.Marshal(wire.StopResult{Mode: "hard", Sessions: 0})
		_ = wire.NewEncoder(conn).Encode(wire.Response{Schema: wire.Schema, ID: request.ID, Result: payload})
	}
}

func (server *cockpitSocket) Await(t *testing.T, command string) {
	t.Helper()
	for {
		server.mu.Lock()
		if server.observed[command] > 0 {
			server.observed[command]--
			server.mu.Unlock()
			return
		}
		server.mu.Unlock()
		<-server.requests
	}
}

func (server *cockpitSocket) ReleaseEvent() {
	server.eventOnce.Do(func() { close(server.releaseEvent) })
}

func (server *cockpitSocket) Close() {
	server.once.Do(func() {
		server.ReleaseEvent()
		close(server.done)
		_ = server.listener.Close()
		server.wait.Wait()
		_ = os.Remove(server.link)
	})
}
