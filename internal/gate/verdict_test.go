package gate

import (
	"strings"
	"testing"
)

func TestParseVerdictApprovedWins(t *testing.T) {
	got := ParseVerdict("REVIEW: DRIFT: stale\nREVIEW: APPROVED")
	want := Verdict{Kind: VerdictApproved}
	if got != want {
		t.Fatalf("ParseVerdict() = %#v, want %#v", got, want)
	}
}

func TestParseVerdictDriftFeedbackEndsAtLineFeed(t *testing.T) {
	got := ParseVerdict("prefix REVIEW: DRIFT: revise API \r\nignored line")
	want := Verdict{Kind: VerdictDrift, Feedback: ": revise API"}
	if got != want {
		t.Fatalf("ParseVerdict() = %#v, want %#v", got, want)
	}
}

func TestParseVerdictDriftFeedbackAtEnd(t *testing.T) {
	for _, transcript := range []string{
		"REVIEW: DRIFT: revise the API",
		"REVIEW: DRIFT: \t \r",
		"REVIEW: DRIFT",
	} {
		got := ParseVerdict(transcript)
		if got.Kind != VerdictDrift {
			t.Fatalf("ParseVerdict(%q).Kind = %v, want drift", transcript, got.Kind)
		}
		if transcript == "REVIEW: DRIFT" && got.Feedback != "" {
			t.Fatalf("ParseVerdict(%q).Feedback = %q, want empty", transcript, got.Feedback)
		}
	}
}

func TestParseVerdictMarkerSplitAcrossChunks(t *testing.T) {
	left := "reviewer output REVIEW: APPRO"
	right := "VED"
	if got := ParseVerdict(left + right); got.Kind != VerdictApproved {
		t.Fatalf("joined chunks parsed as %v, want approved", got.Kind)
	}
	if got := ParseVerdict(left); got.Kind != VerdictUnparseable {
		t.Fatalf("partial chunk parsed as %v, want unparseable", got.Kind)
	}
}

func TestParseVerdictUnparseableTranscripts(t *testing.T) {
	for _, transcript := range []string{"", " \t\r\n ", "review and approved", "REVIEW: approve", "REVIEW: DRIF"} {
		got := ParseVerdict(transcript)
		want := Verdict{Kind: VerdictUnparseable}
		if got != want {
			t.Fatalf("ParseVerdict(%q) = %#v, want %#v", transcript, got, want)
		}
	}
}

func TestParseVerdictLargeTranscript(t *testing.T) {
	transcript := strings.Repeat("output\n", 2*1024*1024) + "REVIEW: DRIFT: final issue\n"
	got := ParseVerdict(transcript)
	want := Verdict{Kind: VerdictDrift, Feedback: ": final issue"}
	if got != want {
		t.Fatalf("ParseVerdict() = %#v, want %#v", got, want)
	}
}

func TestVerdictKindString(t *testing.T) {
	for kind, want := range map[VerdictKind]string{
		VerdictUnparseable: "unparseable",
		VerdictApproved:    "approved",
		VerdictDrift:       "drift",
		VerdictKind(99):    "unparseable",
	} {
		if got := kind.String(); got != want {
			t.Fatalf("VerdictKind(%d).String() = %q, want %q", kind, got, want)
		}
	}
}
