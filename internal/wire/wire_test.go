package wire

import (
	"encoding/json"
	"testing"
	"time"
)

func TestCodeExitStatus(t *testing.T) {
	tests := map[Code]int{
		CodeBadRequest:     2,
		CodeUnknownCommand: 2,
		CodeUnavailable:    3,
		CodeNotFound:       4,
		CodeConflict:       5,
		CodeSchemaMismatch: 6,
		CodeInternal:       1,
		Code("other"):      1,
	}
	for code, want := range tests {
		if got := code.ExitStatus(); got != want {
			t.Errorf("%q ExitStatus() = %d, want %d", code, got, want)
		}
	}
}

func TestParseKind(t *testing.T) {
	kinds := []Kind{
		KindPickup,
		KindComplete,
		KindLand,
		KindClose,
		KindReview,
		KindVerdict,
		KindRecovery,
		KindWarn,
		KindError,
	}
	for _, want := range kinds {
		got, ok := ParseKind(string(want))
		if !ok || got != want {
			t.Errorf("ParseKind(%q) = (%q, %t), want (%q, true)", want, got, ok, want)
		}
	}
	if got, ok := ParseKind("other"); ok || got != "" {
		t.Errorf("ParseKind(other) = (%q, %t), want (empty, false)", got, ok)
	}
}

func TestEnvelopeJSON(t *testing.T) {
	time := time.Date(2026, time.August, 28, 20, 0, 0, 0, time.UTC)
	event := Event{
		Schema: Schema,
		Seq:    4,
		Time:   time,
		Kind:   KindLand,
		Level:  "info",
	}
	data, err := json.Marshal(event)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"schema":1,"seq":4,"time":"2026-08-28T20:00:00Z","kind":"land","level":"info"}`
	if string(data) != want {
		t.Errorf("event JSON = %s, want %s", data, want)
	}

	request := Request{Schema: Schema, ID: "1", Command: "status"}
	data, err = json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(data), `{"schema":1,"id":"1","command":"status"}`; got != want {
		t.Errorf("request JSON = %s, want %s", got, want)
	}
}
