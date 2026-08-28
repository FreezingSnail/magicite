package parity

import "testing"

func TestOrchestrationCounterpartsExhaustive(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{
		"dispatch": "TestMaduinDispatchParity",
		"designer": "TestMaduinOrchestrationParity", "concierge": "TestMaduinOrchestrationParity",
		"handoff": "TestMaduinOrchestrationParity", "main": "TestMaduinOrchestrationParity",
		"fixture": "TestMaduinOrchestrationParity", "prompts": "TestMaduinOrchestrationParity",
		"planner": "TestMaduinOrchestrationParity", "plan": "TestMaduinOrchestrationParity",
		"no": "TestMaduinOrchestrationParity", "lifecycle": "TestMaduinOrchestrationParity",
		"install": "TestMaduinOrchestrationParity", "final": "TestMaduinOrchestrationParity",
		"check": "TestMaduinOrchestrationParity", "bootstrap": "TestMaduinOrchestrationParity",
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
	if len(counterparts) != want || want != 215 {
		t.Fatalf("counterpart count = %d, want %d", len(counterparts), want)
	}
}
