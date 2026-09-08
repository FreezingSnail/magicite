package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestShellKeysMatchHintsAndResize(t *testing.T) {
	shell := NewShell(ShellOptions{Width: 80, Height: 24, NoColor: true})
	for _, test := range []struct {
		key    string
		screen Screen
		want   ShellAction
	}{
		{"tab", ScreenBeads, ShellAction{Focus: true}},
		{"shift+tab", ScreenDashboard, ShellAction{Focus: true}},
		{"5", ScreenEvents, ShellAction{Focus: true}},
		{"r", ScreenEvents, ShellAction{Refresh: true}},
		{"?", ScreenEvents, ShellAction{}},
		{"q", ScreenEvents, ShellAction{Quit: true}},
	} {
		message := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(test.key)}
		switch test.key {
		case "tab":
			message = tea.KeyMsg{Type: tea.KeyTab}
		case "shift+tab":
			message = tea.KeyMsg{Type: tea.KeyShiftTab}
		}
		var action ShellAction
		shell, action = shell.Update(message)
		if shell.Screen() != test.screen || action != test.want {
			t.Fatalf("key %q = (%q, %#v), want (%q, %#v)", test.key, shell.Screen(), action, test.screen, test.want)
		}
	}
	view := shell.View("body")
	for _, hint := range []string{"1 Dashboard", "tab/shift+tab", "r refresh", "? help", "q/ctrl+c quit", "Keys: 1-5"} {
		if !strings.Contains(view, hint) {
			t.Fatalf("view %q missing %q", view, hint)
		}
	}
	shell, action := shell.Update(tea.WindowSizeMsg{Width: 31, Height: 9})
	if action != (ShellAction{Focus: true}) || shell.NoColor() != true {
		t.Fatalf("resize action/color = %#v/%t", action, shell.NoColor())
	}
	if width, height := shell.Size(); width != 31 || height != 9 {
		t.Fatalf("size = %dx%d", width, height)
	}
}

func TestDashboardLayoutNoColorAndDeterministicSizing(t *testing.T) {
	layout := NewDashboardLayout(80, 24, true)
	state := NewModel().State()
	first := layout.Render(state, state.ChangedAt, DashboardRefreshIdle, "")
	layout.Resize(18, 4, true)
	second := layout.Render(state, state.ChangedAt, DashboardRefreshIdle, "")
	if strings.Contains(second, "\x1b[") {
		t.Fatalf("no-color render has ANSI: %q", second)
	}
	if lines := strings.Count(second, "\n") + 1; lines > 4 {
		t.Fatalf("height = %d lines: %q", lines, second)
	}
	if first == second {
		t.Fatal("resize did not affect render")
	}
}

func TestDashboardLayoutComposesMetricsPane(t *testing.T) {
	layout := NewDashboardLayout(80, 48, true)
	state := NewModel().State()
	state.Metrics = MetricsState{HasLastGood: true, LastGood: transport.Metrics{Lifecycle: []wire.MetricsCount{}, Land: []wire.MetricsCount{}, Roles: []wire.RoleDuration{}, Queue: []wire.RepoQueueDepth{}}}
	if view := layout.Render(state, time.Time{}, DashboardRefreshIdle, ""); !strings.Contains(view, "Metrics\n") || !strings.Contains(view, "queue: total 0; none") {
		t.Fatalf("Dashboard metrics missing: %q", view)
	}
	state.Metrics = MetricsState{Unsupported: true}
	if view := layout.Render(state, time.Time{}, DashboardRefreshIdle, ""); strings.Contains(view, "Metrics") {
		t.Fatalf("unsupported metrics rendered: %q", view)
	}
}
