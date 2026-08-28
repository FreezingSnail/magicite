package repo

import (
	"context"
	"sync"

	"github.com/connorfranc/magicite/internal/config"
	"github.com/connorfranc/magicite/internal/logging"
)

// Options configures repository discovery and registry events.
type Options struct {
	Repos  config.ReposConfig
	Dir    string
	Probe  *Prober
	Prefix PrefixSource
	Log    func(level logging.Level, kind string, fields map[string]any)
}

// Registry discovers admitted repositories and caches their records.
type Registry struct {
	mu      sync.Mutex
	options Options
	records []Repo
	valid   bool
	epoch   uint64
}

// New creates a registry from daemon configuration.
func New(cfg config.Config) *Registry {
	return NewWith(Options{
		Repos:  cfg.Repos,
		Probe:  NewProber(),
		Prefix: NewPrefixSource(),
	})
}

// NewWith creates a registry from explicit dependencies.
func NewWith(options Options) *Registry {
	if options.Probe == nil {
		options.Probe = NewProber()
	}
	if options.Prefix.NewRunner == nil {
		options.Prefix = NewPrefixSource()
	}
	if options.Log == nil {
		options.Log = logging.Event
	}
	return &Registry{options: options}
}

// List returns cached records, refreshing when the cache is invalid.
func (r *Registry) List(ctx context.Context) []Repo {
	r.mu.Lock()
	if r.valid {
		records := copyRecords(r.records)
		r.mu.Unlock()
		return records
	}
	r.mu.Unlock()
	return r.Refresh(ctx)
}

// Refresh discovers, admits, names, and prefixes repository records.
func (r *Registry) Refresh(ctx context.Context) []Repo {
	if ctx.Err() != nil {
		return r.cached()
	}

	r.mu.Lock()
	options := r.options
	epoch := r.epoch
	r.mu.Unlock()

	finder := Finder{Repos: options.Repos, Probe: options.Probe, Dir: options.Dir}
	roots, cancelled := admit(ctx, options.Probe, finder.Candidates(ctx))
	if cancelled {
		return r.cached()
	}
	if len(roots) == 0 && options.Repos.Discover != "explicit" {
		roots, cancelled = admit(ctx, options.Probe, finder.AmbientCandidates(ctx))
		if cancelled {
			return r.cached()
		}
	}

	records := Records(roots)
	for index := range records {
		if ctx.Err() != nil {
			return r.cached()
		}
		if prefix, ok := options.Prefix.Prefix(ctx, records[index].Root); ok && ValidPrefix(prefix) {
			records[index].Prefix = prefix
		}
		if ctx.Err() != nil {
			return r.cached()
		}
	}
	records = validRecords(records)

	r.mu.Lock()
	if r.epoch != epoch {
		records = copyRecords(r.records)
		r.mu.Unlock()
		return records
	}
	r.records = copyRecords(records)
	r.valid = true
	r.mu.Unlock()

	if len(roots) == 0 {
		options.Log(logging.Warn, "repo.refresh", map[string]any{"count": 0, "reason": "none-admitted"})
	} else {
		options.Log(logging.Info, "repo.refresh", map[string]any{"count": len(records), "names": logNames(records)})
	}
	return copyRecords(records)
}

// Invalidate drops cached records. The next List performs a full refresh.
func (r *Registry) Invalidate() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.epoch++
	r.records = nil
	r.valid = false
}

func (r *Registry) cached() []Repo {
	r.mu.Lock()
	defer r.mu.Unlock()
	return copyRecords(r.records)
}

func admit(ctx context.Context, probe *Prober, candidates []string) ([]string, bool) {
	roots := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if ctx.Err() != nil {
			return nil, true
		}
		root, ok := probe.Admit(ctx, candidate)
		if ctx.Err() != nil {
			return nil, true
		}
		if ok {
			roots = append(roots, root)
		}
	}
	return roots, false
}

func validRecords(records []Repo) []Repo {
	valid := make([]Repo, 0, len(records))
	for _, record := range records {
		if record.Valid() {
			valid = append(valid, record)
		}
	}
	return valid
}

func copyRecords(records []Repo) []Repo {
	return append([]Repo(nil), records...)
}

func logNames(records []Repo) []string {
	names := make([]string, 0, len(records))
	for _, record := range records {
		names = append(names, record.LogName())
	}
	return names
}
