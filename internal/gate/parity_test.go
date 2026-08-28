package gate

import (
	"sort"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/parity"
)

func TestMaduinReviewParity(t *testing.T) {
	for _, name := range reviewParityNames() {
		t.Run(name, func(t *testing.T) {
			approved := ParseVerdict("noise\nREVIEW: APPROVED\n")
			if approved.Kind != VerdictApproved {
				t.Fatalf("approved verdict = %#v", approved)
			}
			drift := ParseVerdict("REVIEW: DRIFT: revise\n")
			if drift.Kind != VerdictDrift || drift.Feedback != ": revise" {
				t.Fatalf("drift verdict = %#v", drift)
			}
			if got := ParseVerdict("no marker"); got.Kind != VerdictUnparseable {
				t.Fatalf("unparseable verdict = %#v", got)
			}
		})
	}
}

func reviewParityNames() []string {
	counterparts := parity.OrchestrationCounterparts()
	names := make([]string, 0)
	for name, testName := range counterparts {
		if strings.HasPrefix(testName, "TestMaduinReviewParity/") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
