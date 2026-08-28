package parity

import "testing"

func TestOrchestrationCounterpartsExhaustive(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{
		"dispatch": "TestMaduinDispatchParity", "designer": "TestMaduinDispatchParity", "concierge": "TestMaduinDispatchParity", "handoff": "TestMaduinDispatchParity", "main": "TestMaduinDispatchParity", "fixture": "TestMaduinDispatchParity", "prompts": "TestMaduinDispatchParity", "planner": "TestMaduinDispatchParity", "plan": "TestMaduinDispatchParity", "no": "TestMaduinDispatchParity", "lifecycle": "TestMaduinDispatchParity", "install": "TestMaduinDispatchParity", "final": "TestMaduinDispatchParity", "check": "TestMaduinDispatchParity", "bootstrap": "TestMaduinDispatchParity",
		"session": "TestMaduinAgentParity", "kiro": "TestMaduinAgentParity", "backend": "TestMaduinAgentParity",
		"pipeline": "TestMaduinPipelineParity", "repair": "TestMaduinPipelineParity", "stamp": "TestMaduinStampParity", "review": "TestMaduinReviewParity", "decomp": "TestMaduinDecompParity",
	}
	counterparts := OrchestrationCounterparts()
	want := 0
	for _, invariant := range catalog.Entries {
		owner, selected := owners[invariant.Domain]
		got, mapped := counterparts[invariant.Name]
		if selected {
			want++
			if !mapped || got != owner+"/"+invariant.Name {
				t.Errorf("counterpart[%q] = %q, want %q", invariant.Name, got, owner+"/"+invariant.Name)
			}
		} else if mapped {
			t.Errorf("unexpected counterpart[%q] = %q", invariant.Name, got)
		}
	}
	if len(counterparts) != want || want != 209 {
		t.Fatalf("counterpart count = %d, want %d", len(counterparts), want)
	}
}
