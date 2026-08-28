package opencode

import (
	"testing"

	"github.com/connorfranc/magicite/internal/agent"
)

// These public helper checks back TestMaduinAgentParity without requiring its
// owner package to reach adapter internals.
func TestParityOpenCodePublicHelpers(t *testing.T) {
	event, ok := ParseLine(`{"type":"step_finish","sessionID":"ses","part":{"reason":"stop"}}`)
	if !ok || event.SessionID != "ses" || event.Terminal != agent.StatusCompleted {
		t.Fatalf("ParseLine = (%#v, %t)", event, ok)
	}
	args := RunArgs("opencode", "/work", "model", "", "", "h", "plan")
	if len(args) != 12 || args[1] != "run" || args[len(args)-1] != "plan" {
		t.Fatalf("RunArgs = %q", args)
	}
}
