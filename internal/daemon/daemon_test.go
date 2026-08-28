package daemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/connorfranc/magicite/internal/gate"
	"github.com/connorfranc/magicite/internal/testenv"
)

func TestAssembleWiresGateAndState(t *testing.T) {
	assembly := assembleForTest(t, true)
	if assembly.State == nil {
		t.Fatal("Assemble() State = nil")
	}
	capability, ok := assembly.Core.(*core)
	if !ok {
		t.Fatalf("Assemble() Core = %T, want *core", assembly.Core)
	}
	qualityGate, ok := capability.gate.(*gate.Gate)
	if !ok {
		t.Fatalf("core gate = %T, want *gate.Gate", capability.gate)
	}
	if !qualityGate.Enabled() {
		t.Fatal("gate unexpectedly disabled")
	}
}

func TestAssembleWiresDisabledGate(t *testing.T) {
	assembly := assembleForTest(t, false)
	capability := assembly.Core.(*core)
	qualityGate, ok := capability.gate.(*gate.Gate)
	if !ok {
		t.Fatalf("core gate = %T, want *gate.Gate", capability.gate)
	}
	if qualityGate.Enabled() {
		t.Fatal("gate unexpectedly enabled")
	}
	if hold, err := qualityGate.Hold(context.Background(), testRepo(t)); err != nil || hold {
		t.Fatalf("disabled gate Hold() = %t, %v", hold, err)
	}
}

func assembleForTest(t *testing.T, enabled bool) *Assembly {
	t.Helper()
	env := testenv.New(t)
	record := testenv.NewRepo(t, env, "project")
	cfgPath := filepath.Join(t.TempDir(), "magicite.yaml")
	config := fmt.Sprintf("repos:\n  roots:\n    - %s\nreviewer:\n  enabled: %t\n", record.Root, enabled)
	if err := os.WriteFile(cfgPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	assembly, err := Assemble(context.Background(), cfgPath)
	if err != nil {
		t.Fatalf("Assemble() error = %v", err)
	}
	return assembly
}
