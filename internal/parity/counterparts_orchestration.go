package parity

import "fmt"

// OrchestrationCounterparts maps every ported orchestration invariant to its
// permanent Go replay subtest. The current Go ownership names land and gate
// where maduin named the corresponding layers pipeline and review.
func OrchestrationCounterparts() map[string]string {
	owners := map[string]string{
		"dispatch": "TestMaduinDispatchParity",
		"designer": "TestMaduinOrchestrationParity", "concierge": "TestMaduinOrchestrationParity",
		"handoff": "TestMaduinOrchestrationParity", "main": "TestMaduinOrchestrationParity",
		"fixture": "TestMaduinOrchestrationParity", "prompts": "TestMaduinOrchestrationParity",
		"planner": "TestMaduinOrchestrationParity", "plan": "TestMaduinOrchestrationParity",
		"no": "TestMaduinOrchestrationParity", "lifecycle": "TestMaduinOrchestrationParity",
		"install": "TestMaduinOrchestrationParity", "final": "TestMaduinOrchestrationParity",
		"check": "TestMaduinOrchestrationParity", "bootstrap": "TestMaduinOrchestrationParity",
		"repair": "TestMaduinPipelineParity", "session": "TestMaduinAgentParity",
		"kiro": "TestMaduinAgentParity", "backend": "TestMaduinAgentParity",
		"pipeline": "TestMaduinPipelineParity", "stamp": "TestMaduinStampParity",
		"review": "TestMaduinReviewParity", "decomp": "TestMaduinDecompParity",
	}

	catalog, err := LoadCatalog()
	if err != nil {
		panic(fmt.Sprintf("load orchestration parity catalog: %v", err))
	}
	counterparts := make(map[string]string)
	for _, invariant := range catalog.Entries {
		if owner, ok := owners[invariant.Domain]; ok {
			counterparts[invariant.Name] = owner + "/" + invariant.Name
		}
	}
	return counterparts
}
