package kiro

import (
	"testing"

	"github.com/FreezingSnail/magicite/internal/agent"
	"github.com/FreezingSnail/magicite/internal/agent/conformance"
)

func TestConformance(t *testing.T) {
	conformance.Run(t, conformance.Case{
		Name: "kiro",
		Mode: "kiro",
		New: func(_ *testing.T, executable string) agent.Adapter {
			return New(Options{Executable: executable})
		},
		Workdir: gitWorkdir,
	})
}
