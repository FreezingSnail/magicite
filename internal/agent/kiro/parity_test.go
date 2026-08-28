package kiro

import "testing"

// These public helper checks back TestMaduinAgentParity without requiring its
// owner package to reach adapter internals.
func TestParityKiroPublicHelpers(t *testing.T) {
	if !ValidModel("model") || ValidModel("provider/model") {
		t.Fatal("model validation changed")
	}
	args := RunArgs("kiro", "model", "worker", "high", "plan")
	if len(args) != 11 || args[1] != "chat" || args[2] != "--no-interactive" || args[9] != "--trust-all-tools" {
		t.Fatalf("RunArgs = %q", args)
	}
}
