package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

// DashboardSeatsInput is the daemon-owned seat data shown on the Dashboard.
type DashboardSeatsInput struct {
	Seats []transport.Seat
}

// RenderDashboardSeats renders every configured seat without reading UI state.
func RenderDashboardSeats(input DashboardSeatsInput, width int) string {
	seats := append([]transport.Seat(nil), input.Seats...)
	sort.SliceStable(seats, func(left, right int) bool {
		if seats[left].Role != seats[right].Role {
			return seats[left].Role < seats[right].Role
		}
		return seats[left].Name < seats[right].Name
	})

	lines := make([]string, 0, len(seats)+1)
	lines = append(lines, "Seats")
	for _, seat := range seats {
		assignment := "idle"
		if seat.Busy {
			assignment = "assigned"
			if seat.Task != "" {
				assignment += " " + seat.Task
			}
			if seat.Repo != "" {
				assignment += " @" + seat.Repo
			}
		}
		lines = append(lines, truncateDashboardLine(fmt.Sprintf("%s [%s] %s", dashboardValue(seat.Name, "unnamed"), dashboardValue(seat.Role, "unassigned"), assignment), width))
	}
	return strings.Join(lines, "\n")
}

func dashboardValue(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func truncateDashboardLine(line string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(line) <= width {
		return line
	}
	if width <= 3 {
		return line[:width]
	}
	return line[:width-3] + "..."
}
