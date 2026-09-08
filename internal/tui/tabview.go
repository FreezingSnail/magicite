package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// Tab reuses the shell's navigation identity for a tab view.
type Tab = Screen

const (
	TabDashboard    Tab = ScreenDashboard
	TabBeads        Tab = ScreenBeads
	TabSeats        Tab = ScreenSeats
	TabRepositories Tab = ScreenRepositories
	TabEvents       Tab = ScreenEvents

	Dashboard    = TabDashboard
	Beads        = TabBeads
	Seats        = TabSeats
	Repositories = TabRepositories
	Events       = TabEvents
)

var tabOrder = []Tab{TabDashboard, TabBeads, TabSeats, TabRepositories, TabEvents}

// Hint describes a currently available tab command.
type Hint struct {
	Key     string
	Label   string
	Enabled bool
}

// TabView owns one tab's local state and rendering.
type TabView interface {
	Title() string
	Update(tea.Msg) (TabView, tea.Cmd)
	View(width, height int) string
	Hints() []Hint
	Snapshot(wire.SnapshotResult) TabView
}

// EmptyTabPlaceholder renders when navigation reaches an unregistered tab.
const EmptyTabPlaceholder = "empty"

// TabRegistry holds tab views by the shell's fixed navigation identities.
// Views are always traversed in tab order, never map order.
type TabRegistry struct {
	views map[Tab]TabView
}

// NewTabRegistry copies views into a registry. Nil entries are unregistered.
func NewTabRegistry(views map[Tab]TabView) TabRegistry {
	registry := TabRegistry{views: make(map[Tab]TabView, len(tabOrder))}
	for _, tab := range tabOrder {
		if view := views[tab]; view != nil {
			registry.views[tab] = view
		}
	}
	return registry
}

// Update sends a message only to the focused view.
func (registry TabRegistry) Update(focused Tab, message tea.Msg) (TabRegistry, tea.Cmd) {
	view := registry.view(focused)
	if view == nil {
		return registry, nil
	}
	next, command := view.Update(message)
	return registry.with(focused, next), command
}

// Snapshot replaces each registered view with its snapshot-derived successor.
func (registry TabRegistry) Snapshot(snapshot wire.SnapshotResult) TabRegistry {
	for _, tab := range tabOrder {
		view := registry.view(tab)
		if view != nil {
			registry = registry.with(tab, view.Snapshot(snapshot))
		}
	}
	return registry
}

// View renders the focused view or an explicit empty placeholder.
func (registry TabRegistry) View(focused Tab, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	view := registry.view(focused)
	if view == nil {
		return EmptyTabPlaceholder
	}
	return view.View(width, height)
}

// Hints returns only enabled hints for the focused view.
func (registry TabRegistry) Hints(focused Tab) []Hint {
	view := registry.view(focused)
	if view == nil {
		return nil
	}
	return effectiveHints(view.Hints())
}

// AllHints concatenates enabled hints from registered views in tab order.
func (registry TabRegistry) AllHints() []Hint {
	var hints []Hint
	for _, tab := range tabOrder {
		hints = append(hints, registry.Hints(tab)...)
	}
	return hints
}

func (registry TabRegistry) view(tab Tab) TabView {
	return registry.views[tab]
}

func (registry TabRegistry) with(tab Tab, view TabView) TabRegistry {
	if view == nil {
		return registry
	}
	views := make(map[Tab]TabView, len(registry.views)+1)
	for _, candidate := range tabOrder {
		if current := registry.view(candidate); current != nil {
			views[candidate] = current
		}
	}
	views[tab] = view
	registry.views = views
	return registry
}

func effectiveHints(hints []Hint) []Hint {
	effective := make([]Hint, 0, len(hints))
	for _, hint := range hints {
		if hint.Enabled {
			effective = append(effective, hint)
		}
	}
	return effective
}

// KeepSelection resolves a stable row key against the current display order.
func KeepSelection(keys []string, previous Selection) Selection {
	if len(keys) == 0 {
		return Selection{Index: -1}
	}
	if previous.Key != "" {
		for index, key := range keys {
			if key == previous.Key {
				return Selection{Key: key, Index: index}
			}
		}
	}
	index := previous.Index
	if index < 0 {
		index = 0
	}
	if index >= len(keys) {
		index = len(keys) - 1
	}
	return Selection{Key: keys[index], Index: index}
}
