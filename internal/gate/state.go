package gate

import (
	"github.com/FreezingSnail/magicite/internal/logging"
	"github.com/FreezingSnail/magicite/internal/repo"
)

type key struct {
	repo, epic string
}

func (g *Gate) key(r repo.Repo, epic string) (key, bool) {
	if r == (repo.Repo{}) {
		g.warn("nil-repo", epic)
		return key{}, false
	}
	if r.Name == "" {
		g.warn("empty-repo", epic)
		return key{}, false
	}
	if epic == "" {
		g.warn("empty-epic", r.Name)
		return key{}, false
	}
	return key{repo: r.Name, epic: epic}, true
}

func (g *Gate) attempts(k key) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.attempted[k]
}

func (g *Gate) noteAttempt(k key) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.attempted[k]++
}

// exhaust records an exhausted retry budget and reports whether it was already recorded.
func (g *Gate) exhaust(k key) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	already := g.exhausted[k]
	g.exhausted[k] = true
	return already
}

func (g *Gate) track(handle string, k key) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.tracked[handle] = k
}

func (g *Gate) drop(handle string) (key, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	k, ok := g.tracked[handle]
	if ok {
		delete(g.tracked, handle)
	}
	return k, ok
}

func (g *Gate) inFlight(r repo.Repo) bool {
	k, ok := g.key(r, "in-flight")
	if !ok {
		return false
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	for _, tracked := range g.tracked {
		if tracked.repo == k.repo {
			return true
		}
	}
	return false
}

func (g *Gate) resolve(handle string) (repo.Repo, string, bool) {
	g.mu.Lock()
	k, ok := g.tracked[handle]
	g.mu.Unlock()
	if !ok {
		return repo.Repo{}, "", false
	}
	r, ok := g.repos.Get(k.repo)
	if !ok {
		g.warn("vanished-repo", k.repo)
		return repo.Repo{}, "", false
	}
	return r, k.epic, true
}

// recordStart records the first SHA observed for k and reports whether it was new.
func (g *Gate) recordStart(k key, sha string) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.started[k]; exists {
		return false
	}
	g.started[k] = sha
	return true
}

func (g *Gate) start(k key) (string, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	sha, ok := g.started[k]
	return sha, ok
}

func (g *Gate) clear(k key) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.attempted, k)
	delete(g.exhausted, k)
	delete(g.started, k)
	for handle, tracked := range g.tracked {
		if tracked == k {
			delete(g.tracked, handle)
		}
	}
}

func (g *Gate) warn(reason, value string) {
	g.log.Event(logging.Warn, "gate.state", map[string]any{"reason": reason, "value": value})
}
