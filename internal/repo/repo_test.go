package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDirectoryNormalizesWithoutFilesystemResolution(t *testing.T) {
	got, ok := Directory("./nested/../repo//")
	if !ok {
		t.Fatal("Directory() ok = false")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("Directory() = %q, want absolute path", got)
	}
	if !strings.HasSuffix(got, string(filepath.Separator)) {
		t.Fatalf("Directory() = %q, want trailing separator", got)
	}
	if strings.HasSuffix(strings.TrimSuffix(got, string(filepath.Separator)), string(filepath.Separator)) {
		t.Fatalf("Directory() = %q, want exactly one trailing separator", got)
	}

	for _, input := range []string{"", "/", "////"} {
		got, ok := Directory(input)
		if input == "" {
			if ok || got != "" {
				t.Errorf("Directory(%q) = %q, %v, want empty failure", input, got, ok)
			}
			continue
		}
		if !ok || got == "" {
			t.Errorf("Directory(%q) = %q, %v, want success", input, got, ok)
		}
	}
}

func TestMakeDefaultsAndNormalizesFields(t *testing.T) {
	got, ok := Make("./project/../repo", " \t", "\n", " ")
	if !ok {
		t.Fatal("Make() ok = false")
	}
	root, rootOK := Directory("./repo")
	if !rootOK {
		t.Fatal("Directory() setup failed")
	}
	wantName := filepath.Base(root)
	want := Repo{Root: root, Name: wantName, Prefix: wantName, Branch: "main"}
	if got != want {
		t.Errorf("Make() = %#v, want %#v", got, want)
	}
	if !got.Valid() {
		t.Errorf("Make() record invalid: %#v", got)
	}

	if _, ok := Make("", "name", "prefix", "branch"); ok {
		t.Error("Make(empty root) ok = true, want false")
	}
}

func TestValidAndLogName(t *testing.T) {
	valid := Repo{Root: "/repo/", Name: "alpha \t beta\n", Prefix: "a", Branch: "main"}
	if !valid.Valid() {
		t.Fatal("Valid() = false, want true")
	}
	if got, want := valid.LogName(), "alpha_beta_"; got != want {
		t.Errorf("LogName() = %q, want %q", got, want)
	}

	for _, record := range []Repo{
		{},
		{Root: "/repo/", Name: "name", Prefix: "", Branch: "main"},
		{Root: "repo/", Name: "name", Prefix: "prefix", Branch: "main"},
	} {
		if record.Valid() {
			t.Errorf("Valid(%#v) = true, want false", record)
		}
		if got := record.LogName(); got != "-" {
			t.Errorf("LogName(%#v) = %q, want %q", record, got, "-")
		}
	}
}

func TestSameRootResolvesSymlinkAndFallsBackForMissingPaths(t *testing.T) {
	directory := t.TempDir()
	target := filepath.Join(directory, "target")
	link := filepath.Join(directory, "link")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if !SameRoot(link, target) {
		t.Errorf("SameRoot(%q, %q) = false, want true", link, target)
	}

	missingA := filepath.Join(directory, "missing", "../future")
	missingB := filepath.Join(directory, "future")
	if !SameRoot(missingA, missingB) {
		t.Errorf("SameRoot(%q, %q) = false, want true", missingA, missingB)
	}
	if SameRoot("", directory) || SameRoot(directory, "") {
		t.Error("SameRoot() with empty path = true, want false")
	}
}
