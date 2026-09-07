package tui

import (
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/tui/transport"
)

func TestRenderDashboardSeatsShowsEverySeatInStableOrder(t *testing.T) {
	input := DashboardSeatsInput{Seats: []transport.Seat{
		{Name: "shiva", Role: "implementer", Busy: true, Task: "magicite-2", Repo: "magicite"},
		{Name: "odin", Role: "reviewer"},
		{Name: "ifrit", Role: "implementer"},
	}}
	original := append([]transport.Seat(nil), input.Seats...)

	got := RenderDashboardSeats(input, 80)
	want := "Seats\nifrit [implementer] idle\nshiva [implementer] assigned magicite-2 @magicite\nodin [reviewer] idle"
	if got != want {
		t.Fatalf("RenderDashboardSeats() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(input.Seats, original) {
		t.Fatalf("RenderDashboardSeats() mutated input: %#v", input.Seats)
	}
}

func TestRenderDashboardSeatsHandlesMissingValuesAndNarrowWidth(t *testing.T) {
	got := RenderDashboardSeats(DashboardSeatsInput{Seats: []transport.Seat{{Busy: true}}}, 12)
	want := "Seats\nunnamed [..."
	if got != want {
		t.Fatalf("RenderDashboardSeats() = %q, want %q", got, want)
	}
}

func TestRenderDashboardSessionsShowsOrderedActiveMetadata(t *testing.T) {
	input := DashboardSessionsInput{Sessions: []transport.Session{
		{Handle: "beta", Phase: "review", Backend: "kiro", Model: "gpt-5.6-terra", UptimeSeconds: 60},
		{Handle: "alpha", Phase: "implementing", Backend: "opencode", Model: "model", UptimeSeconds: 60},
		{Handle: "long", Phase: "tool", Backend: "kiro", Model: "gpt-5.6-luna", UptimeSeconds: 3601},
	}}
	original := append([]transport.Session(nil), input.Sessions...)

	got := RenderDashboardSessions(input, 80)
	want := "Sessions\nlong [tool] kiro/gpt-5.6-luna 1h0m1s\nalpha [implementing] opencode/model 1m0s\nbeta [review] kiro/gpt-5.6-terra 1m0s"
	if got != want {
		t.Fatalf("RenderDashboardSessions() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(input.Sessions, original) {
		t.Fatalf("RenderDashboardSessions() mutated input: %#v", input.Sessions)
	}
}

func TestRenderDashboardSessionsHandlesMissingValuesAndNarrowWidth(t *testing.T) {
	got := RenderDashboardSessions(DashboardSessionsInput{Sessions: []transport.Session{{UptimeSeconds: -1}}}, 12)
	want := "Sessions\nunnamed [..."
	if got != want {
		t.Fatalf("RenderDashboardSessions() = %q, want %q", got, want)
	}
}
