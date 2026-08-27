package decomp

import (
	"fmt"
	"strings"
)

const (
	MaxProseWords      = 400
	MaxAcceptanceWords = 80
	MaxDesignWords     = 150
)

var interimPhrases = []string{"until then", "placeholder", "for now pass nil"}

func InterimPhrases() []string {
	return append([]string(nil), interimPhrases...)
}

func DifficultyLabels(c Child) []string {
	labels := make([]string, 0, len(c.Labels))
	for _, label := range c.Labels {
		if label == "difficulty:low" || label == "difficulty:high" {
			labels = append(labels, label)
		}
	}
	return labels
}

func CheckChild(c Child) []Violation {
	contract := Parse(c.Description)
	derived := DeriveTier(contract)
	labels := DifficultyLabels(c)
	var violations []Violation

	add := func(rule Rule, detail string) {
		violations = append(violations, Violation{ID: c.ID, Rule: rule, Detail: detail})
	}

	if detail := SchemaDetail(c.Description); detail != "" {
		add(RuleSchemaSection, detail)
	}
	if prose := ProseWords(c.Description); prose > MaxProseWords {
		add(RuleCapProse, fmt.Sprintf("words=%d maximum=%d", prose, MaxProseWords))
	}

	acceptanceWords := Words(c.Acceptance)
	if acceptanceWords > MaxAcceptanceWords || !plainASCIIOneLine(c.Acceptance) {
		if acceptanceWords > MaxAcceptanceWords {
			add(RuleCapAcceptance, fmt.Sprintf("words=%d maximum=%d", acceptanceWords, MaxAcceptanceWords))
		} else {
			add(RuleCapAcceptance, fmt.Sprintf("not plain ASCII one line: %q", c.Acceptance))
		}
	}

	designWords := Words(c.Design)
	if derived == TierLow && designWords > MaxDesignWords {
		add(RuleCapDesign, fmt.Sprintf("words=%d maximum=%d", designWords, MaxDesignWords))
	}

	wantLabel := "difficulty:" + string(derived)
	if len(labels) != 1 || labels[0] != wantLabel {
		add(RuleTierMismatch, fmt.Sprintf("derived=%s labels=%v", derived, labels))
	}

	text := strings.ToLower(strings.Join([]string{c.Description, c.Design, c.Acceptance}, "\n"))
	for _, phrase := range interimPhrases {
		if strings.Contains(text, phrase) {
			add(RuleInterimValue, "matched="+phrase)
			break
		}
	}

	return violations
}

func plainASCIIOneLine(text string) bool {
	for i := 0; i < len(text); i++ {
		if text[i] < ' ' || text[i] > '~' {
			return false
		}
	}
	return true
}
