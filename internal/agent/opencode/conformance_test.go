package opencode

import (
	"testing"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/agent/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, conformance.Case{
		Name: "opencode",
		Mode: "opencode",
		New: func(_ *testing.T, executable string) agent.Adapter {
			return New(Options{Executable: executable})
		},
		Workdir: func(t *testing.T) string { return t.TempDir() },
	})
}
