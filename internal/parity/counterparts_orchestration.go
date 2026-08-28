package parity

import "fmt"

// OrchestrationCounterparts maps every ported orchestration invariant to its
// permanent Go replay subtest. The current Go ownership names land and gate
// where maduin named the corresponding layers pipeline and review.
func OrchestrationCounterparts() map[string]string {
	owners := map[string]string{
		"dispatch": "TestMaduinDispatchParity", "designer": "TestMaduinDispatchParity",
		"concierge": "TestMaduinDispatchParity", "handoff": "TestMaduinDispatchParity",
		"main": "TestMaduinDispatchParity", "fixture": "TestMaduinDispatchParity",
		"repair": "TestMaduinPipelineParity", "prompts": "TestMaduinDispatchParity",
		"planner": "TestMaduinDispatchParity", "plan": "TestMaduinDispatchParity",
		"no": "TestMaduinDispatchParity", "lifecycle": "TestMaduinDispatchParity",
		"install": "TestMaduinDispatchParity", "final": "TestMaduinDispatchParity",
		"check": "TestMaduinDispatchParity", "bootstrap": "TestMaduinDispatchParity",
		"session": "TestMaduinAgentParity", "kiro": "TestMaduinAgentParity",
		"backend": "TestMaduinAgentParity", "pipeline": "TestMaduinPipelineParity",
		"stamp": "TestMaduinStampParity", "review": "TestMaduinReviewParity",
		"decomp": "TestMaduinDecompParity",
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
