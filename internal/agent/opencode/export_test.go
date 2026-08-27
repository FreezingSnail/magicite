package opencode

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/FreezingSnail/magicite/internal/agent"
)

func TestParseExport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "export_two_files.json"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseExport(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []agent.FileDiff{
		{File: "first.go", Patch: "@@ -1 +1 @@\n-old\n+new\n", Additions: 1, Deletions: 1, Status: "modified"},
		{File: "new.txt", Patch: "@@ -0,0 +1 @@\n+new\n", Additions: 1, Deletions: 0, Status: "added"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseExport() = %#v, want %#v", got, want)
	}
}

func TestParseExportRejectsInvalidJSON(t *testing.T) {
	if diffs, err := parseExport([]byte("{")); err == nil || diffs != nil {
		t.Fatalf("parseExport() = (%v, %v), want (nil, error)", diffs, err)
	}
}
