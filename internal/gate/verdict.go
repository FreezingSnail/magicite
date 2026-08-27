package gate

import "strings"

type VerdictKind int

const (
	VerdictUnparseable VerdictKind = iota
	VerdictApproved
	VerdictDrift
)

func (k VerdictKind) String() string {
	switch k {
	case VerdictApproved:
		return "approved"
	case VerdictDrift:
		return "drift"
	default:
		return "unparseable"
	}
}

type Verdict struct {
	Kind     VerdictKind
	Feedback string
}

const (
	MarkerApproved = "REVIEW: APPROVED"
	MarkerDrift    = "REVIEW: DRIFT"
)

func ParseVerdict(transcript string) Verdict {
	if strings.Contains(transcript, MarkerApproved) {
		return Verdict{Kind: VerdictApproved}
	}

	marker := strings.Index(transcript, MarkerDrift)
	if marker < 0 {
		return Verdict{Kind: VerdictUnparseable}
	}

	feedbackStart := marker + len(MarkerDrift)
	feedbackEnd := strings.IndexByte(transcript[feedbackStart:], '\n')
	if feedbackEnd < 0 {
		feedbackEnd = len(transcript)
	} else {
		feedbackEnd += feedbackStart
	}

	return Verdict{
		Kind:     VerdictDrift,
		Feedback: strings.TrimSpace(transcript[feedbackStart:feedbackEnd]),
	}
}
