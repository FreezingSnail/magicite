package logging

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

type panickingJSON struct{}

func (panickingJSON) MarshalJSON() ([]byte, error) {
	panic("bad marshaler")
}

func TestEventJSONRendersMalformedValuesOnOneLine(t *testing.T) {
	var output bytes.Buffer
	Configure(Config{Level: Debug, Format: JSON, Writer: &output})

	Event(Info, "land\ncomplete", map[string]any{
		"bad":   make(chan int),
		"panic": panickingJSON{},
		"text":  "line one\nline two",
	})

	line := output.String()
	if strings.Count(line, "\n") != 1 {
		t.Fatalf("line count = %q, want one line", line)
	}

	var record struct {
		Sequence uint64         `json:"sequence"`
		Level    string         `json:"level"`
		Kind     string         `json:"kind"`
		Fields   map[string]any `json:"fields"`
	}
	if err := json.Unmarshal([]byte(line), &record); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, line)
	}
	if record.Sequence == 0 || record.Level != "info" || record.Kind != "land\ncomplete" {
		t.Fatalf("record = %+v", record)
	}
	for _, key := range []string{"bad", "panic"} {
		if value, ok := record.Fields[key].(string); !ok || value == "" {
			t.Errorf("malformed field %q = %#v, want rendered string", key, record.Fields[key])
		}
	}
}

func TestEventThresholdAndTextFormat(t *testing.T) {
	var output bytes.Buffer
	Configure(Config{Level: Warn, Format: Text, Writer: &output})

	Event(Debug, "debug", nil)
	Event(Info, "info", nil)
	Event(Warn, "warn", map[string]any{"message": "watch\nout"})
	Event(Error, "error", nil)

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("records = %d, want 2: %q", len(lines), output.String())
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "sequence=") || strings.Contains(line, "\n") {
			t.Errorf("text line = %q", line)
		}
	}
	if strings.Contains(output.String(), "kind=\"debug\"") || strings.Contains(output.String(), "kind=\"info\"") {
		t.Errorf("threshold failed: %q", output.String())
	}
}

func TestEventOrdering(t *testing.T) {
	var output bytes.Buffer
	Configure(Config{Level: Debug, Format: JSON, Writer: &output})

	Event(Info, "one", nil)
	Event(Info, "two", nil)

	var previous uint64
	for _, line := range strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n") {
		var record struct {
			Sequence uint64 `json:"sequence"`
		}
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		if record.Sequence <= previous {
			t.Fatalf("sequence %d after %d", record.Sequence, previous)
		}
		previous = record.Sequence
	}
}
