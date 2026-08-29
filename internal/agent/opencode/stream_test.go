package opencode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/FreezingSnail/magicite/internal/agent"
)

func TestParseLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want Event
		ok   bool
	}{
		{name: "completed", line: `{"type":"step_finish","sessionID":"ses_1","part":{"reason":"stop"}}`, want: Event{Type: "step_finish", SessionID: "ses_1", Terminal: agent.StatusCompleted}, ok: true},
		{name: "failed step", line: `{"type":"step_finish","part":{"reason":"error"}}`, want: Event{Type: "step_finish", Terminal: agent.StatusFailed}, ok: true},
		{name: "failed tool", line: `{"type":"tool_use","part":{"state":{"status":"error"}}}`, want: Event{Type: "tool_use", Terminal: agent.StatusFailed}, ok: true},
		{name: "nonterminal", line: `{"type":"step_start","properties":{"sessionID":"ses_2"}}`, want: Event{Type: "step_start", SessionID: "ses_2"}, ok: true},
		{name: "blank", line: " \t", ok: false},
		{name: "invalid", line: `{`, ok: false},
		{name: "missing type", line: `{"sessionID":"ses_3"}`, ok: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ParseLine(test.line)
			if ok != test.ok || got != test.want {
				t.Fatalf("ParseLine(%q) = (%+v, %t), want (%+v, %t)", test.line, got, ok, test.want, test.ok)
			}
		})
	}
}

func TestScannerFixtures(t *testing.T) {
	tests := []struct {
		file      string
		sessionID string
		status    agent.Status
		limited   bool
	}{
		{file: "stream_completed.ndjson", sessionID: "ses_completed", status: agent.StatusCompleted},
		{file: "stream_denied.ndjson", sessionID: "ses_denied", status: agent.StatusFailed},
		{file: "stream_limited.ndjson", sessionID: "ses_limited", status: agent.StatusRunning, limited: true},
	}

	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			stream := fixture(t, test.file)
			scanner := NewScanner()
			if written, err := scanner.Write([]byte(stream)); err != nil || written != len(stream) {
				t.Fatalf("Write() = (%d, %v), want (%d, nil)", written, err, len(stream))
			}
			scanner.Flush()
			if got := scanner.SessionID(); got != test.sessionID {
				t.Errorf("SessionID() = %q, want %q", got, test.sessionID)
			}
			if got := scanner.Status(); got != test.status {
				t.Errorf("Status() = %q, want %q", got, test.status)
			}
			if got := scanner.Limited(); got != test.limited {
				t.Errorf("Limited() = %t, want %t", got, test.limited)
			}
			if got := scanner.Transcript(); got != stream {
				t.Errorf("Transcript() = %q, want %q", got, stream)
			}
		})
	}
}

func TestScannerChunkIndependenceAndLatches(t *testing.T) {
	stream := `{"type":"step_start","sessionID":"ses_first"}` + "\n" +
		`{"type":"step_finish","sessionID":"ses_second","part":{"reason":"error"}}` + "\n" +
		`{"type":"step_finish","part":{"reason":"stop"}}`

	whole := NewScanner()
	_, _ = whole.Write([]byte(stream))
	whole.Flush()

	chunked := NewScanner()
	for i := range stream {
		_, _ = chunked.Write([]byte(stream[i : i+1]))
	}
	chunked.Flush()

	if got, want := chunked.SessionID(), whole.SessionID(); got != want || got != "ses_first" {
		t.Errorf("SessionID() = %q, want first ID %q", got, want)
	}
	if got, want := chunked.Status(), whole.Status(); got != want || got != agent.StatusFailed {
		t.Errorf("Status() = %q, want latched failure %q", got, want)
	}
	if got, want := chunked.Limited(), whole.Limited(); got != want {
		t.Errorf("Limited() = %t, want %t", got, want)
	}
	if got, want := chunked.Transcript(), whole.Transcript(); got != want || got != stream {
		t.Errorf("Transcript() = %q, want %q", got, want)
	}
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
