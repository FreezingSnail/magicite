package wire

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type flushingBuffer struct {
	bytes.Buffer
	flushed bool
}

func (b *flushingBuffer) Flush() error {
	b.flushed = true
	return nil
}

func TestEncoderEncode(t *testing.T) {
	var output flushingBuffer
	encoder := NewEncoder(&output)
	request := Request{
		Schema:  Schema,
		ID:      "req-1",
		Command: "status",
		Params:  []byte(`{"verbose":true}`),
	}
	if err := encoder.Encode(request); err != nil {
		t.Fatal(err)
	}
	const want = "{\"schema\":1,\"id\":\"req-1\",\"command\":\"status\",\"params\":{\"verbose\":true}}\n"
	if got := output.String(); got != want {
		t.Errorf("encoded frame = %q, want %q", got, want)
	}
	if !output.flushed {
		t.Error("encoder did not flush writer")
	}
}

func TestDecoderRequest(t *testing.T) {
	decoder := NewDecoder(strings.NewReader("{\"schema\":1,\"id\":\"req-1\",\"command\":\"status\",\"params\":{\"verbose\":true}}\n"))
	request, err := decoder.Request()
	if err != nil {
		t.Fatal(err)
	}
	if request.Schema != Schema || request.ID != "req-1" || request.Command != "status" || string(request.Params) != `{"verbose":true}` {
		t.Errorf("decoded request = %#v", request)
	}
	if _, err := decoder.Request(); !errors.Is(err, io.EOF) {
		t.Errorf("second Request() error = %v, want io.EOF", err)
	}
}

func TestDecoderSchemaMismatch(t *testing.T) {
	decoder := NewDecoder(strings.NewReader("{\"schema\":2,\"id\":\"req-1\",\"command\":\"status\"}\n"))
	if _, err := decoder.Request(); !errors.Is(err, ErrSchema) {
		t.Errorf("Request() error = %v, want ErrSchema", err)
	}

	decoder = NewDecoder(strings.NewReader("{\"schema\":2,\"id\":\"req-1\"}\n"))
	if _, err := decoder.Frame(); !errors.Is(err, ErrSchema) {
		t.Errorf("Frame() error = %v, want ErrSchema", err)
	}
}

func TestDecoderFrame(t *testing.T) {
	input := strings.Join([]string{
		`{"schema":1,"id":"req-1","result":{"ok":true}}`,
		`{"schema":1,"seq":7,"time":"2026-08-28T20:00:00Z","kind":"land","level":"info"}`,
		"",
	}, "\n")
	decoder := NewDecoder(strings.NewReader(input))

	frame, err := decoder.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Response == nil || frame.Event != nil || frame.Response.ID != "req-1" || string(frame.Response.Result) != `{"ok":true}` {
		t.Errorf("response frame = %#v", frame)
	}

	frame, err = decoder.Frame()
	if err != nil {
		t.Fatal(err)
	}
	if frame.Event == nil || frame.Response != nil || frame.Event.Seq != 7 || frame.Event.Kind != KindLand {
		t.Errorf("event frame = %#v", frame)
	}
	if _, err := decoder.Frame(); !errors.Is(err, io.EOF) {
		t.Errorf("third Frame() error = %v, want io.EOF", err)
	}
}

func TestDecoderFrameWithoutIdentifierNamesLine(t *testing.T) {
	decoder := NewDecoder(strings.NewReader("{\"schema\":1}\n"))
	if _, err := decoder.Frame(); err == nil || !strings.Contains(err.Error(), "line 1") {
		t.Errorf("Frame() error = %v, want line number", err)
	}
}
