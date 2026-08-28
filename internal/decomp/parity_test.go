package decomp

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/parity"
)

func TestMaduinDecompParity(t *testing.T) {
	for _, name := range decompParityNames() {
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile("testdata/clean-ten.json")
			if err != nil {
				t.Fatal(err)
			}
			var fixture checkFixture
			if err := json.Unmarshal(data, &fixture); err != nil {
				t.Fatal(err)
			}
			if got := Check(fixture.Children); len(got) != 0 {
				t.Fatalf("Check(clean-ten) = %#v", got)
			}
			fixture.Children[0].Acceptance = "not\nplain"
			violations := Check(fixture.Children)
			found := false
			for _, violation := range violations {
				found = found || violation.ID == "a" && violation.Rule == RuleCapAcceptance
			}
			if !found {
				t.Fatalf("Check(invalid acceptance) = %#v", violations)
			}
		})
	}
}

func decompParityNames() []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinDecompParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
