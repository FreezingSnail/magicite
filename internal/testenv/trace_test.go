package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestRecordReadRoundTrip(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "trace.tsv")
	argv := []string{"fake", "", "tab\tvalue", "line one\nline two", `quote " and '`, "日本語"}
	if err := Record(trace, argv, "/work\tdir\nnext"); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(trace)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{Seq: 1, Dir: "/work\tdir\nnext", Argv: argv}}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("Read() = %#v, want %#v", entries, want)
	}
}

func TestReadMissingAndReset(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "trace.tsv")
	entries, err := Read(trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Read(missing) = %#v, want empty", entries)
	}
	if err := Record(trace, []string{"fake"}, "/work"); err != nil {
		t.Fatal(err)
	}
	if err := Reset(trace); err != nil {
		t.Fatal(err)
	}
	entries, err = Read(trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("Read(reset) = %#v, want empty", entries)
	}
}

func TestReadIgnoresPartialFinalLine(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "trace.tsv")
	if err := os.WriteFile(trace, []byte("1\t\"/work\"\t\"fake\"\n2\t\"partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	entries, err := Read(trace)
	if err != nil {
		t.Fatal(err)
	}
	want := []Entry{{Seq: 1, Dir: "/work", Argv: []string{"fake"}}}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("Read() = %#v, want %#v", entries, want)
	}
}

func TestRecordConcurrentProcesses(t *testing.T) {
	trace := filepath.Join(t.TempDir(), "trace.tsv")
	const writers = 24
	var group sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		group.Add(1)
		go func() {
			defer group.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestTraceHelperProcess$", "--", trace, fmt.Sprint(i))
			if output, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("helper: %w: %s", err, output)
			}
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	entries, err := Read(trace)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != writers {
		t.Fatalf("entries = %d, want %d", len(entries), writers)
	}
	for i, entry := range entries {
		if got, want := entry.Seq, i+1; got != want {
			t.Errorf("entry %d sequence = %d, want %d", i, got, want)
		}
		if len(entry.Argv) != 2 || entry.Argv[0] != "fake" {
			t.Errorf("entry %d argv = %#v, want complete fake invocation", i, entry.Argv)
		}
	}
}

func TestTraceHelperProcess(t *testing.T) {
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator == -1 || len(os.Args) != separator+3 {
		return
	}
	if err := Record(os.Args[separator+1], []string{"fake", os.Args[separator+2]}, "/work"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Exit(0)
}
