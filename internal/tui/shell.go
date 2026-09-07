package tui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

// Screen identifies a read-only shell view.
type Screen string

const (
	ScreenDashboard    Screen = "Dashboard"
	ScreenBeads        Screen = "Beads"
	ScreenSeats        Screen = "Seats"
	ScreenRepositories Screen = "Repositories"
	ScreenEvents       Screen = "Events"
)

var shellScreens = []Screen{ScreenDashboard, ScreenBeads, ScreenSeats, ScreenRepositories, ScreenEvents}

// Shell owns keyboard navigation, help, dimensions, and command hints.
type Shell struct {
	screen  Screen
	width   int
	height  int
	help    bool
	noColor bool
}

// ShellOptions configures a read-only shell.
type ShellOptions struct {
	Width   int
	Height  int
	NoColor bool
}

// ShellAction records a non-mutating shell request.
type ShellAction struct {
	Refresh bool
	Focus   bool
	Quit    bool
}

// NewShell constructs the Dashboard shell at a deterministic default size.
func NewShell(options ShellOptions) Shell {
	if options.Width <= 0 {
		options.Width = 80
	}
	if options.Height <= 0 {
		options.Height = 24
	}
	return Shell{screen: ScreenDashboard, width: options.Width, height: options.Height, noColor: options.NoColor}
}

// Screen returns the active shell screen.
func (s Shell) Screen() Screen { return s.screen }

// Size returns the current render bounds.
func (s Shell) Size() (int, int) { return s.width, s.height }

// NoColor reports whether renderers must suppress ANSI styling.
func (s Shell) NoColor() bool { return s.noColor }

// Update consumes only documented read-only shell keys.
func (s Shell) Update(message tea.Msg) (Shell, ShellAction) {
	switch message := message.(type) {
	case tea.WindowSizeMsg:
		if message.Width > 0 {
			s.width = message.Width
		}
		if message.Height > 0 {
			s.height = message.Height
		}
		return s, ShellAction{Focus: true}
	case tea.KeyMsg:
		switch message.String() {
		case "q", "ctrl+c":
			return s, ShellAction{Quit: true}
		case "r":
			return s, ShellAction{Refresh: true}
		case "?":
			s.help = !s.help
			return s, ShellAction{}
		case "esc":
			s.help = false
			return s, ShellAction{}
		case "tab":
			s.screen = shellScreens[(shellScreenIndex(s.screen)+1)%len(shellScreens)]
			return s, ShellAction{Focus: true}
		case "shift+tab":
			index := shellScreenIndex(s.screen) - 1
			if index < 0 {
				index = len(shellScreens) - 1
			}
			s.screen = shellScreens[index]
			return s, ShellAction{Focus: true}
		case "1", "2", "3", "4", "5":
			s.screen = shellScreens[int(message.String()[0]-'1')]
			return s, ShellAction{Focus: true}
		}
	}
	return s, ShellAction{}
}

// View surrounds body with navigation and exactly the accepted key hints.
func (s Shell) View(body string) string {
	lines := []string{
		fit("1 Dashboard  2 Beads  3 Seats  4 Repositories  5 Events", s.width),
		fit("tab/shift+tab navigate  r refresh  ? help  q/ctrl+c quit", s.width),
	}
	if s.help {
		lines = append(lines,
			fit("Keys: 1-5 select a screen; tab/shift+tab navigate; r refresh; ? help; q quit", s.width),
		)
	}
	if body != "" {
		lines = append(lines, body)
	}
	return trimShellHeight(strings.Join(lines, "\n"), s.height)
}

func shellScreenIndex(screen Screen) int {
	for index, candidate := range shellScreens {
		if candidate == screen {
			return index
		}
	}
	return 0
}

func trimShellHeight(view string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(view, "\n")
	if len(lines) <= height {
		return view
	}
	return strings.Join(lines[:height], "\n")
}

// DashboardLayout owns Dashboard sizing and renderer-input adaptation.
type DashboardLayout struct {
	width   int
	height  int
	noColor bool
}

// NewDashboardLayout constructs a layout from shell-owned dimensions.
func NewDashboardLayout(width, height int, noColor bool) DashboardLayout {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	return DashboardLayout{width: width, height: height, noColor: noColor}
}

// Resize updates renderer bounds and no-color propagation.
func (l *DashboardLayout) Resize(width, height int, noColor bool) {
	if width > 0 {
		l.width = width
	}
	if height > 0 {
		l.height = height
	}
	l.noColor = noColor
}

// Render adapts immutable model state into deterministic Dashboard panels.
func (l DashboardLayout) Render(state ModelState, now time.Time, refresh DashboardRefreshState, refreshError string) string {
	age := time.Duration(0)
	if !state.ChangedAt.IsZero() && now.After(state.ChangedAt) {
		age = now.Sub(state.ChangedAt)
	}
	free, busy := 0, 0
	for _, seat := range state.Snapshot.Seats {
		if seat.Busy {
			busy++
		} else {
			free++
		}
	}
	events := make([]DashboardEvent, 0, len(state.Events))
	for _, event := range state.Events {
		events = append(events, DashboardEvent{Kind: string(event.Event.Kind), Detail: event.Event.Task, Style: EventStyleSuccess})
	}
	notices := make([]DashboardNotice, 0, len(state.Notices))
	for _, notice := range state.Notices {
		switch notice.Kind {
		case transport.StreamNoticeEOF, transport.StreamNoticeUnavailable:
			notices = append(notices, DashboardNotice{Kind: DashboardNoticeReconnect, Detail: string(notice.Kind)})
		case transport.StreamNoticeGap:
			notices = append(notices, DashboardNotice{Kind: DashboardNoticeGap, Detail: fmt.Sprintf("cursor %d to %d", notice.MissingFrom, notice.MissingTo)})
		case transport.StreamNoticeMiss:
			notices = append(notices, DashboardNotice{Kind: DashboardNoticeMiss, Detail: "events unavailable"})
		}
	}
	panels := []string{
		RenderDashboardHeader(DashboardHeaderInput{
			Connection: dashboardConnection(state.Connection), Freshness: dashboardFreshness(state.Freshness),
			SnapshotAge: age, Generation: state.Generation, Running: state.Snapshot.Runtime.Running,
			Draining: state.Snapshot.Runtime.Draining, Refresh: refresh, Error: refreshError,
		}, l.width),
		RenderDashboardSummary(DashboardSummaryInput{Repositories: len(state.Snapshot.Repositories), Sessions: len(state.Snapshot.Sessions), StatusCounts: state.Snapshot.StatusCounts, FreeSeats: free, BusySeats: busy}, l.width),
		RenderDashboardSeats(DashboardSeatsInput{Seats: state.Snapshot.Seats}, l.width),
		RenderDashboardSessions(DashboardSessionsInput{Sessions: state.Snapshot.Sessions}, l.width),
		RenderDashboardEvents(DashboardEventsInput{Events: events, Notices: notices, Width: l.width, Color: !l.noColor}),
	}
	return trimShellHeight(strings.Join(panels, "\n\n"), l.height)
}

func dashboardConnection(connection ConnectionState) DashboardConnectionState {
	switch connection {
	case ConnectionOnline:
		return DashboardConnectionConnected
	case ConnectionOffline:
		return DashboardConnectionOffline
	default:
		return DashboardConnectionLoading
	}
}

func dashboardFreshness(freshness SnapshotFreshness) DashboardFreshness {
	switch freshness {
	case SnapshotFresh:
		return DashboardFreshnessFresh
	case SnapshotStale:
		return DashboardFreshnessStale
	default:
		return DashboardFreshnessLoading
	}
}

// ProgramOptions supplies the composed TUI runtime dependencies and test seams.
type ProgramOptions struct {
	API            transport.DaemonAPI
	Stream         transport.EventStream
	Now            func() time.Time
	Cadence        time.Duration
	InitialBackoff time.Duration
	MaximumBackoff time.Duration
	NewTimer       TimerFactory
	Width          int
	Height         int
	NoColor        bool
}

// Program composes the root model, coordinator, transport inputs, shell, and Dashboard.
type Program struct {
	model       Model
	shell       Shell
	layout      DashboardLayout
	coordinator *RefreshCoordinator
	now         func() time.Time
	messages    chan tea.Msg

	mu           sync.Mutex
	context      context.Context
	cancel       context.CancelFunc
	started      bool
	generation   uint64
	refresh      DashboardRefreshState
	refreshError string
}

// NewProgram constructs an inert composed runtime. Init or Run starts I/O.
func NewProgram(options ProgramOptions) *Program {
	if options.Now == nil {
		options.Now = time.Now
	}
	shell := NewShell(ShellOptions{Width: options.Width, Height: options.Height, NoColor: options.NoColor})
	width, height := shell.Size()
	program := &Program{
		model: NewModel(ModelOptions{Now: options.Now}), shell: shell,
		layout: NewDashboardLayout(width, height, shell.NoColor()), now: options.Now,
		messages: make(chan tea.Msg, 64), refresh: DashboardRefreshIdle,
	}
	program.coordinator = NewRefreshCoordinator(options.API, options.Stream, RefreshOptions{
		Cadence: options.Cadence, InitialBackoff: options.InitialBackoff, MaximumBackoff: options.MaximumBackoff, NewTimer: options.NewTimer,
		ResultSink: program.deliverResult, NoticeSink: program.deliverNotice,
	})
	return program
}

// Init starts transport ownership and awaits coordinator messages.
func (p *Program) Init() tea.Cmd {
	p.start(context.Background())
	return p.nextMessage()
}

// Run launches Program through Bubble Tea with explicit I/O ownership.
func (p *Program) Run(ctx context.Context, input io.Reader, output io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	p.start(ctx)
	defer p.Stop()
	_, err := tea.NewProgram(p, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(output)).Run()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

// Stop cancels coordinator-owned request and stream connections.
func (p *Program) Stop() {
	p.mu.Lock()
	cancel := p.cancel
	p.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	p.coordinator.Cancel()
}

// State returns an immutable model snapshot for composed regression coverage.
func (p *Program) State() ModelState { return p.model.State() }

// Update serializes shell commands and typed coordinator lifecycle messages.
func (p *Program) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	var action ShellAction
	p.shell, action = p.shell.Update(message)
	width, height := p.shell.Size()
	p.layout.Resize(width, height, p.shell.NoColor())
	if action.Quit {
		p.Stop()
		return p, tea.Quit
	}
	if action.Refresh {
		p.refresh = DashboardRefreshRefreshing
		p.refreshError = ""
		p.coordinator.Refresh()
	}
	if action.Focus {
		p.coordinator.Focus()
	}

	switch value := message.(type) {
	case RefreshResult:
		if value.Generation < p.generation {
			break
		}
		p.generation = value.Generation
		if value.Err != nil {
			p.refresh = DashboardRefreshError
			p.refreshError = value.Err.Error()
		} else {
			p.refresh = DashboardRefreshIdle
			p.refreshError = ""
		}
		p.updateModel(SnapshotMsg{Snapshot: value.Snapshot, Error: value.Err})
	case RefreshNotice:
		if value.Generation >= p.generation {
			if value.Kind == RefreshEvent {
				p.updateModel(StreamEventMsg{Event: value.Event})
			} else {
				p.updateModel(StreamNoticeMsg{Notice: value.Stream})
			}
		}
	case SnapshotMsg, transport.Snapshot, StreamEventMsg, transport.Event, StreamNoticeMsg, transport.StreamNotice, SelectionMsg:
		p.updateModel(value)
	}
	if p.isStarted() {
		return p, p.nextMessage()
	}
	return p, nil
}

// View renders the selected read-only screen.
func (p *Program) View() string {
	if p.shell.Screen() != ScreenDashboard {
		return p.shell.View(fit(string(p.shell.Screen())+"\nread-only panel not implemented", p.layout.width))
	}
	return p.shell.View(p.layout.Render(p.model.State(), p.now(), p.refresh, p.refreshError))
}

func (p *Program) updateModel(message tea.Msg) {
	next, _ := p.model.Update(message)
	p.model = next.(Model)
}

func (p *Program) start(ctx context.Context) {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return
	}
	p.context, p.cancel = context.WithCancel(ctx)
	p.started = true
	run := p.context
	p.mu.Unlock()
	p.coordinator.Start(run)
}

func (p *Program) isStarted() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.started
}

func (p *Program) nextMessage() tea.Cmd {
	return func() tea.Msg {
		p.mu.Lock()
		ctx := p.context
		p.mu.Unlock()
		if ctx == nil {
			return nil
		}
		select {
		case message := <-p.messages:
			return message
		case <-ctx.Done():
			return tea.Quit()
		}
	}
}

func (p *Program) deliverResult(result RefreshResult) { p.deliver(result) }
func (p *Program) deliverNotice(notice RefreshNotice) { p.deliver(notice) }

func (p *Program) deliver(message tea.Msg) {
	p.mu.Lock()
	ctx := p.context
	p.mu.Unlock()
	if ctx == nil || ctx.Err() != nil {
		return
	}
	select {
	case p.messages <- message:
	case <-ctx.Done():
	}
}

var _ tea.Model = (*Program)(nil)
