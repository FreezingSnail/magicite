package config

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinConfigParity(t *testing.T) {
	for _, name := range configParityNames() {
		t.Run(name, func(t *testing.T) { assertConfigReplay(t, name) })
	}
}

func configParityNames() []string {
	counterparts := parity.SubstrateCounterparts()
	names := make([]string, 0, 48)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinConfigParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func assertConfigReplay(t *testing.T, name string) {
	t.Helper()
	cfg := Default()
	if cfg.Harness.Name != "maduin" || cfg.Workspaces.Path != "harness/workspaces" {
		t.Fatalf("default substrate = %#v", cfg)
	}
	roles := []struct {
		role  string
		seats []string
	}{
		{"concierge", []string{"alexander"}}, {"designer", []string{"ramuh"}},
		{"implementer", []string{"ifrit", "shiva", "titan"}}, {"reviewer", []string{"odin"}}, {"repairer", []string{"phoenix"}},
	}
	for _, want := range roles {
		section, ok := cfg.Role(want.role)
		if !ok || len(section.Seats) != len(want.seats) {
			t.Fatalf("role %q seats = %#v", want.role, section.Seats)
		}
		for i, seat := range section.Seats {
			if seat.Name != want.seats[i] || seat.Role == "" {
				t.Fatalf("role %q seat %d = %#v", want.role, i, seat)
			}
		}
		for _, backend := range []string{BackendOpenCode, BackendKiro} {
			copy := cfg
			copy.Crew.Backend = backend
			resolution, err := Resolve(copy, want.role, DifficultyLow)
			if err != nil || resolution.Backend != backend || resolution.Model == "" {
				t.Fatalf("Resolve(%q, %q) = %#v, %v", want.role, backend, resolution, err)
			}
		}
	}
	low, err := Resolve(Config{Crew: CrewConfig{Backend: BackendKiro}, Fleet: cfg.Fleet, Concierge: cfg.Concierge, Designer: cfg.Designer, Reviewer: cfg.Reviewer, Repairer: cfg.Repairer}, "implementer", DifficultyLow)
	if err != nil || low.Model != "gpt-5.6-luna" || low.Effort != "medium" {
		t.Fatalf("fleet low Kiro = %#v, %v", low, err)
	}
	if fallback, err := FallbackModel(cfg, "implementer"); err != nil || fallback != "opencode-go/deepseek-v4-flash" {
		t.Fatalf("fleet fallback = %q, %v", fallback, err)
	}
	if strings.Contains(name, "loads") {
		loaded, err := Load("testdata/parity-missing.yaml")
		if err != nil || !reflect.DeepEqual(loaded, cfg) {
			t.Fatalf("Load missing = %#v, %v", loaded, err)
		}
	}
}
