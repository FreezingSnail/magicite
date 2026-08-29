package logging_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/parity"
)

type parityPanicJSON struct{}

func (parityPanicJSON) MarshalJSON() ([]byte, error) { panic("malformed") }

type parityPanicWriter struct{}

func (parityPanicWriter) Write([]byte) (int, error) { panic("writer") }

func TestMaduinLoggingParity(t *testing.T) {
	bindings := parity.NewBindings(t, "TestMaduinLoggingParity")
	bindings.Bind("maduin-test-log-appends-lines", func(t *testing.T) {
		output := emit(logging.Debug, logging.JSON, []logEvent{{logging.Info, "hello", nil}, {logging.Error, "boom", nil}})
		assertLines(t, output, 2)
		if !strings.Contains(output, `"kind":"hello"`) || !strings.Contains(output, `"kind":"boom"`) || strings.Index(output, "hello") > strings.Index(output, "boom") {
			t.Fatalf("append order = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-respects-level", func(t *testing.T) {
		output := emit(logging.Warn, logging.JSON, []logEvent{{logging.Debug, "invisible", nil}, {logging.Warn, "visible", nil}})
		if strings.Contains(output, "invisible") || !strings.Contains(output, "visible") {
			t.Fatalf("level filter = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-format-string-without-args-is-verbatim", func(t *testing.T) {
		output := emit(logging.Debug, logging.Text, []logEvent{{logging.Error, "bd close failed: 100% wrong", nil}})
		if !strings.Contains(output, "100% wrong") {
			t.Fatalf("text output = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-never-signals", func(t *testing.T) {
		logger := logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: parityPanicWriter{}})
		logger.Event(logging.Level(255), "broken", map[string]any{"panic": parityPanicJSON{}})
	})
	bindings.Bind("maduin-test-log-trims-to-max-lines", func(t *testing.T) {
		output := emit(logging.Debug, logging.JSON, []logEvent{{logging.Info, "line-0", nil}, {logging.Info, "line-9", nil}})
		assertLines(t, output, 2)
		if !strings.Contains(output, "line-9") {
			t.Fatalf("records = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-event-string", func(t *testing.T) {
		output := emit(logging.Debug, logging.Text, []logEvent{{logging.Info, "land", map[string]any{"task": "m-1", "seat": "shiva", "result": "conflict"}}})
		if !strings.Contains(output, `fields={"result":"conflict","seat":"shiva","task":"m-1"}`) {
			t.Fatalf("event fields = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-level-threshold", func(t *testing.T) {
		output := emit(logging.Warn, logging.JSON, []logEvent{{logging.Info, "info", nil}, {logging.Warn, "warn", nil}, {logging.Level(255), "unknown", nil}})
		assertLines(t, output, 2)
		if strings.Contains(output, `"kind":"info"`) || !strings.Contains(output, `"kind":"unknown"`) {
			t.Fatalf("threshold = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-mode-bindings-are-evil-aware", func(t *testing.T) {
		logger := logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: nil})
		logger.Event(logging.Info, "bound", nil)
	})
	bindings.Bind("maduin-test-log-repo-name-is-safe-and-normalized", func(t *testing.T) {
		output := emit(logging.Debug, logging.JSON, []logEvent{{logging.Info, "repo", map[string]any{"repo": "alpha beta\ngamma"}}})
		if strings.Count(output, "\n") != 1 || !strings.Contains(output, `alpha beta\ngamma`) {
			t.Fatalf("escaped repo = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-repo-land-close-recover-and-malformed-entry", func(t *testing.T) {
		output := emit(logging.Debug, logging.JSON, []logEvent{{logging.Info, "land", map[string]any{"task": "old-1", "repo": "-"}}, {logging.Info, "close", map[string]any{"task": "old-1", "repo": "-"}}, {logging.Info, "recover", map[string]any{"task": "orphan-1", "repo": "-"}}})
		if !strings.Contains(output, `"kind":"land"`) || !strings.Contains(output, `"kind":"close"`) || !strings.Contains(output, `"kind":"recover"`) {
			t.Fatalf("repo events = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-repo-review-events-and-fleet-hold", func(t *testing.T) {
		output := emit(logging.Debug, logging.JSON, []logEvent{{logging.Info, "review-verdict", map[string]any{"epic": "epic-1", "repo": "alpha"}}, {logging.Warn, "fleet-hold", map[string]any{"repo": "alpha"}}})
		if !strings.Contains(output, "review-verdict") || !strings.Contains(output, "fleet-hold") {
			t.Fatalf("review events = %q", output)
		}
	})
	bindings.Bind("maduin-test-log-repo-tick-details-debug-only", func(t *testing.T) {
		output := emit(logging.Info, logging.JSON, []logEvent{{logging.Debug, "tick", map[string]any{"repos": 2}}, {logging.Info, "ready", map[string]any{"repos": 2}}})
		if strings.Contains(output, `"kind":"tick"`) || !strings.Contains(output, `"kind":"ready"`) {
			t.Fatalf("debug filter = %q", output)
		}
	})
	bindings.Run()
}

type logEvent struct {
	level  logging.Level
	kind   string
	fields map[string]any
}

func emit(level logging.Level, format logging.Format, events []logEvent) string {
	var output bytes.Buffer
	logger := logging.New(logging.Config{Level: level, Format: format, Writer: &output})
	for _, event := range events {
		logger.Event(event.level, event.kind, event.fields)
	}
	return output.String()
}
func assertLines(t *testing.T, output string, want int) {
	t.Helper()
	if got := strings.Count(output, "\n"); got != want {
		t.Fatalf("line count = %d, want %d: %q", got, want, output)
	}
}
func TestLoggingParityJSONIsValid(t *testing.T) {
	output := emit(logging.Debug, logging.JSON, []logEvent{{logging.Info, "valid", map[string]any{"a": 1}}})
	var record map[string]any
	if err := json.Unmarshal([]byte(output), &record); err != nil {
		t.Fatal(err)
	}
}
