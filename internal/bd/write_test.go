package bd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/logging"
)

func TestCreateWritesTextToPrivateFilesAndReturnsID(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"create"}, Stdout: " bd-42\n"})
	id, err := client.Create(context.Background(), CreateRequest{
		Title: "title", Type: "bug", Parent: "bd-1", Body: "body\ntext", Design: "design\ntext",
		Acceptance: "done", Labels: []string{"a", "b"}, Priority: "1",
	})
	if err != nil || id != "bd-42" {
		t.Fatalf("Create() = %q, %v", id, err)
	}
	calls := fakeCalls(t, client)
	if len(calls) != 1 {
		t.Fatalf("calls = %#v", calls)
	}
	args := calls[0][2:]
	if got, want := args[:4], []string{"create", "title", "--type", "bug"}; !reflect.DeepEqual(got, want) {
		t.Errorf("create prefix = %#v, want %#v", got, want)
	}
	assertFileArgument(t, args, "--body-file", "body\ntext")
	assertFileArgument(t, args, "--design-file", "design\ntext")
	if strings.Contains(strings.Join(args, "\x00"), "body\ntext") || strings.Contains(strings.Join(args, "\x00"), "design\ntext") {
		t.Errorf("text reached argv: %#v", args)
	}
}

func TestCreateRejectsEmptyTitleBeforeSpawning(t *testing.T) {
	client := newFake(t)
	_, err := client.Create(context.Background(), CreateRequest{})
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUsage {
		t.Fatalf("Create() error = %T %v, want usage *Error", err, err)
	}
	if calls := fakeCalls(t, client); len(calls) != 0 {
		t.Errorf("calls = %#v, want none", calls)
	}
}

func TestCreateRejectsEmptyOrMultilineID(t *testing.T) {
	for _, output := range []string{" \n", "bd-1\nbd-2"} {
		t.Run(strings.ReplaceAll(output, "\n", "_"), func(t *testing.T) {
			client := newFake(t, fakeEntry{Match: []string{"create"}, Stdout: output})
			_, err := client.Create(context.Background(), CreateRequest{Title: "title"})
			var classified *Error
			if !errors.As(err, &classified) || classified.Kind != KindFailed {
				t.Fatalf("Create() error = %T %v, want failed *Error", err, err)
			}
		})
	}
}

func TestUpdateFileTextAndEmptyNoop(t *testing.T) {
	client := newFake(t, fakeEntry{Match: []string{"update", "bd-1"}})
	if err := client.Update(context.Background(), "bd-1", UpdateRequest{}); err != nil {
		t.Fatalf("empty Update() error = %v", err)
	}
	if calls := fakeCalls(t, client); len(calls) != 0 {
		t.Fatalf("empty Update calls = %#v", calls)
	}
	if err := client.Update(context.Background(), "bd-1", UpdateRequest{Body: "body\ntext", Design: "design\ntext", Claim: true}); err != nil {
		t.Fatal(err)
	}
	calls := fakeCalls(t, client)
	if len(calls) != 1 {
		t.Fatalf("calls = %#v", calls)
	}
	args := calls[0][2:]
	assertFileArgument(t, args, "--body-file", "body\ntext")
	assertFileArgument(t, args, "--design-file", "design\ntext")
	if !containsArgs(args, "--claim") {
		t.Errorf("args = %#v, want --claim", args)
	}
}

func TestCloseAndCommentUseTemporaryFiles(t *testing.T) {
	client := newFake(t,
		fakeEntry{Match: []string{"close", "bd-1"}},
		fakeEntry{Match: []string{"comment", "bd-1"}},
	)
	if err := client.Close(context.Background(), "bd-1", "reason\ntext"); err != nil {
		t.Fatal(err)
	}
	if err := client.Comment(context.Background(), "bd-1", "comment\ntext"); err != nil {
		t.Fatal(err)
	}
	calls := fakeCalls(t, client)
	if len(calls) != 2 {
		t.Fatalf("calls = %#v", calls)
	}
	for _, args := range calls {
		args = args[2:]
		switch args[0] {
		case "close":
			assertFileArgument(t, args, "--reason-file", "reason\ntext")
		case "comment":
			assertFileArgument(t, args, "--file", "comment\ntext")
		default:
			t.Errorf("unexpected args: %#v", args)
		}
	}
}

func TestMutationConveniencesAndUndefer(t *testing.T) {
	client := newFake(t,
		fakeEntry{Match: []string{"update", "bd-1", "--claim"}},
		fakeEntry{Match: []string{"update", "bd-2", "--status", "open"}},
		fakeEntry{Match: []string{"label", "add", "bd-3", "a"}},
		fakeEntry{Match: []string{"label", "remove", "bd-3", "a"}},
		fakeEntry{Match: []string{"update", "bd-4", "--defer", "2026-09-01"}},
		fakeEntry{Match: []string{"update", "bd-5", "--defer", ""}},
		fakeEntry{Match: []string{"dep", "add", "bd-6", "bd-7"}},
	)
	checks := []struct {
		name string
		run  func() error
	}{
		{"claim", func() error { return client.Claim(context.Background(), "bd-1") }},
		{"release", func() error { return client.Release(context.Background(), "bd-2") }},
		{"label add", func() error { return client.LabelAdd(context.Background(), "bd-3", "a") }},
		{"label remove", func() error { return client.LabelRemove(context.Background(), "bd-3", "a") }},
		{"defer", func() error { return client.Defer(context.Background(), "bd-4", "2026-09-01") }},
		{"undefer", func() error { return client.Undefer(context.Background(), "bd-5") }},
		{"dependency", func() error { return client.DepAdd(context.Background(), "bd-6", "bd-7") }},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			if err := check.run(); err != nil {
				t.Fatal(err)
			}
		})
	}
	calls := fakeCalls(t, client)
	if len(calls) != len(checks) {
		t.Fatalf("calls = %#v", calls)
	}
	var undefer []string
	for _, call := range calls {
		args := call[2:]
		if len(args) == 4 && args[0] == "update" && args[1] == "bd-5" {
			undefer = args
		}
	}
	if !reflect.DeepEqual(undefer, []string{"update", "bd-5", "--defer", ""}) {
		t.Errorf("undefer = %#v", undefer)
	}
}

func TestMutationClassifiesAndLogsOnce(t *testing.T) {
	var output bytes.Buffer
	client := newFake(t, fakeEntry{Match: []string{"label", "add"}, Stderr: "failed", Exit: 7})
	client.Log = logging.New(logging.Config{Level: logging.Debug, Format: logging.JSON, Writer: &output})
	err := client.LabelAdd(context.Background(), "bd-1", "a")
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindFailed || classified.Op != "label" {
		t.Fatalf("LabelAdd() error = %#v, want failed label error", err)
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if len(lines) != 1 {
		t.Fatalf("events = %d: %s", len(lines), output.String())
	}
	var event map[string]any
	if err := json.Unmarshal(lines[0], &event); err != nil {
		t.Fatal(err)
	}
	if event["kind"] != "bd.run" {
		t.Errorf("event kind = %#v", event["kind"])
	}
}

func TestMutationRejectsNewlineArgumentsBeforeSpawning(t *testing.T) {
	client := newFake(t)
	err := client.LabelAdd(context.Background(), "bd-1", "bad\nlabel")
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUsage {
		t.Fatalf("LabelAdd() error = %T %v, want usage *Error", err, err)
	}
	if calls := fakeCalls(t, client); len(calls) != 0 {
		t.Errorf("calls = %#v, want none", calls)
	}
}

func TestTempTextUsesPrivateFilesAndRemovesThem(t *testing.T) {
	path, remove, err := tempText("text\nwith lines")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %#o, want 0600", info.Mode().Perm())
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "text\nwith lines" {
		t.Fatalf("file = %q, %v", data, err)
	}
	remove()
	remove()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file exists after cleanup: %v", err)
	}
	path, remove, err = tempText("")
	if err != nil || path != "" {
		t.Fatalf("empty tempText() = %q, %v", path, err)
	}
	remove()
}

func assertFileArgument(t *testing.T, args []string, flag, want string) {
	t.Helper()
	for index, arg := range args {
		if arg != flag || index+1 == len(args) {
			continue
		}
		path := args[index+1]
		data, err := os.ReadFile(path)
		if !os.IsNotExist(err) {
			t.Fatalf("temporary file %q remains: data=%q err=%v", path, data, err)
		}
		return
	}
	t.Errorf("args %#v lack %s", args, flag)
}

func containsArgs(args []string, want string) bool {
	for _, arg := range args {
		if arg == want {
			return true
		}
	}
	return false
}
