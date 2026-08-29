package bd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestReadMethodsDecodeAndPreserveArgv(t *testing.T) {
	client := newFake(t,
		fakeEntry{Match: ArgsShow("one"), Stdout: `[{"id":"one"}]`},
		fakeEntry{Match: ArgsList(true), Stdout: `[]`},
		fakeEntry{Match: ArgsReady(), Stdout: `[{"id":"ready"}]`},
		fakeEntry{Match: ArgsQuery("status=open", true), Stdout: `[]`},
		fakeEntry{Match: ArgsDepList("one"), Stdout: `[{"id":"two"},{"id":"three"}]`},
		fakeEntry{Match: ArgsLabelList("one"), Stdout: `["first",{"name":"second"}]`},
	)

	bead, err := client.Show(context.Background(), "one")
	if err != nil || bead.ID != "one" {
		t.Fatalf("Show() = %#v, %v", bead, err)
	}
	list, err := client.List(context.Background(), true)
	if err != nil || list == nil || len(list) != 0 {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	ready, err := client.Ready(context.Background())
	if err != nil || len(ready) != 1 || ready[0].ID != "ready" {
		t.Fatalf("Ready() = %#v, %v", ready, err)
	}
	query, err := client.Query(context.Background(), "status=open", true)
	if err != nil || query == nil || len(query) != 0 {
		t.Fatalf("Query() = %#v, %v", query, err)
	}
	deps, err := client.Deps(context.Background(), "one")
	if err != nil || len(deps) != 2 {
		t.Fatalf("Deps() = %#v, %v", deps, err)
	}
	if got := []string{deps[0].ID, deps[1].ID}; !reflect.DeepEqual(got, []string{"two", "three"}) {
		t.Fatalf("Deps() = %#v, want order two, three", deps)
	}
	labels, err := client.Labels(context.Background(), "one")
	if err != nil || !reflect.DeepEqual(labels, []string{"first", "second"}) {
		t.Fatalf("Labels() = %#v, %v", labels, err)
	}

	want := [][]string{
		append([]string{"-C", client.Root}, ArgsShow("one")...),
		append([]string{"-C", client.Root}, ArgsList(true)...),
		append([]string{"-C", client.Root}, ArgsReady()...),
		append([]string{"-C", client.Root}, ArgsQuery("status=open", true)...),
		append([]string{"-C", client.Root}, ArgsDepList("one")...),
		append([]string{"-C", client.Root}, ArgsLabelList("one")...),
	}
	if got := fakeCalls(t, client); !reflect.DeepEqual(got, want) {
		t.Errorf("argv = %#v, want %#v", got, want)
	}
}

func TestReadSemanticAndMalformedFailures(t *testing.T) {
	t.Run("not found does not log", func(t *testing.T) {
		client := newFake(t, fakeEntry{Match: ArgsShow("missing"), Stdout: `{"error":"missing"}`, Exit: 1})
		var output bytes.Buffer
		client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})

		bead, err := client.Show(context.Background(), "missing")
		if bead.ID != "" || !IsNotFound(err) {
			t.Fatalf("Show() = %#v, %v", bead, err)
		}
		if output.Len() != 0 {
			t.Errorf("semantic no logged: %s", output.String())
		}
	})

	t.Run("malformed output logs once", func(t *testing.T) {

		t.Run("empty show is not found and does not log", func(t *testing.T) {
			client := newFake(t, fakeEntry{Match: ArgsShow("empty"), Stdout: `[]`})
			var output bytes.Buffer
			client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})

			bead, err := client.Show(context.Background(), "empty")
			if bead.ID != "" || !IsNotFound(err) {
				t.Fatalf("Show() = %#v, %v", bead, err)
			}
			if output.Len() != 0 {
				t.Errorf("empty show logged: %s", output.String())
			}
		})

		t.Run("classified failure logs once", func(t *testing.T) {
			client := newFake(t, fakeEntry{Match: ArgsLabelList("one"), Stderr: "Usage: bd label", Exit: 2})
			var output bytes.Buffer
			client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})

			_, err := client.Labels(context.Background(), "one")
			var classified *Error
			if !errors.As(err, &classified) || classified.Kind != KindUsage {
				t.Fatalf("Labels() error = %T %v", err, err)
			}
			assertReadLog(t, output.Bytes(), "label", 2, "usage")
		})
		client := newFake(t, fakeEntry{Match: ArgsList(false), Stdout: `{}`})
		var output bytes.Buffer
		client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})

		_, err := client.List(context.Background(), false)
		var classified *Error
		if !errors.As(err, &classified) || classified.Kind != KindFailed || !errors.Is(err, ErrMalformedOutput) {
			t.Fatalf("List() error = %T %v", err, err)
		}
		assertReadLog(t, output.Bytes(), "list", 0, "failed")
	})
}

func TestReadTransportFailureWrapsAndLogsOnce(t *testing.T) {
	client := newFake(t, fakeEntry{Match: ArgsReady(), DelayMS: 500})
	var output bytes.Buffer
	client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()

	_, err := client.Ready(ctx)
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUnavailable || !errors.Is(err, context.Canceled) {
		t.Fatalf("Ready() error = %T %v", err, err)
	}
	assertReadLog(t, output.Bytes(), "ready", classified.ExitCode, "unavailable")

	client.Program = filepath.Join(t.TempDir(), "missing-bd")
	_, err = client.Ready(context.Background())
	if !errors.As(err, &classified) || classified.Kind != KindFailed {
		t.Fatalf("Ready() spawn error = %T %v", err, err)
	}
	assertReadLogCount(t, output.Bytes(), 2)
}

func assertReadLog(t *testing.T, output []byte, op string, exit int, kind string) {
	t.Helper()
	var event struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &event); err != nil {
		t.Fatal(err)
	}
	if got := event.Fields; !reflect.DeepEqual(got["op"], op) || !reflect.DeepEqual(got["exit"], float64(exit)) || !reflect.DeepEqual(got["kind"], kind) || got["detail"] == "" {
		t.Errorf("log fields = %#v", got)
	}
	assertReadLogCount(t, output, 1)
}

func assertReadLogCount(t *testing.T, output []byte, want int) {
	t.Helper()
	if got := len(bytes.Split(bytes.TrimSpace(output), []byte{'\n'})); got != want {
		t.Errorf("log events = %d, want %d: %s", got, want, output)
	}
}
