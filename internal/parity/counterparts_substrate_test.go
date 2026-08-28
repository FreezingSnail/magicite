package parity

import "testing"

func TestSubstrateCounterpartsExhaustive(t *testing.T) {
	catalog, err := LoadCatalog()
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{
		"config": "TestMaduinConfigParity", "log": "TestMaduinLoggingParity",
		"state": "TestMaduinStateParity", "bd": "TestMaduinBDParity",
		"repo": "TestMaduinRepoParity", "workspace": "TestMaduinWorktreeParity",
		"multirepo": "TestMaduinRepoParity", "backcompat": "TestMaduinBDParity",
	}
	counterparts := SubstrateCounterparts()
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
	if len(counterparts) != want || want != 107 {
		t.Fatalf("counterpart count = %d, want %d", len(counterparts), want)
	}
}
