package parity

import "fmt"

// SubstrateCounterparts maps every substrate and data invariant to its
// permanent, package-local Go replay subtest.
func SubstrateCounterparts() map[string]string {
	owners := map[string]string{
		"config": "TestMaduinConfigParity", "log": "TestMaduinLoggingParity",
		"state": "TestMaduinStateParity", "bd": "TestMaduinBDParity",
		"repo": "TestMaduinRepoParity", "workspace": "TestMaduinWorktreeParity",
		"multirepo": "TestMaduinRepoParity", "backcompat": "TestMaduinBDParity",
	}
	catalog, err := LoadCatalog()
	if err != nil {
		panic(fmt.Sprintf("load substrate parity catalog: %v", err))
	}
	counterparts := make(map[string]string)
	for _, invariant := range catalog.Entries {
		if owner, ok := owners[invariant.Domain]; ok {
			counterparts[invariant.Name] = owner + "/" + invariant.Name
		}
	}
	return counterparts
}
