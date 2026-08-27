package decomp

import (
	"sort"
	"strings"
)

// Check returns deterministic decomposition violations for children.
func Check(children []Child) []Violation {
	if len(children) == 0 {
		return nil
	}

	ordered := append([]Child(nil), children...)
	sort.Slice(ordered, func(i, j int) bool {
		return ordered[i].ID < ordered[j].ID
	})

	violations := CheckGraph(ordered)
	for _, child := range ordered {
		violations = append(violations, CheckChild(child)...)
	}
	if len(violations) == 0 {
		return nil
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].ID == violations[j].ID {
			return violations[i].Rule < violations[j].Rule
		}
		return violations[i].ID < violations[j].ID
	})
	return violations
}

// Format renders violations as gate body text.
func Format(violations []Violation) string {
	if len(violations) == 0 {
		return ""
	}

	var text strings.Builder
	for _, violation := range violations {
		text.WriteString(violation.String())
		text.WriteByte('\n')
	}
	return text.String()
}
