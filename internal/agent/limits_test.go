package agent

import "testing"

func TestLimitedTailPhrases(t *testing.T) {
	for _, phrase := range []string{
		"usage limit",
		"usage-limit reached",
		"usage exceeded",
		"rate limit",
		"rate-limit exceeded",
		"too-many-requests reached",
		"too many requests",
		"credit-limit-reached",
		"429",
		"quota reached",
		"insufficient credits",
		"credit limit",
		"credits-exceeded",
	} {
		if !LimitedTail("provider: " + phrase) {
			t.Errorf("LimitedTail(%q) = false", phrase)
		}
	}
}

func TestLimitedLineRequiresErrorCarrier(t *testing.T) {
	for _, line := range []string{
		`{"type":"message","text":"quota exceeded"}`,
		`{"type":"message","text":"429"}`,
		`{"type":"message","text":"rate limit"}`,
	} {
		if LimitedLine(line) {
			t.Errorf("LimitedLine(%q) = true, want false", line)
		}
	}

	for _, line := range []string{
		`{"error":"rate limit reached"}`,
		`{"type":"session.error","message":"quota"}`,
		`{"event":{"error":"429"}}`,
	} {
		if !LimitedLine(line) {
			t.Errorf("LimitedLine(%q) = false, want true", line)
		}
	}
}

func TestFailureTailAuthenticationAndBoundaries(t *testing.T) {
	for _, phrase := range []string{
		"auth",
		"authentication failed",
		"authorization denied",
		"unauthorized",
		"not authenticated",
		"invalid api key",
	} {
		if !FailureTail("provider: " + phrase) {
			t.Errorf("FailureTail(%q) = false", phrase)
		}
	}
	for _, output := range []string{"tokenizer", "quotative", "oauthentication", "invalid api keys"} {
		if FailureTail(output) {
			t.Errorf("FailureTail(%q) = true, want false", output)
		}
	}
	if !FailureTail("usage limit reached") || !FailureTail("RATE-LIMIT") {
		t.Error("FailureTail must include all limited tails")
	}
}

func TestTailWindowIsByteBoundedAndUTF8Safe(t *testing.T) {
	if LimitedTail("quota " + string(make([]byte, TailWindow+1))) {
		t.Error("invalid prefix must not move quota into tail")
	}
	if !LimitedTail(string(make([]byte, TailWindow-1)) + "é quota") {
		t.Error("UTF-8 tail with quota = false, want true")
	}
	if LimitedTail("quota" + string(make([]byte, TailWindow))) {
		t.Error("phrase outside tail = true, want false")
	}
}

func TestClassifiersAcceptEmptyInput(t *testing.T) {
	if LimitedLine("") || LimitedTail("") || FailureTail("") {
		t.Error("empty input classified as failure")
	}
}
