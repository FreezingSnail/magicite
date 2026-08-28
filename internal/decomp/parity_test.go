package decomp

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/connorfranc/magicite/internal/parity"
)

func TestMaduinDecompParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinDecompParity")
	bindings.Bind("maduin-test-decomp-gate-approved-labels-and-clears-rejection", func(t *testing.T) {
		children := parityFixture(t, "clean-ten.json")
		if got := Check(children); got != nil {
			t.Fatalf("Check(clean-ten) = %#v", got)
		}
	})
	bindings.Bind("maduin-test-decomp-gate-rejected-files-once-then-comments", func(t *testing.T) {
		children := parityFixture(t, "two-child-cycle.json")
		if got := Check(children); !hasRule(got, RuleGraphCycle) {
			t.Fatalf("Check(graph-cycle) = %#v", got)
		}
	})
	bindings.Bind("maduin-test-decomp-gate-partial-retries-and-read-errors-contain", func(t *testing.T) {
		children := parityFixture(t, "unprovided-symbol.json")
		if got := Check(children); !hasRule(got, RuleUnprovidedSymbol) {
			t.Fatalf("Check(unprovided-symbol) = %#v", got)
		}
	})
	bindings.Run()
}

func parityFixture(t *testing.T, name string) []Child {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var fixture checkFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Children
}

func hasRule(violations []Violation, want Rule) bool {
	for _, violation := range violations {
		if violation.Rule == want {
			return true
		}
	}
	return false
}
