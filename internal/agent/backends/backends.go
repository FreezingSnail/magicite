// Package backends wires configured agent adapters into a runtime.
package backends

import (
	"errors"
	"os"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/agent/kiro"
	"github.com/connorfranc/magicite/internal/agent/opencode"
	"github.com/connorfranc/magicite/internal/config"
)

var errNilRegistry = errors.New("nil agent registry")

// Register constructs and registers the supported agent backends.
func Register(reg *agent.Registry, cfg config.Config) error {
	if reg == nil {
		return errNilRegistry
	}
	_ = cfg

	for _, adapter := range []agent.Adapter{
		opencode.New(opencode.Options{}),
		kiro.New(kiro.Options{Env: os.Environ()}),
	} {
		if err := reg.Register(adapter); err != nil {
			return err
		}
	}
	return nil
}

// New constructs a runtime containing all supported agent backends.
func New(cfg config.Config) (*agent.Runtime, error) {
	reg := agent.NewRegistry()
	if err := Register(reg, cfg); err != nil {
		return nil, err
	}
	return agent.NewRuntime(reg), nil
}

// Missing returns registered backends whose executables are unavailable.
func Missing(reg *agent.Registry) []string {
	if reg == nil {
		return nil
	}

	var missing []string
	for _, name := range reg.Names() {
		if errors.Is(reg.Available(name), agent.ErrExecutableMissing) {
			missing = append(missing, name)
		}
	}
	return missing
}
