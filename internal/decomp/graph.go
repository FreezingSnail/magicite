package decomp

import (
	"fmt"
	"sort"
	"strings"
)

const (
	MinChildren      = 8
	MaxChildren      = 12
	FleetConcurrency = 3
)

func RecordedVerdict(children []Child) bool {
	for _, child := range children {
		if child.DecompVerdict {
			return true
		}
		for _, label := range child.Labels {
			if label == "decomp-verdict" || strings.HasPrefix(label, "decomp-verdict:") || strings.HasPrefix(label, "decomp-verdict/") {
				return true
			}
		}
	}
	return false
}

func Find(children []Child, id string) (Child, bool) {
	for _, child := range children {
		if child.ID == id {
			return child, true
		}
	}
	return Child{}, false
}

func AncestorProvides(children []Child, child Child) []string {
	seenChildren := make(map[string]bool)
	seenSymbols := make(map[string]bool)
	var provides []string

	var visit func(string)
	visit = func(id string) {
		if seenChildren[id] {
			return
		}
		ancestor, ok := Find(children, id)
		if !ok {
			return
		}
		seenChildren[id] = true
		for _, symbol := range Parse(ancestor.Description).Provides {
			if !seenSymbols[symbol] {
				seenSymbols[symbol] = true
				provides = append(provides, symbol)
			}
		}
		for _, dependency := range ancestor.Deps {
			visit(dependency)
		}
	}

	for _, dependency := range child.Deps {
		visit(dependency)
	}
	return provides
}

func InCycle(children []Child, child Child) bool {
	visited := make(map[string]bool)
	var reachesChild func(string) bool
	reachesChild = func(id string) bool {
		ancestor, ok := Find(children, id)
		if !ok {
			return false
		}
		if ancestor.ID == child.ID {
			return true
		}
		if visited[id] {
			return false
		}
		visited[id] = true
		for _, dependency := range ancestor.Deps {
			if reachesChild(dependency) {
				return true
			}
		}
		return false
	}
	for _, dependency := range child.Deps {
		if reachesChild(dependency) {
			return true
		}
	}
	return false
}

func Depth(children []Child, child Child) int {
	visiting := make(map[string]bool)
	var depth func(Child) int
	depth = func(current Child) int {
		if visiting[current.ID] {
			return 0
		}
		visiting[current.ID] = true
		defer delete(visiting, current.ID)

		resolvable := false
		maximum := 0
		for _, id := range current.Deps {
			dependency, ok := Find(children, id)
			if !ok {
				continue
			}
			resolvable = true
			if value := depth(dependency); value > maximum {
				maximum = value
			}
		}
		if !resolvable {
			return 0
		}
		return maximum + 1
	}
	return depth(child)
}

func Waves(children []Child) []int {
	var waves []int
	for _, child := range children {
		depth := Depth(children, child)
		for len(waves) <= depth {
			waves = append(waves, 0)
		}
		waves[depth]++
	}
	return waves
}

func Median(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]int(nil), values...)
	sort.Ints(ordered)
	middle := len(ordered) / 2
	if len(ordered)%2 != 0 {
		return float64(ordered[middle])
	}
	return float64(ordered[middle-1])/2 + float64(ordered[middle])/2
}

func CheckGraph(children []Child) []Violation {
	var violations []Violation
	lowest, hasChildren := lowestID(children)
	if hasChildren {
		switch {
		case len(children) > MaxChildren:
			violations = append(violations, Violation{ID: lowest.ID, Rule: RuleCountBudget, Detail: fmt.Sprintf("child count %d exceeds maximum %d", len(children), MaxChildren)})
		case len(children) < MinChildren && !RecordedVerdict(children):
			violations = append(violations, Violation{ID: lowest.ID, Rule: RuleCountBudget, Detail: fmt.Sprintf("child count %d is below minimum %d; no decomp verdict recorded", len(children), MinChildren)})
		}
	}

	for _, child := range children {
		provided := symbols(AncestorProvides(children, child))
		for _, symbol := range Parse(child.Description).Consumes {
			if !provided[symbol] {
				violations = append(violations, Violation{ID: child.ID, Rule: RuleUnprovidedSymbol, Detail: "no dependency provides " + symbol})
			}
		}
	}

	cyclic := false
	for _, child := range children {
		if InCycle(children, child) {
			cyclic = true
			violations = append(violations, Violation{ID: child.ID, Rule: RuleGraphCycle, Detail: "child " + child.ID + " is in a dependency cycle"})
		}
	}
	if hasChildren && !cyclic {
		waves := Waves(children)
		median := Median(waves)
		if median < FleetConcurrency {
			violations = append(violations, Violation{ID: lowest.ID, Rule: RuleGraphWidth, Detail: fmt.Sprintf("median wave width %g is below concurrency cap %d: %v", median, FleetConcurrency, waves)})
		}
	}
	return violations
}

func lowestID(children []Child) (Child, bool) {
	if len(children) == 0 {
		return Child{}, false
	}
	lowest := children[0]
	for _, child := range children[1:] {
		if child.ID < lowest.ID {
			lowest = child
		}
	}
	return lowest, true
}

func symbols(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}
