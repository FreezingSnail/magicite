package decomp

import (
	"fmt"
	"reflect"
	"testing"
)

func TestRecordedVerdict(t *testing.T) {
	for name, test := range map[string]struct {
		children []Child
		want     bool
	}{
		"field":       {[]Child{{DecompVerdict: true}}, true},
		"label":       {[]Child{{Labels: []string{"decomp-verdict:approved"}}}, true},
		"slash":       {[]Child{{Labels: []string{"decomp-verdict/review"}}}, true},
		"exact":       {[]Child{{Labels: []string{"decomp-verdict"}}}, true},
		"prefix only": {[]Child{{Labels: []string{"decomp-verdicts"}}}, false},
		"empty":       {nil, false},
	} {
		t.Run(name, func(t *testing.T) {
			if got := RecordedVerdict(test.children); got != test.want {
				t.Fatalf("RecordedVerdict(%#v) = %t, want %t", test.children, got, test.want)
			}
		})
	}
}

func TestFind(t *testing.T) {
	children := []Child{{ID: "first"}, {ID: "second"}}
	if got, ok := Find(children, "second"); !ok || got.ID != "second" {
		t.Fatalf("Find() = %#v, %t, want second child, true", got, ok)
	}
	if got, ok := Find(children, "missing"); ok || !reflect.DeepEqual(got, Child{}) {
		t.Fatalf("Find() = %#v, %t, want zero child, false", got, ok)
	}
}

func TestAncestorProvidesTraversesAndTerminates(t *testing.T) {
	children := []Child{
		{ID: "consumer", Deps: []string{"middle", "outside"}},
		{ID: "middle", Description: graphDescription("middle.symbol, shared", ""), Deps: []string{"root"}},
		{ID: "root", Description: graphDescription("root.symbol, shared", ""), Deps: []string{"middle"}},
	}
	if got, want := AncestorProvides(children, children[0]), []string{"middle.symbol", "shared", "root.symbol"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("AncestorProvides() = %#v, want %#v", got, want)
	}
}

func TestCycleDepthWavesAndMedian(t *testing.T) {
	children := []Child{
		{ID: "root"},
		{ID: "left", Deps: []string{"root"}},
		{ID: "right", Deps: []string{"root"}},
		{ID: "tip", Deps: []string{"left", "outside"}},
	}
	if InCycle(children, children[3]) {
		t.Fatal("InCycle() reported acyclic child")
	}
	if got, want := Depth(children, children[0]), 0; got != want {
		t.Fatalf("Depth(root) = %d, want %d", got, want)
	}
	if got, want := Depth(children, children[3]), 2; got != want {
		t.Fatalf("Depth(tip) = %d, want %d", got, want)
	}
	if got, want := Waves(children), []int{1, 2, 1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Waves() = %#v, want %#v", got, want)
	}

	cycle := []Child{{ID: "a", Deps: []string{"b"}}, {ID: "b", Deps: []string{"a"}}, {ID: "outside", Deps: []string{"missing"}}}
	for _, child := range cycle[:2] {
		if !InCycle(cycle, child) {
			t.Fatalf("InCycle(%q) = false, want true", child.ID)
		}
		_ = Depth(cycle, child)
	}
	if InCycle(cycle, cycle[2]) {
		t.Fatal("InCycle() treated unresolved dependency as a cycle")
	}

	values := []int{4, 1, 3, 2}
	if got, want := Median(values), 2.5; got != want {
		t.Fatalf("Median() = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(values, []int{4, 1, 3, 2}) {
		t.Fatalf("Median() mutated input: %#v", values)
	}
	if got := Median(nil); got != 0 {
		t.Fatalf("Median(nil) = %v, want 0", got)
	}
}

func TestCheckGraph(t *testing.T) {
	t.Run("count budget", func(t *testing.T) {
		many := graphChildren(13)
		got := CheckGraph(many)
		want := []Violation{{ID: "child-00", Rule: RuleCountBudget, Detail: "child count 13 exceeds maximum 12"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("CheckGraph() = %#v, want %#v", got, want)
		}

		few := graphChildren(7)
		got = CheckGraph(few)
		want = []Violation{{ID: "child-00", Rule: RuleCountBudget, Detail: "child count 7 is below minimum 8; no decomp verdict recorded"}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("CheckGraph() = %#v, want %#v", got, want)
		}
		few[0].Labels = []string{"decomp-verdict:approved"}
		if got = CheckGraph(few); len(got) != 0 {
			t.Fatalf("CheckGraph() with verdict = %#v, want none", got)
		}
	})

	t.Run("transitive symbols", func(t *testing.T) {
		children := graphChildren(8)
		children[0] = Child{ID: "consumer", Description: graphDescription("", "root.symbol, missing.symbol"), Deps: []string{"middle"}}
		children[1] = Child{ID: "middle", Deps: []string{"root"}}
		children[2] = Child{ID: "root", Description: graphDescription("root.symbol", "")}
		for i := 3; i < 5; i++ {
			children[i].Deps = []string{"root"}
		}
		for i := 5; i < len(children); i++ {
			children[i].Deps = []string{"middle"}
		}
		want := []Violation{{ID: "consumer", Rule: RuleUnprovidedSymbol, Detail: "no dependency provides missing.symbol"}}
		if got := CheckGraph(children); !reflect.DeepEqual(got, want) {
			t.Fatalf("CheckGraph() = %#v, want %#v", got, want)
		}
	})

	t.Run("cycles suppress width", func(t *testing.T) {
		children := graphChildren(8)
		children[0] = Child{ID: "a", Deps: []string{"b"}}
		children[1] = Child{ID: "b", Deps: []string{"a"}}
		want := []Violation{
			{ID: "a", Rule: RuleGraphCycle, Detail: "child a is in a dependency cycle"},
			{ID: "b", Rule: RuleGraphCycle, Detail: "child b is in a dependency cycle"},
		}
		if got := CheckGraph(children); !reflect.DeepEqual(got, want) {
			t.Fatalf("CheckGraph() = %#v, want %#v", got, want)
		}
	})

	t.Run("narrow waves", func(t *testing.T) {
		children := graphChildren(8)
		for i := 1; i < len(children); i++ {
			children[i].Deps = []string{children[i-1].ID}
		}
		want := []Violation{{ID: "child-00", Rule: RuleGraphWidth, Detail: "median wave width 1 is below concurrency cap 3: [1 1 1 1 1 1 1 1]"}}
		if got := CheckGraph(children); !reflect.DeepEqual(got, want) {
			t.Fatalf("CheckGraph() = %#v, want %#v", got, want)
		}
	})

	if got := CheckGraph(nil); len(got) != 0 {
		t.Fatalf("CheckGraph(nil) = %#v, want none", got)
	}
}

func graphChildren(count int) []Child {
	children := make([]Child, count)
	for i := range children {
		children[i].ID = fmt.Sprintf("child-%02d", i)
	}
	return children
}

func graphDescription(provides, consumes string) string {
	return fmt.Sprintf("# Scope\nx\n# Files\nx\n# Contract\nx\n# Invariants\nx\n# Non-goals\nx\n# MACHINE\nprovides: %s\nconsumes: %s\nfiles: x\ntier: low", provides, consumes)
}
