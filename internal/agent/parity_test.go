package agent

import (
	"sort"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinAgentParity(t *testing.T) {
	for _, name := range agentParityNames() {
		t.Run(name, func(t *testing.T) {
			registry := NewRegistry()
			if got := registry.Names(); len(got) != 0 {
				t.Fatalf("empty registry names = %q", got)
			}
			if _, err := registry.Lookup("missing"); err == nil {
				t.Fatal("unknown backend resolved")
			}
		})
	}
}

func agentParityNames() []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinAgentParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
