package repo

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/FreezingSnail/magicite/internal/bd"
)

const maxConfigBytes = 64 * 1024

// Runner invokes bd with argv arguments.
type Runner interface {
	Run(ctx context.Context, args ...string) (bd.Result, error)
}

// PrefixSource derives repository issue prefixes from bd and its config file.
type PrefixSource struct {
	NewRunner func(root string) (Runner, error)
}

// NewPrefixSource creates a prefix source backed by the bd command.
func NewPrefixSource() PrefixSource {
	return PrefixSource{
		NewRunner: func(root string) (Runner, error) {
			return bd.New("bd", root)
		},
	}
}

// Prefix returns the first valid issue prefix found for root.
func (s PrefixSource) Prefix(ctx context.Context, root string) (string, bool) {
	if ctx.Err() != nil {
		return "", false
	}

	if s.NewRunner != nil {
		runner, err := s.NewRunner(root)
		if err == nil && runner != nil && ctx.Err() == nil {
			result, err := runner.Run(ctx, "config", "list")
			if err == nil && ctx.Err() == nil && result.ExitCode == 0 {
				if prefix, ok := configPrefix(result.Stdout); ok {
					return prefix, true
				}
			}
		}
	}

	if ctx.Err() != nil {
		return "", false
	}
	return filePrefix(root)
}

// ValidPrefix reports whether prefix contains only bd issue-prefix characters.
func ValidPrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	for i := range prefix {
		character := prefix[i]
		if !((character >= 'a' && character <= 'z') ||
			(character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') ||
			character == '_' || character == '-') {
			return false
		}
	}
	return true
}

func configPrefix(output []byte) (string, bool) {
	for _, line := range strings.Split(string(output), "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if !found || strings.TrimSpace(key) != "issue_prefix" {
			continue
		}
		if prefix := strings.TrimSpace(value); ValidPrefix(prefix) {
			return prefix, true
		}
	}
	return "", false
}

func filePrefix(root string) (string, bool) {
	file, err := os.Open(filepath.Join(root, ".beads", "config.yaml"))
	if err != nil {
		return "", false
	}
	defer file.Close()

	contents, err := io.ReadAll(io.LimitReader(file, maxConfigBytes))
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(contents), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found || strings.TrimSpace(key) != "issue-prefix" {
			continue
		}
		prefix := strings.TrimSpace(value)
		if len(prefix) >= 2 && prefix[0] == '"' && prefix[len(prefix)-1] == '"' {
			prefix = prefix[1 : len(prefix)-1]
		}
		if ValidPrefix(prefix) {
			return prefix, true
		}
	}
	return "", false
}
