// Package repo defines normalized repository records.
package repo

import (
	"path/filepath"
	"strings"
	"unicode"
)

const defaultBranch = "main"

// Repo identifies a repository and its routing metadata.
type Repo struct {
	Root, Name, Prefix, Branch string
}

// Make builds a repository record from root and optional metadata.
func Make(root, name, prefix, branch string) (Repo, bool) {
	normalizedRoot, ok := Directory(root)
	if !ok {
		return Repo{}, false
	}

	defaultName := filepath.Base(normalizedRoot)
	name = fallback(name, defaultName)
	prefix = fallback(prefix, defaultName)
	branch = fallback(branch, defaultBranch)
	return Repo{
		Root:   normalizedRoot,
		Name:   name,
		Prefix: prefix,
		Branch: branch,
	}, true
}

// Valid reports whether the repository record contains all required fields.
func (r Repo) Valid() bool {
	separator := string(filepath.Separator)
	return filepath.IsAbs(r.Root) &&
		strings.HasSuffix(r.Root, separator) &&
		r.Name != "" && r.Prefix != "" && r.Branch != ""
}

// LogName returns a log-safe representation of the repository name.
func (r Repo) LogName() string {
	if !r.Valid() {
		return "-"
	}

	var builder strings.Builder
	space := false
	for _, character := range r.Name {
		if unicode.IsSpace(character) {
			space = true
			continue
		}
		if space {
			builder.WriteByte('_')
			space = false
		}
		builder.WriteRune(character)
	}
	if space {
		builder.WriteByte('_')
	}
	return builder.String()
}

// Directory normalizes path without inspecting the filesystem.
func Directory(path string) (string, bool) {
	if path == "" {
		return "", false
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	cleaned := filepath.Clean(absolute)
	separator := string(filepath.Separator)
	return strings.TrimRight(cleaned, separator) + separator, true
}

// SameRoot reports whether paths resolve to the same directory.
func SameRoot(a, b string) bool {
	left, leftOK := canonical(a)
	right, rightOK := canonical(b)
	return leftOK && rightOK && left == right
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func canonical(path string) (string, bool) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return Directory(path)
	}
	return Directory(resolved)
}
