package parity

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/testenv"
)

var updateGoldens bool

var (
	shaPattern       = regexp.MustCompile(`[0-9a-fA-F]{40}`)
	timestampPattern = regexp.MustCompile(`\b[0-9]{4}-[0-9]{2}-[0-9]{2}(?:[T ][0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]+)?(?:Z|[+-][0-9]{2}:?[0-9]{2})?)?\b`)
	durationPattern  = regexp.MustCompile(`\b(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:ns|us|µs|ms|s|m|h)(?:(?:[0-9]+(?:\.[0-9]+)?|\.[0-9]+)(?:ns|us|µs|ms|s|m|h))*\b`)
)

func init() {
	flag.BoolVar(&updateGoldens, "update", false, "update parity golden files")
}

// AssertTrace compares entries with the named checked-in golden trace.
func AssertTrace(t *testing.T, name string, entries []testenv.Entry) {
	t.Helper()

	path := dataPath(filepath.Join("golden", name+".trace"))
	got := Normalize(traceRoot(entries), Render(entries))
	if updateGoldens {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden %q: %v", path, err)
		}
		t.Logf("updated golden %q", path)
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("missing golden %q; create it with: go test ./internal/parity -run '^%s$' -update", path, regexp.QuoteMeta(t.Name()))
		}
		t.Fatalf("read golden %q: %v", path, err)
	}
	want := string(wantBytes)
	if want == got {
		return
	}

	line := firstDifferentLine(want, got)
	t.Fatalf("trace %q differs at line %d:\n%s", name, line, unifiedDiff(want, got, line))
}

// Render formats each recorded invocation as cwd followed by its argv.
func Render(entries []testenv.Entry) string {
	var rendered strings.Builder
	for _, entry := range entries {
		fields := make([]string, 0, len(entry.Argv)+1)
		fields = append(fields, filepath.ToSlash(entry.Dir))
		fields = append(fields, entry.Argv...)
		for i, field := range fields {
			if i != 0 {
				rendered.WriteByte(' ')
			}
			rendered.WriteString(shellField(field))
		}
		rendered.WriteByte('\n')
	}
	return rendered.String()
}

// Normalize removes paths and execution-time values that vary between runs.
func Normalize(root string, s string) string {
	s = normalizeRoot(root, s)
	shaNames := make(map[string]string)
	shaNumber := 0
	s = shaPattern.ReplaceAllStringFunc(s, func(value string) string {
		key := strings.ToLower(value)
		if name, ok := shaNames[key]; ok {
			return name
		}
		shaNumber++
		name := fmt.Sprintf("{sha%d}", shaNumber)
		shaNames[key] = name
		return name
	})
	s = timestampPattern.ReplaceAllString(s, "{time}")
	return durationPattern.ReplaceAllString(s, "{time}")
}

func shellField(field string) string {
	if !strings.ContainsAny(field, " \t\r\n") {
		return field
	}
	return "'" + strings.ReplaceAll(field, "'", "'\\''") + "'"
}

func normalizeRoot(root, s string) string {
	if root == "" {
		return s
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return s
	}
	absolute = filepath.Clean(absolute)
	if absolute == string(filepath.Separator) {
		return s
	}

	var normalized strings.Builder
	for len(s) > 0 {
		index := strings.Index(s, absolute)
		if index < 0 {
			normalized.WriteString(s)
			break
		}
		if index > 0 && isPathCharacter(s[index-1]) {
			normalized.WriteString(s[:index+len(absolute)])
			s = s[index+len(absolute):]
			continue
		}
		end := index + len(absolute)
		if end < len(s) && s[end] != filepath.Separator && s[end] != '/' {
			normalized.WriteString(s[:end])
			s = s[end:]
			continue
		}
		normalized.WriteString(s[:index])
		normalized.WriteString("{root}")
		s = s[end:]
	}
	return normalized.String()
}

func isPathCharacter(value byte) bool {
	return value == '/' || value == '\\' || value == '.' || value == '-' || value == '_' ||
		(value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9')
}

func traceRoot(entries []testenv.Entry) string {
	var root string
	for _, entry := range entries {
		paths := append([]string{entry.Dir}, entry.Argv...)
		for _, path := range paths {
			if !filepath.IsAbs(path) {
				continue
			}
			dir := filepath.Clean(path)
			if root == "" {
				root = dir
				continue
			}
			for !pathWithin(root, dir) {
				parent := filepath.Dir(root)
				if parent == root {
					return ""
				}
				root = parent
			}
		}
	}
	return root
}

func pathWithin(root, path string) bool {
	if root == path {
		return true
	}
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func firstDifferentLine(want, got string) int {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	limit := len(wantLines)
	if len(gotLines) < limit {
		limit = len(gotLines)
	}
	for i := 0; i < limit; i++ {
		if wantLines[i] != gotLines[i] {
			return i + 1
		}
	}
	return limit + 1
}

func unifiedDiff(want, got string, line int) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	var diff strings.Builder
	fmt.Fprintf(&diff, "--- golden\n+++ trace\n@@ -%d +%d @@\n", line, line)
	if line <= len(wantLines) {
		fmt.Fprintf(&diff, "-%s\n", wantLines[line-1])
	}
	if line <= len(gotLines) {
		fmt.Fprintf(&diff, "+%s", gotLines[line-1])
	}
	return diff.String()
}
