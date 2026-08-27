package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NotFoundError reports that a repository could not be resolved.
type NotFoundError struct {
	Query string
}

// Error returns the unresolved repository query.
func (e *NotFoundError) Error() string {
	return fmt.Sprintf("repository not found: %s", e.Query)
}

// IsNotFound reports whether err unwraps to a NotFoundError.
func IsNotFound(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}

// GetIn resolves a repository by name or absolute root.
func GetIn(repos []Repo, nameOrRoot string) (Repo, error) {
	if nameOrRoot == "" {
		return Repo{}, &NotFoundError{Query: nameOrRoot}
	}
	for _, record := range repos {
		if record.Name == nameOrRoot {
			return record, nil
		}
	}
	if filepath.IsAbs(nameOrRoot) {
		for _, record := range repos {
			if SameRoot(record.Root, nameOrRoot) {
				return record, nil
			}
		}
	}
	return Repo{}, &NotFoundError{Query: nameOrRoot}
}

// CurrentIn resolves the repository containing dir, preferring the innermost root.
func CurrentIn(repos []Repo, dir string) (Repo, error) {
	query := dir
	if query == "" {
		var err error
		query, err = os.Getwd()
		if err != nil {
			query = ""
		}
	}

	normalized, ok := Directory(query)
	if !ok {
		return Repo{}, &NotFoundError{Query: dir}
	}
	canonicalDir, ok := canonical(normalized)
	if !ok {
		return Repo{}, &NotFoundError{Query: dir}
	}

	best := Repo{}
	bestLength := -1
	for _, record := range repos {
		root, ok := canonical(record.Root)
		if !ok || !strings.HasPrefix(canonicalDir, root) {
			continue
		}
		if len(root) > bestLength {
			best = record
			bestLength = len(root)
		}
	}
	if bestLength < 0 {
		return Repo{}, &NotFoundError{Query: dir}
	}
	return best, nil
}

// Get resolves a repository after loading the registry records.
func (r *Registry) Get(ctx context.Context, nameOrRoot string) (Repo, error) {
	return GetIn(r.List(ctx), nameOrRoot)
}

// Current resolves the repository containing dir after loading registry records.
func (r *Registry) Current(ctx context.Context, dir string) (Repo, error) {
	return CurrentIn(r.List(ctx), dir)
}
