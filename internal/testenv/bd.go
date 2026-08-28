package testenv

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
)

// Dependency is a bd dependency record stored by the fake.
type Dependency = bd.Dependency

// Bead is the mutable bd record seeded into the fake store.
type Bead struct {
	ID                 string       `json:"id"`
	Title              string       `json:"title"`
	Description        string       `json:"description"`
	Design             string       `json:"design"`
	AcceptanceCriteria string       `json:"acceptance_criteria"`
	Status             string       `json:"status"`
	Priority           int          `json:"priority"`
	IssueType          string       `json:"issue_type"`
	Assignee           string       `json:"assignee"`
	Owner              string       `json:"owner"`
	Parent             string       `json:"parent"`
	CreatedAt          string       `json:"created_at"`
	UpdatedAt          string       `json:"updated_at"`
	StartedAt          string       `json:"started_at"`
	DependencyCount    int          `json:"dependency_count"`
	DependentCount     int          `json:"dependent_count"`
	CommentCount       int          `json:"comment_count"`
	Dependencies       []Dependency `json:"dependencies"`
	Labels             []string     `json:"labels"`
	DeferredUntil      string       `json:"deferred_until,omitempty"`
	Comments           []string     `json:"comments,omitempty"`
	CloseReason        string       `json:"close_reason,omitempty"`
}

type bdFailure struct {
	Subcommand string `json:"subcommand"`
	ExitCode   int    `json:"exit_code"`
	Stderr     string `json:"stderr"`
}

type bdStore struct {
	Beads    []Bead      `json:"beads"`
	Failures []bdFailure `json:"failures,omitempty"`
	NextID   int         `json:"next_id"`
}

// BD controls one fake bd executable installed in an Env.
type BD struct {
	t     *testing.T
	env   *Env
	store string
}

// NewBD installs fake-bd as bd and initializes its process-shared store.
func NewBD(t *testing.T, env *Env) *BD {
	t.Helper()
	fake := &BD{t: t, env: env, store: filepath.Join(env.Root, "fake-bd-store.json")}
	env.fakeBDStore = fake.store
	env.Install("bd", "./cmd/fake-bd")
	fake.write(bdStore{NextID: 1})
	return fake
}

// Seed replaces the store's beads and clears scripted failures.
func (b *BD) Seed(beads ...Bead) {
	b.t.Helper()
	copy := append([]Bead(nil), beads...)
	next := 1
	for _, item := range copy {
		var number int
		if _, err := fmt.Sscanf(item.ID, "bd-%d", &number); err == nil && number >= next {
			next = number + 1
		}
	}
	b.write(bdStore{Beads: copy, NextID: next})
}

// Bead returns the current record and whether id exists.
func (b *BD) Bead(id string) (Bead, bool) {
	b.t.Helper()
	for _, item := range b.read().Beads {
		if item.ID == id {
			return item, true
		}
	}
	return Bead{}, false
}

// FailNext makes the next matching fake-bd subcommand fail once.
func (b *BD) FailNext(subcommand string, exitCode int, stderr string) {
	b.t.Helper()
	if exitCode == 0 {
		b.t.Fatalf("fake bd failure %q has zero exit code", subcommand)
	}
	b.withStore(func(value *bdStore) {
		value.Failures = append(value.Failures, bdFailure{Subcommand: subcommand, ExitCode: exitCode, Stderr: stderr})
	})
}

// Calls returns fake-bd invocations in trace order.
func (b *BD) Calls() []Entry {
	b.t.Helper()
	entries, err := Read(b.env.TracePath)
	if err != nil {
		b.t.Fatalf("read fake bd calls: %v", err)
	}
	return entries
}

// Store returns all current beads in insertion order.
func (b *BD) Store() []Bead {
	b.t.Helper()
	return append([]Bead(nil), b.read().Beads...)
}

func (b *BD) read() bdStore {
	b.t.Helper()
	var result bdStore
	b.withStore(func(value *bdStore) { result = *value })
	return result
}

func (b *BD) write(value bdStore) {
	b.t.Helper()
	b.withStore(func(target *bdStore) { *target = value })
}

func (b *BD) withStore(update func(*bdStore)) {
	b.t.Helper()
	file, err := os.OpenFile(b.store, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		b.t.Fatalf("open fake bd store: %v", err)
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		b.t.Fatalf("lock fake bd store: %v", err)
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	value, err := readBDStore(file)
	if err != nil {
		b.t.Fatalf("read fake bd store: %v", err)
	}
	update(&value)
	contents, err := json.Marshal(value)
	if err != nil {
		b.t.Fatalf("encode fake bd store: %v", err)
	}
	if err := file.Truncate(0); err != nil {
		b.t.Fatalf("truncate fake bd store: %v", err)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		b.t.Fatalf("seek fake bd store: %v", err)
	}
	if _, err := file.Write(contents); err != nil {
		b.t.Fatalf("write fake bd store: %v", err)
	}
	if err := file.Sync(); err != nil {
		b.t.Fatalf("sync fake bd store: %v", err)
	}
}

func readBDStore(file *os.File) (bdStore, error) {
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return bdStore{}, err
	}
	contents, err := io.ReadAll(file)
	if err != nil {
		return bdStore{}, err
	}
	if len(contents) == 0 {
		return bdStore{NextID: 1}, nil
	}
	var value bdStore
	if err := json.Unmarshal(contents, &value); err != nil {
		return bdStore{}, err
	}
	if value.NextID < 1 {
		value.NextID = 1
	}
	return value, nil
}
