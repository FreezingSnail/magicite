package config

import (
	"fmt"
	"strings"
)

const (
	DifficultyLow  = "low"
	DifficultyHigh = "high"
)

// Resolution is the backend-specific configuration used to run one role.
type Resolution struct {
	Backend string
	Agent   string
	Model   string
	Effort  string
}

// Resolve returns the effective backend, agent, model, and effort for ROLE at
// DIFFICULTY. A crew backend overrides the role backend. Kiro low and high
// models fall back to the role's KiroModel when the tier is unset.
func Resolve(cfg Config, role, difficulty string) (Resolution, error) {
	section, key, err := resolveRole(cfg, role)
	if err != nil {
		return Resolution{}, err
	}
	if difficulty != DifficultyLow && difficulty != DifficultyHigh {
		return Resolution{}, resolveError("difficulty", "must be low or high")
	}

	backend, err := effectiveBackend(cfg, section, key)
	if err != nil {
		return Resolution{}, err
	}
	if strings.TrimSpace(section.Agent) == "" {
		return Resolution{}, resolveError(key+".agent", "must not be empty")
	}

	resolution := Resolution{Backend: backend, Agent: section.Agent}
	switch backend {
	case BackendOpenCode:
		resolution.Model = section.Model
		if difficulty == DifficultyLow {
			resolution.Effort = section.EffortLow
		} else {
			resolution.Effort = section.EffortHigh
		}
	case BackendKiro:
		resolution.Model = section.KiroModel
		if difficulty == DifficultyLow {
			if section.KiroModelLow != "" {
				resolution.Model = section.KiroModelLow
			}
			resolution.Effort = section.KiroEffortLow
		} else {
			if section.KiroModelHigh != "" {
				resolution.Model = section.KiroModelHigh
			}
			resolution.Effort = section.KiroEffortHigh
		}
	}
	if strings.TrimSpace(resolution.Model) == "" {
		return Resolution{}, resolveError(key+".model", "must not be empty")
	}
	return resolution, nil
}

// FallbackModel returns ROLE's substitute model for its effective backend.
func FallbackModel(cfg Config, role string) (string, error) {
	section, key, err := resolveRole(cfg, role)
	if err != nil {
		return "", err
	}
	backend, err := effectiveBackend(cfg, section, key)
	if err != nil {
		return "", err
	}

	fallback, fallbackKey := section.Fallback, key+".fallback"
	if backend == BackendKiro {
		fallback, fallbackKey = section.KiroFallback, key+".kiro-fallback"
	}
	if strings.TrimSpace(fallback) == "" {
		return "", resolveError(fallbackKey, "must not be empty")
	}
	return fallback, nil
}

func effectiveBackend(cfg Config, section RoleConfig, key string) (string, error) {
	if cfg.Crew.Backend != "" {
		if !validBackend(cfg.Crew.Backend) {
			return "", resolveError("crew.backend", "must be opencode or kiro")
		}
		return cfg.Crew.Backend, nil
	}
	if !validBackend(section.Backend) {
		return "", resolveError(key+".backend", "must be opencode or kiro")
	}
	return section.Backend, nil
}

func resolveRole(cfg Config, role string) (RoleConfig, string, error) {
	section, ok := cfg.Role(role)
	if !ok {
		return RoleConfig{}, "", resolveError("role", fmt.Sprintf("unsupported role %q", role))
	}
	key := role
	if role == "implementer" {
		key = "fleet"
	}
	return section, key, nil
}

func resolveError(key, message string) error {
	return &Error{Key: key, Err: fmt.Errorf("%s", message)}
}
