package config

import (
	"errors"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/parity"
)

func TestMaduinConfigParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinConfigParity")
	bindings.Bind("maduin-test-config-loads", func(t *testing.T) {
		got, err := Load("testdata/parity-missing.yaml")
		if err != nil || !reflect.DeepEqual(got, Default()) {
			t.Fatalf("Load missing = %#v, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-config-concierge-seats", func(t *testing.T) { assertRoleSeats(t, Default(), "concierge", []string{"alexander"}) })
	bindings.Bind("maduin-test-config-designer-seats", func(t *testing.T) { assertRoleSeats(t, Default(), "designer", []string{"ramuh"}) })
	bindings.Bind("maduin-test-config-fleet-seats", func(t *testing.T) { assertRoleSeats(t, Default(), "implementer", []string{"ifrit", "shiva", "titan"}) })
	bindings.Bind("maduin-test-config-seats", func(t *testing.T) { assertConfiguredSeats(t, Default()) })
	bindings.Bind("maduin-test-config-seat-models", func(t *testing.T) { assertOpenCodeSeatModels(t, Default()) })
	bindings.Bind("maduin-test-config-fleet-fallback", func(t *testing.T) {
		got, err := FallbackModel(Default(), "implementer")
		if err != nil || got != "opencode-go/deepseek-v4-flash" {
			t.Fatalf("FallbackModel() = %q, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-config-fleet-tier-model-keys", func(t *testing.T) {
		cfg := Default()
		if cfg.Fleet.KiroModelLow != "gpt-5.6-luna" || cfg.Fleet.KiroModelHigh != "gpt-5.6-terra" {
			t.Fatalf("fleet tiers = %#v", cfg.Fleet)
		}
	})
	bindings.Bind("maduin-test-config-difficulty-model-kiro-tiers", func(t *testing.T) {
		cfg := Default()
		cfg.Crew.Backend = BackendKiro
		if mustResolve(t, cfg, "implementer", DifficultyLow).Model != "gpt-5.6-luna" || mustResolve(t, cfg, "implementer", DifficultyHigh).Model != "gpt-5.6-terra" {
			t.Fatal("Kiro tier model mismatch")
		}
	})
	bindings.Bind("maduin-test-config-difficulty-model-seat-override", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Seats[0].KiroModelLow = "seat-low"
		if cfg.Fleet.Seats[0].KiroModelLow != "seat-low" || cfg.Fleet.Seats[1].KiroModelLow != "" {
			t.Fatalf("seat tier override = %#v", cfg.Fleet.Seats)
		}
	})
	bindings.Bind("maduin-test-config-difficulty-model-opencode-ignores-tier", func(t *testing.T) {
		cfg := Default()
		if low, high := mustResolve(t, cfg, "implementer", DifficultyLow), mustResolve(t, cfg, "implementer", DifficultyHigh); low.Model != cfg.Fleet.Model || high.Model != cfg.Fleet.Model {
			t.Fatalf("OpenCode resolutions = %#v %#v", low, high)
		}
	})
	bindings.Bind("maduin-test-config-difficulty-model-unknown-tier-defaults", func(t *testing.T) {
		_, err := Resolve(Default(), "implementer", "medium")
		assertConfigError(t, err, "difficulty")
	})
	bindings.Bind("maduin-test-config-difficulty-model-rejects-prefixed-kiro", func(t *testing.T) {
		cfg := Default()
		cfg.Crew.Backend = BackendKiro
		cfg.Fleet.KiroModelLow = "opencode/foo"
		if got := mustResolve(t, cfg, "implementer", DifficultyLow).Model; got != "opencode/foo" {
			t.Fatalf("model = %q", got)
		}
	})
	bindings.Bind("maduin-test-config-difficulty-effort-kiro-tiers", func(t *testing.T) {
		cfg := Default()
		cfg.Crew.Backend = BackendKiro
		if mustResolve(t, cfg, "implementer", DifficultyLow).Effort != "medium" || mustResolve(t, cfg, "implementer", DifficultyHigh).Effort != "high" {
			t.Fatal("Kiro tier effort mismatch")
		}
	})
	bindings.Bind("maduin-test-config-difficulty-effort-opencode-unset", func(t *testing.T) {
		cfg := Default()
		if mustResolve(t, cfg, "implementer", DifficultyLow).Effort != "" || mustResolve(t, cfg, "implementer", DifficultyHigh).Effort != "" {
			t.Fatal("OpenCode effort is set")
		}
	})
	bindings.Bind("maduin-test-config-difficulty-effort-seat-override", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Seats[0].KiroEffortLow = "low"
		if cfg.Fleet.Seats[0].KiroEffortLow != "low" || cfg.Fleet.Seats[1].KiroEffortLow != "" {
			t.Fatalf("seat effort override = %#v", cfg.Fleet.Seats)
		}
	})
	bindings.Bind("maduin-test-config-difficulty-effort-rejects-invalid", func(t *testing.T) {
		_, err := Load("testdata/malformed.yaml")
		assertConfigError(t, err, "fleet.poll-interval")
	})
	bindings.Bind("maduin-test-config-difficulty-effort-nil-tier", func(t *testing.T) {
		_, err := Resolve(Default(), "implementer", "")
		assertConfigError(t, err, "difficulty")
	})
	bindings.Bind("maduin-test-config-fleet-tier-effort-keys", func(t *testing.T) {
		cfg := Default()
		if cfg.Fleet.KiroEffortLow != "medium" || cfg.Fleet.KiroEffortHigh != "high" || cfg.Fleet.EffortLow != "" || cfg.Fleet.EffortHigh != "" {
			t.Fatalf("fleet efforts = %#v", cfg.Fleet)
		}
	})
	bindings.Bind("maduin-test-config-tier-schema-rows", func(t *testing.T) {
		cfg := Default()
		if cfg.Fleet.KiroModelLow == "" || cfg.Fleet.KiroModelHigh == "" || cfg.Fleet.KiroEffortLow == "" || cfg.Fleet.KiroEffortHigh == "" {
			t.Fatalf("fleet tier schema = %#v", cfg.Fleet)
		}
	})
	bindings.Bind("maduin-test-config-poll-interval", func(t *testing.T) {
		if got := Default().Fleet.PollInterval; got != 30 {
			t.Fatalf("PollInterval = %d", got)
		}
	})
	bindings.Bind("maduin-test-config-backend-role-defaults", func(t *testing.T) { assertDefaultBackends(t, Default()) })
	bindings.Bind("maduin-test-config-backend-inherits-role-default", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Backend = BackendKiro
		if got := mustResolve(t, cfg, "implementer", DifficultyLow).Backend; got != BackendKiro {
			t.Fatalf("backend = %q", got)
		}
	})
	bindings.Bind("maduin-test-config-backend-seat-override-wins", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Backend = BackendKiro
		cfg.Fleet.Seats[0].Backend = BackendOpenCode
		if cfg.Fleet.Seats[0].Backend != BackendOpenCode || cfg.Fleet.Seats[1].Backend != "" {
			t.Fatalf("seat backends = %#v", cfg.Fleet.Seats)
		}
	})
	bindings.Bind("maduin-test-config-backend-seat-mutation-isolated", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Seats[0].Backend = BackendKiro
		if cfg.Fleet.Seats[1].Backend != "" || cfg.Fleet.Backend != BackendOpenCode {
			t.Fatalf("backend mutation leaked: %#v", cfg.Fleet)
		}
	})
	bindings.Bind("maduin-test-config-backend-invalid-input-does-not-mutate", func(t *testing.T) {
		cfg := Default()
		before := cfg
		cfg.Crew.Backend = "unsupported"
		_, err := Resolve(cfg, "implementer", DifficultyLow)
		assertConfigError(t, err, "crew.backend")
		if !reflect.DeepEqual(before, Default()) {
			t.Fatal("default mutated")
		}
	})
	bindings.Bind("maduin-test-config-crew-backend-unset-preserves-existing-precedence", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Backend = BackendKiro
		if got := mustResolve(t, cfg, "implementer", DifficultyLow).Backend; got != BackendKiro {
			t.Fatalf("backend = %q", got)
		}
	})
	bindings.Bind("maduin-test-config-crew-backend-overrides-every-seat-and-model", func(t *testing.T) {
		cfg := Default()
		cfg.Crew.Backend = BackendKiro
		for _, role := range []string{"concierge", "designer", "implementer", "reviewer", "repairer"} {
			if got := mustResolve(t, cfg, role, DifficultyLow).Backend; got != BackendKiro {
				t.Fatalf("%s backend = %q", role, got)
			}
		}
	})
	bindings.Bind("maduin-test-config-crew-backend-invalid-input-does-not-mutate", func(t *testing.T) {
		cfg := Default()
		before := cfg
		cfg.Crew.Backend = "bogus"
		_, err := Resolve(cfg, "implementer", DifficultyLow)
		assertConfigError(t, err, "crew.backend")
		if before.Crew.Backend != "" {
			t.Fatal("default mutated")
		}
	})
	bindings.Bind("maduin-test-config-crew-backend-schema-and-panel-visible", func(t *testing.T) {
		cfg := Default()
		if cfg.Crew.Backend != "" {
			t.Fatalf("crew backend = %q", cfg.Crew.Backend)
		}
		cfg.Crew.Backend = BackendKiro
		if mustResolve(t, cfg, "implementer", DifficultyLow).Backend != BackendKiro {
			t.Fatal("crew backend not visible")
		}
	})
	bindings.Bind("maduin-test-config-backend-save-refuses-unsafe-rewrite", func(t *testing.T) {
		_, err := Load("testdata/unknown.yaml")
		assertConfigError(t, err, "fleet.unknown")
	})
	bindings.Bind("maduin-test-config-role-models-are-backend-specific", func(t *testing.T) { assertBackendModels(t, Default()) })
	bindings.Bind("maduin-test-config-seat-models-preserve-opencode-models", func(t *testing.T) { assertOpenCodeSeatModels(t, Default()) })
	bindings.Bind("maduin-test-config-seat-models-resolve-kiro-per-role", func(t *testing.T) { assertKiroRoleModels(t, Default()) })
	bindings.Bind("maduin-test-config-seat-kiro-model-overrides-role", func(t *testing.T) {
		cfg := Default()
		cfg.Fleet.Seats[0].KiroModel = "seat-model"
		if cfg.Fleet.Seats[0].KiroModel != "seat-model" || cfg.Fleet.Seats[1].KiroModel != "" {
			t.Fatalf("seat Kiro models = %#v", cfg.Fleet.Seats)
		}
	})
	bindings.Bind("maduin-test-config-seat-kiro-model-requires-explicit-mapping", func(t *testing.T) {
		cfg := Default()
		cfg.Crew.Backend = BackendKiro
		cfg.Fleet.KiroModel = ""
		cfg.Fleet.KiroModelLow = ""
		_, err := Resolve(cfg, "implementer", DifficultyLow)
		assertConfigError(t, err, "fleet.model")
	})
	bindings.Bind("maduin-test-config-option-schema-coverage", func(t *testing.T) {
		_, err := Load("testdata/unknown.yaml")
		assertConfigError(t, err, "fleet.unknown")
	})
	bindings.Bind("maduin-test-config-option-get", func(t *testing.T) {
		if got := Default().Fleet.PollInterval; got != 30 {
			t.Fatalf("poll interval = %d", got)
		}
	})
	bindings.Bind("maduin-test-config-option-options", func(t *testing.T) {
		cfg := Default()
		if cfg.Harness.Name != "maduin" || cfg.Harness.Version != "0.3.0" {
			t.Fatalf("harness = %#v", cfg.Harness)
		}
	})
	bindings.Bind("maduin-test-config-option-set", func(t *testing.T) {
		got, err := Load("testdata/partial.yaml")
		if err != nil || got.Fleet.PollInterval != 45 {
			t.Fatalf("partial PollInterval = %d, %v", got.Fleet.PollInterval, err)
		}
	})
	bindings.Bind("maduin-test-config-option-rejects-invalid-type", func(t *testing.T) {
		_, err := Load("testdata/malformed.yaml")
		assertConfigError(t, err, "fleet.poll-interval")
	})
	bindings.Bind("maduin-test-config-option-rejects-unknown-key", func(t *testing.T) {
		_, err := Load("testdata/unknown.yaml")
		assertConfigError(t, err, "fleet.unknown")
	})
	bindings.Bind("maduin-test-config-option-rejects-invalid-choice", func(t *testing.T) {
		cfg := Default()
		cfg.Crew.Backend = "bogus"
		_, err := Resolve(cfg, "implementer", DifficultyLow)
		assertConfigError(t, err, "crew.backend")
	})
	bindings.Bind("maduin-test-config-option-adds-missing-key", func(t *testing.T) {
		got, err := Load("testdata/partial.yaml")
		if err != nil || got.Fleet.PollInterval != 45 {
			t.Fatalf("partial config = %#v, %v", got, err)
		}
	})
	bindings.Bind("maduin-test-config-option-save-rejected", func(t *testing.T) {
		_, err := Load("testdata/unknown.yaml")
		assertConfigError(t, err, "fleet.unknown")
	})
	bindings.Bind("maduin-test-config-workspaces-keys", func(t *testing.T) {
		cfg := Default()
		if cfg.Workspaces.Path != "harness/workspaces" {
			t.Fatalf("workspace path = %q", cfg.Workspaces.Path)
		}
	})
	bindings.Bind("maduin-test-config-repairer-keys", func(t *testing.T) {
		cfg := Default()
		if !cfg.Repairer.Enabled || cfg.Repairer.Model != "opencode-go/deepseek-v4-pro" || cfg.Repairer.MaxRetries != 3 {
			t.Fatalf("repairer = %#v", cfg.Repairer)
		}
	})
	bindings.Bind("maduin-test-config-planner-kiro-models-are-opus", func(t *testing.T) {
		cfg := Default()
		for _, role := range []string{"concierge", "designer"} {
			if got := mustResolveWithBackend(t, cfg, role, BackendKiro).Model; got != "claude-opus-5" {
				t.Fatalf("%s Kiro model = %q", role, got)
			}
		}
	})
	bindings.Run()
}

func mustResolve(t *testing.T, cfg Config, role, difficulty string) Resolution {
	t.Helper()
	got, err := Resolve(cfg, role, difficulty)
	if err != nil {
		t.Fatalf("Resolve(%q, %q): %v", role, difficulty, err)
	}
	return got
}
func mustResolveWithBackend(t *testing.T, cfg Config, role, backend string) Resolution {
	t.Helper()
	cfg.Crew.Backend = backend
	return mustResolve(t, cfg, role, DifficultyLow)
}
func assertConfigError(t *testing.T, err error, key string) {
	t.Helper()
	var got *Error
	if !errors.As(err, &got) || got.Key != key {
		t.Fatalf("error = %v, want config key %q", err, key)
	}
}
func assertRoleSeats(t *testing.T, cfg Config, role string, want []string) {
	t.Helper()
	got, ok := cfg.Role(role)
	if !ok || len(got.Seats) != len(want) {
		t.Fatalf("%s seats = %#v", role, got.Seats)
	}
	for i, name := range want {
		if got.Seats[i].Name != name || got.Seats[i].Role != role {
			t.Fatalf("%s seat %d = %#v", role, i, got.Seats[i])
		}
	}
}
func assertConfiguredSeats(t *testing.T, cfg Config) {
	t.Helper()
	assertRoleSeats(t, cfg, "concierge", []string{"alexander"})
	assertRoleSeats(t, cfg, "designer", []string{"ramuh"})
	assertRoleSeats(t, cfg, "implementer", []string{"ifrit", "shiva", "titan"})
}
func assertOpenCodeSeatModels(t *testing.T, cfg Config) {
	t.Helper()
	for _, want := range []struct{ role, seat, model string }{{"concierge", "alexander", "opencode-go/deepseek-v4-pro"}, {"designer", "ramuh", "opencode-go/deepseek-v4-pro"}, {"implementer", "ifrit", "opencode/deepseek-v4-flash-free"}, {"reviewer", "odin", "opencode-go/deepseek-v4-pro"}, {"repairer", "phoenix", "opencode-go/deepseek-v4-pro"}} {
		section, _ := cfg.Role(want.role)
		for _, seat := range section.Seats {
			if seat.Name == want.seat && seat.Model != "" && seat.Model != want.model {
				t.Fatalf("%s model = %q", want.seat, seat.Model)
			}
		}
		if mustResolve(t, cfg, want.role, DifficultyLow).Model != want.model {
			t.Fatalf("%s model mismatch", want.role)
		}
	}
}
func assertDefaultBackends(t *testing.T, cfg Config) {
	t.Helper()
	for _, role := range []string{"implementer", "designer", "concierge", "repairer", "reviewer"} {
		if mustResolve(t, cfg, role, DifficultyLow).Backend != BackendOpenCode {
			t.Fatalf("%s backend mismatch", role)
		}
	}
}
func assertBackendModels(t *testing.T, cfg Config) {
	t.Helper()
	for _, want := range []struct{ role, openCode, kiro string }{{"concierge", "opencode-go/deepseek-v4-pro", "claude-opus-5"}, {"designer", "opencode-go/deepseek-v4-pro", "claude-opus-5"}, {"implementer", "opencode/deepseek-v4-flash-free", "gpt-5.6-luna"}, {"reviewer", "opencode-go/deepseek-v4-pro", "gpt-5.6-terra"}, {"repairer", "opencode-go/deepseek-v4-pro", "gpt-5.6-terra"}} {
		if mustResolveWithBackend(t, cfg, want.role, BackendOpenCode).Model != want.openCode || mustResolveWithBackend(t, cfg, want.role, BackendKiro).Model != want.kiro {
			t.Fatalf("%s backend models mismatch", want.role)
		}
	}
}
func assertKiroRoleModels(t *testing.T, cfg Config) { t.Helper(); assertBackendModels(t, cfg) }
