package server

import (
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestNormalizeSnapshotOrdersAndPreservesInput(t *testing.T) {
	input := snapshotFixture()
	wantInput := snapshotFixture()

	got := normalizeSnapshot(input)

	if !reflect.DeepEqual(input, wantInput) {
		t.Errorf("normalizeSnapshot mutated input:\n got %#v\nwant %#v", input, wantInput)
	}
	assertRepositoryNames(t, got.Repositories, "alpha", "alpha", "beta")
	if got.Repositories[0].Path != "/a" || got.Repositories[1].Path != "/z" {
		t.Errorf("repository ties = %#v, want /a before /z", got.Repositories)
	}
	assertSeatNames(t, got.Seats, "concierge/z", "implementer/a", "implementer/b")
	assertSessionHandles(t, got.Sessions, "a", "b", "z")
	assertSessionHandles(t, got.Runtime.Sessions, "a", "b", "z")
	assertBeadIDs(t, got.Beads, "alpha-1", "alpha-2", "beta-1")
	assertStatusCounts(t, got.StatusCounts, "closed", "open", "open")
	if got.StatusCounts[1].Count != 1 || got.StatusCounts[2].Count != 2 {
		t.Errorf("status count ties = %#v, want counts 1 then 2", got.StatusCounts)
	}
	assertErrorRepositories(t, got.RepositoryErrors, "alpha", "alpha", "beta")
	if got.RepositoryErrors[0].Error != "a" || got.RepositoryErrors[1].Error != "z" {
		t.Errorf("error ties = %#v, want a before z", got.RepositoryErrors)
	}
	if !reflect.DeepEqual(got.Beads[0].Labels, []string{"second", "first"}) {
		t.Errorf("labels = %#v, want raw order", got.Beads[0].Labels)
	}
	if !reflect.DeepEqual(got.Beads[0].Dependencies, []wire.DependencyResult{{ID: "second"}, {ID: "first"}}) {
		t.Errorf("dependencies = %#v, want raw order", got.Beads[0].Dependencies)
	}
}

func TestNormalizeSnapshotReturnsNonnilIndependentCollections(t *testing.T) {
	empty := normalizeSnapshot(wire.SnapshotResult{})
	for name, values := range map[string]any{
		"repositories":      empty.Repositories,
		"seats":             empty.Seats,
		"sessions":          empty.Sessions,
		"runtime sessions":  empty.Runtime.Sessions,
		"beads":             empty.Beads,
		"status counts":     empty.StatusCounts,
		"repository errors": empty.RepositoryErrors,
	} {
		if reflect.ValueOf(values).IsNil() {
			t.Errorf("%s is nil", name)
		}
	}

	input := snapshotFixture()
	got := normalizeSnapshot(input)
	for index, bead := range got.Beads {
		if bead.Dependencies == nil || bead.Labels == nil {
			t.Errorf("bead %d collections = %#v, want nonnil", index, bead)
		}
	}
	got.Repositories[0].Name = "mutated"
	got.Seats[0].Name = "mutated"
	got.Sessions[0].Handle = "mutated"
	got.Runtime.Sessions[0].Handle = "mutated"
	got.Beads[0].ID = "mutated"
	got.Beads[0].Labels[0] = "mutated"
	got.Beads[0].Dependencies[0].ID = "mutated"
	got.StatusCounts[0].Status = "mutated"
	got.RepositoryErrors[0].Repository = "mutated"

	if !reflect.DeepEqual(input, snapshotFixture()) {
		t.Errorf("returned collections share input storage: %#v", input)
	}
}

func snapshotFixture() wire.SnapshotResult {
	return wire.SnapshotResult{
		Runtime: wire.StatusResult{Sessions: []wire.SessionResult{
			{Handle: "z", UptimeSeconds: 1},
			{Handle: "b", UptimeSeconds: 2},
			{Handle: "a", UptimeSeconds: 2},
		}},
		Repositories: []wire.RepoResult{
			{Name: "beta"}, {Name: "alpha", Path: "/z"}, {Name: "alpha", Path: "/a"},
		},
		Seats: []wire.SeatResult{
			{Name: "b", Role: "implementer"}, {Name: "z", Role: "concierge"}, {Name: "a", Role: "implementer"},
		},
		Sessions: []wire.SessionResult{
			{Handle: "z", UptimeSeconds: 1}, {Handle: "b", UptimeSeconds: 2}, {Handle: "a", UptimeSeconds: 2},
		},
		Beads: []wire.BeadResult{
			{ID: "beta-1", Repo: "beta", Priority: 1},
			{ID: "alpha-2", Repo: "alpha", Priority: 2},
			{ID: "alpha-1", Repo: "alpha", Priority: 1, Labels: []string{"second", "first"}, Dependencies: []wire.DependencyResult{{ID: "second"}, {ID: "first"}}},
		},
		StatusCounts:     []wire.StatusCount{{Status: "open", Count: 2}, {Status: "closed", Count: 3}, {Status: "open", Count: 1}},
		RepositoryErrors: []wire.RepositoryError{{Repository: "beta"}, {Repository: "alpha", Error: "z"}, {Repository: "alpha", Error: "a"}},
	}
}

func assertRepositoryNames(t *testing.T, values []wire.RepoResult, want ...string) {
	t.Helper()
	got := make([]string, len(values))
	for index := range values {
		got[index] = values[index].Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("repository names = %#v, want %#v", got, want)
	}
}

func assertSeatNames(t *testing.T, values []wire.SeatResult, want ...string) {
	t.Helper()
	got := make([]string, len(values))
	for index := range values {
		got[index] = values[index].Role + "/" + values[index].Name
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("seats = %#v, want %#v", got, want)
	}
}

func assertSessionHandles(t *testing.T, values []wire.SessionResult, want ...string) {
	t.Helper()
	got := make([]string, len(values))
	for index := range values {
		got[index] = values[index].Handle
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sessions = %#v, want %#v", got, want)
	}
}

func assertBeadIDs(t *testing.T, values []wire.BeadResult, want ...string) {
	t.Helper()
	got := make([]string, len(values))
	for index := range values {
		got[index] = values[index].ID
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("beads = %#v, want %#v", got, want)
	}
}

func assertStatusCounts(t *testing.T, values []wire.StatusCount, want ...string) {
	t.Helper()
	got := make([]string, len(values))
	for index := range values {
		got[index] = values[index].Status
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("status counts = %#v, want %#v", got, want)
	}
}

func assertErrorRepositories(t *testing.T, values []wire.RepositoryError, want ...string) {
	t.Helper()
	got := make([]string, len(values))
	for index := range values {
		got[index] = values[index].Repository
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("repository errors = %#v, want %#v", got, want)
	}
}
