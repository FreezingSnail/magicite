package config

import (
	"errors"
	"testing"
)

func TestResolveSelectsBackendModelAndEffort(t *testing.T) {
	cfg := Default()
	cfg.Crew.Backend = BackendKiro

	low, err := Resolve(cfg, "implementer", DifficultyLow)
	if err != nil {
		t.Fatalf("Resolve(low) error = %v", err)
	}
	if want := (Resolution{Backend: BackendKiro, Agent: "slugineer-worker", Model: "gpt-5.6-luna", Effort: "medium"}); low != want {
		t.Errorf("Resolve(low) = %#v, want %#v", low, want)
	}

	high, err := Resolve(cfg, "implementer", DifficultyHigh)
	if err != nil {
		t.Fatalf("Resolve(high) error = %v", err)
	}
	if want := (Resolution{Backend: BackendKiro, Agent: "slugineer-worker", Model: "gpt-5.6-terra", Effort: "high"}); high != want {
		t.Errorf("Resolve(high) = %#v, want %#v", high, want)
	}
}

func TestResolveCrewBackendOverridesRole(t *testing.T) {
	cfg := Default()
	cfg.Fleet.Backend = BackendKiro
	cfg.Crew.Backend = BackendOpenCode

	got, err := Resolve(cfg, "implementer", DifficultyLow)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if got.Backend != BackendOpenCode || got.Model != cfg.Fleet.Model {
		t.Errorf("Resolve() = %#v, want crew backend and OpenCode model", got)
	}
}

func TestResolveFallsBackToRoleModel(t *testing.T) {
	cfg := Default()
	cfg.Crew.Backend = BackendKiro
	cfg.Fleet.KiroModelLow = ""
	cfg.Fleet.KiroModelHigh = ""

	for _, difficulty := range []string{DifficultyLow, DifficultyHigh} {
		got, err := Resolve(cfg, "implementer", difficulty)
		if err != nil {
			t.Fatalf("Resolve(%q) error = %v", difficulty, err)
		}
		if got.Model != cfg.Fleet.KiroModel {
			t.Errorf("Resolve(%q) model = %q, want role default %q", difficulty, got.Model, cfg.Fleet.KiroModel)
		}
	}
}

func TestResolveAllRolesAndRejectsInvalidInputs(t *testing.T) {
	cfg := Default()
	for _, role := range []string{"concierge", "designer", "implementer", "fleet", "reviewer", "repairer"} {
		for _, difficulty := range []string{DifficultyLow, DifficultyHigh} {
			if _, err := Resolve(cfg, role, difficulty); err != nil {
				t.Errorf("Resolve(%q, %q) error = %v", role, difficulty, err)
			}
		}
	}

	for _, test := range []struct {
		role       string
		difficulty string
		key        string
	}{
		{role: "unknown", difficulty: DifficultyLow, key: "role"},
		{role: "implementer", difficulty: "medium", key: "difficulty"},
	} {
		_, err := Resolve(cfg, test.role, test.difficulty)
		var configErr *Error
		if !errors.As(err, &configErr) {
			t.Errorf("Resolve(%q, %q) error = %v, want *Error", test.role, test.difficulty, err)
			continue
		}
		if configErr.Key != test.key {
			t.Errorf("Resolve(%q, %q) error key = %q, want %q", test.role, test.difficulty, configErr.Key, test.key)
		}
	}
}

func TestFallbackModelUsesEffectiveBackend(t *testing.T) {
	cfg := Default()

	got, err := FallbackModel(cfg, "implementer")
	if err != nil {
		t.Fatalf("FallbackModel() error = %v", err)
	}
	if got != cfg.Fleet.Fallback {
		t.Errorf("FallbackModel() = %q, want %q", got, cfg.Fleet.Fallback)
	}

	cfg.Crew.Backend = BackendKiro
	got, err = FallbackModel(cfg, "implementer")
	if err != nil {
		t.Fatalf("FallbackModel() with crew Kiro error = %v", err)
	}
	if got != cfg.Fleet.KiroFallback {
		t.Errorf("FallbackModel() with crew Kiro = %q, want %q", got, cfg.Fleet.KiroFallback)
	}
}
