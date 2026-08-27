package repo

import (
	"context"
	"strings"
)

// ForBeadIn resolves a bead ID to the repository owning its prefix.
func ForBeadIn(repos []Repo, id string) (Repo, error) {
	if id == "" || strings.TrimSpace(id) == "" {
		return Repo{}, &NotFoundError{Query: id}
	}

	best := Repo{}
	bestLength := -1
	for _, record := range repos {
		prefix := record.Prefix
		if prefix == "" || len(id) <= len(prefix)+1 || id[:len(prefix)] != prefix || id[len(prefix)] != '-' {
			continue
		}
		if len(prefix) > bestLength {
			best = record
			bestLength = len(prefix)
		}
	}
	if bestLength < 0 {
		return Repo{}, &NotFoundError{Query: id}
	}
	return best, nil
}

// ForBead resolves a bead ID after loading the registry records.
func (r *Registry) ForBead(ctx context.Context, id string) (Repo, error) {
	return ForBeadIn(r.List(ctx), id)
}
