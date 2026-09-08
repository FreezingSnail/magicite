package tui

import (
	"github.com/charmbracelet/bubbletea"

	"github.com/FreezingSnail/magicite/internal/wire"
)

const cockpitEventCapacity = 256

// RegisterTabs installs the interactive cockpit views and action flows into
// registry. The returned registry is the single runtime composition point for
// inventory tabs; Dashboard remains owned by the runtime shell.
func RegisterTabs(registry *TabRegistry, api DaemonAPI) *TabRegistry {
	var controls ControlAPI
	if candidate, ok := api.(ControlAPI); ok {
		controls = candidate
	}
	views := map[Tab]TabView{}
	if registry != nil && registry.view(TabDashboard) != nil {
		views[TabDashboard] = registry.view(TabDashboard)
	}
	beads := NewBeadsView()
	views[TabBeads] = &cockpitBeadsView{
		beads:    beads,
		dispatch: NewDispatchFlow(api),
		controls: NewControlFlow(controls),
	}
	views[TabSeats] = NewSeatsView()
	views[TabRepositories] = NewReposView()
	views[TabEvents] = NewEventsView(cockpitEventCapacity)
	assembled := NewTabRegistry(views)
	return &assembled
}

// cockpitBeadsView joins the Beads detail pane with its contextual action
// flows without making either flow a separate navigable tab.
type cockpitBeadsView struct {
	beads    *BeadsView
	dispatch *DispatchFlow
	controls *ControlFlow
}

func (*cockpitBeadsView) Title() string { return "Beads" }

func (view *cockpitBeadsView) Update(message tea.Msg) (TabView, tea.Cmd) {
	if view.dispatch.Action.Phase != PhaseIdle {
		return view, view.dispatch.Update(view.beads, message)
	}
	if view.controls.Action.Phase != PhaseIdle {
		return view, view.controls.Update(view.beads, message)
	}
	if key, ok := message.(tea.KeyMsg); ok {
		switch key.String() {
		case "d":
			if view.dispatch.OpenSelected(view.beads) {
				return view, nil
			}
			return view, view.controls.Update(view.beads, message)
		case "s", "h", "r":
			return view, view.controls.Update(view.beads, message)
		}
	}
	return view, updateCockpitBeads(view.beads, message)
}

func updateCockpitBeads(beads *BeadsView, message tea.Msg) tea.Cmd {
	_, command := beads.Update(message)
	return command
}

func (view *cockpitBeadsView) Snapshot(snapshot wire.SnapshotResult) TabView {
	view.beads.Snapshot(snapshot)
	view.controls.Snapshot(snapshot)
	return view
}

func (view *cockpitBeadsView) Hints() []Hint {
	if view.dispatch.Action.Phase != PhaseIdle {
		return view.dispatch.Hints(view.beads)
	}
	if view.controls.Action.Phase != PhaseIdle {
		return view.controls.Hints(view.beads)
	}
	hints := append([]Hint{}, view.beads.Hints()...)
	hints = append(hints, view.dispatch.Hints(view.beads)...)
	return append(hints, view.controls.Hints(view.beads)...)
}

func (view *cockpitBeadsView) View(width, height int) string {
	body := view.beads.View(width, height)
	if view.dispatch.Action.Phase != PhaseIdle {
		return view.dispatch.View(width)
	}
	if view.controls.Action.Phase != PhaseIdle {
		return view.controls.View(width)
	}
	return body
}

func cockpitEvents(registry *TabRegistry) *EventsView {
	if registry == nil {
		return nil
	}
	view, _ := registry.view(TabEvents).(*EventsView)
	return view
}
