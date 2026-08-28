// Package bdtest provides an in-memory bd.Bridge for consumer tests.
package bdtest

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/connorfranc/magicite/internal/bd"
)

// Call records one Bridge method invocation.
type Call struct {
	Op   string
	Args []string
}

// Fake is a synchronized in-memory bd.Bridge.
type Fake struct {
	mu       sync.Mutex
	beads    map[string]bd.Bead
	ids      []string
	labels   map[string][]string
	deferred map[string]string
	calls    []Call
	failures map[string]error
	prefix   string
	counter  int
}

var _ bd.Bridge = (*Fake)(nil)

// New creates an empty fake.
func New() *Fake {
	return &Fake{
		beads:    make(map[string]bd.Bead),
		labels:   make(map[string][]string),
		deferred: make(map[string]string),
		failures: make(map[string]error),
		prefix:   "fake",
	}
}

// Seed adds beads in order. Re-seeding an id replaces its value without changing order.
func (f *Fake) Seed(beads ...bd.Bead) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.counter == 0 && f.prefix == "fake" && len(beads) != 0 && beads[0].ID != "" {
		f.prefix = seededPrefix(beads[0].ID)
	}
	for _, bead := range beads {
		if _, exists := f.beads[bead.ID]; !exists {
			f.ids = append(f.ids, bead.ID)
		}
		f.beads[bead.ID] = cloneBead(bead)
	}
}

// SetLabels replaces id's labels.
func (f *Fake) SetLabels(id string, labels ...string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.labels[id] = append([]string(nil), labels...)
}

// Fail makes op return err until replaced or cleared with a nil error.
func (f *Fake) Fail(op string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		delete(f.failures, op)
		return
	}
	f.failures[op] = err
}

// Calls returns an independent copy of recorded calls.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]Call, len(f.calls))
	for i, call := range f.calls {
		calls[i] = Call{Op: call.Op, Args: append([]string(nil), call.Args...)}
	}
	return calls
}

// Bead returns a copy of the bead identified by id.
func (f *Fake) Bead(id string) (bd.Bead, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	bead, ok := f.beads[id]
	return cloneBead(bead), ok
}

func (f *Fake) Show(_ context.Context, id string) (bd.Bead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("show", id)
	if err := f.failed("show"); err != nil {
		return bd.Bead{}, err
	}
	bead, ok := f.beads[id]
	if !ok {
		return bd.Bead{}, notFound("show", id)
	}
	return cloneBead(bead), nil
}

func (f *Fake) List(_ context.Context, all bool) ([]bd.Bead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("list", strconv.FormatBool(all))
	if err := f.failed("list"); err != nil {
		return nil, err
	}
	return f.allBeads(), nil
}

func (f *Fake) Ready(_ context.Context) ([]bd.Bead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("ready")
	if err := f.failed("ready"); err != nil {
		return nil, err
	}
	ready := make([]bd.Bead, 0, len(f.beads))
	for _, id := range f.order() {
		bead := f.beads[id]
		if bead.IssueType == "epic" || bd.IsHuman(f.labels[id]) {
			continue
		}
		ready = append(ready, cloneBead(bead))
	}
	return ready, nil
}

func (f *Fake) Query(_ context.Context, q string, all bool) ([]bd.Bead, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("query", q, strconv.FormatBool(all))
	if err := f.failed("query"); err != nil {
		return nil, err
	}
	return f.allBeads(), nil
}

func (f *Fake) Deps(_ context.Context, id string) ([]bd.Dependency, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("deps", id)
	if err := f.failed("deps"); err != nil {
		return nil, err
	}
	bead, ok := f.beads[id]
	if !ok {
		return nil, notFound("dep", id)
	}
	return append([]bd.Dependency(nil), bead.Dependencies...), nil
}

func (f *Fake) Labels(_ context.Context, id string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("labels", id)
	if err := f.failed("labels"); err != nil {
		return nil, err
	}
	if _, ok := f.beads[id]; !ok {
		return nil, notFound("label", id)
	}
	return append([]string(nil), f.labels[id]...), nil
}

func (f *Fake) Create(_ context.Context, req bd.CreateRequest) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("create", req.Title)
	if err := f.failed("create"); err != nil {
		return "", err
	}
	for {
		f.counter++
		id := fmt.Sprintf("%s.%d", f.prefix, f.counter)
		if _, exists := f.beads[id]; exists {
			continue
		}
		bead := bd.Bead{ID: id, Title: req.Title, IssueType: req.Type, Parent: req.Parent, Description: req.Body, Design: req.Design, AcceptanceCriteria: req.Acceptance, Status: "open"}
		if bead.IssueType == "" {
			bead.IssueType = "task"
		}
		if req.Priority != "" {
			bead.Priority, _ = strconv.Atoi(req.Priority)
		}
		f.beads[id] = bead
		f.ids = append(f.ids, id)
		f.labels[id] = append([]string(nil), req.Labels...)
		return id, nil
	}
}

func (f *Fake) Update(_ context.Context, id string, req bd.UpdateRequest) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("update", id)
	if err := f.failed("update"); err != nil {
		return err
	}
	bead, ok := f.beads[id]
	if !ok {
		return notFound("update", id)
	}
	if req.Status != "" {
		bead.Status = req.Status
	}
	if req.Assignee != "" {
		bead.Assignee = req.Assignee
	}
	if req.Body != "" {
		bead.Description = req.Body
	}
	if req.Design != "" {
		bead.Design = req.Design
	}
	if req.Acceptance != "" {
		bead.AcceptanceCriteria = req.Acceptance
	}
	if req.Claim {
		bead.Status = "in_progress"
	}
	f.beads[id] = bead
	f.addLabels(id, req.AddLabels)
	f.removeLabels(id, req.RemoveLabels)
	if req.Defer != "" {
		f.deferred[id] = req.Defer
	}
	return nil
}

func (f *Fake) Claim(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("claim", id)
	if err := f.failed("claim"); err != nil {
		return err
	}
	return f.setStatus(id, "in_progress", "claim")
}

func (f *Fake) Release(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("release", id)
	if err := f.failed("release"); err != nil {
		return err
	}
	return f.setStatus(id, "open", "release")
}

func (f *Fake) Close(_ context.Context, id, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("close", id, reason)
	if err := f.failed("close"); err != nil {
		return err
	}
	return f.setStatus(id, "closed", "close")
}

func (f *Fake) Comment(_ context.Context, id, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("comment", id, text)
	return f.failed("comment")
}

func (f *Fake) LabelAdd(_ context.Context, id, label string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("label-add", id, label)
	if err := f.failed("label-add"); err != nil {
		return err
	}
	if _, ok := f.beads[id]; !ok {
		return notFound("label", id)
	}
	f.addLabels(id, []string{label})
	return nil
}

func (f *Fake) LabelRemove(_ context.Context, id, label string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("label-remove", id, label)
	if err := f.failed("label-remove"); err != nil {
		return err
	}
	if _, ok := f.beads[id]; !ok {
		return notFound("label", id)
	}
	f.removeLabels(id, []string{label})
	return nil
}

func (f *Fake) Defer(_ context.Context, id, until string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("defer", id, until)
	if err := f.failed("defer"); err != nil {
		return err
	}
	if _, ok := f.beads[id]; !ok {
		return notFound("update", id)
	}
	f.deferred[id] = until
	return nil
}

func (f *Fake) Undefer(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("undefer", id)
	if err := f.failed("undefer"); err != nil {
		return err
	}
	if _, ok := f.beads[id]; !ok {
		return notFound("update", id)
	}
	delete(f.deferred, id)
	return nil
}

func (f *Fake) DepAdd(_ context.Context, id, dependsOn string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record("dep-add", id, dependsOn)
	return f.failed("dep-add")
}

func (f *Fake) record(op string, args ...string) {
	f.calls = append(f.calls, Call{Op: op, Args: append([]string(nil), args...)})
}

func (f *Fake) failed(op string) error { return f.failures[op] }

func (f *Fake) setStatus(id, status, op string) error {
	bead, ok := f.beads[id]
	if !ok {
		return notFound(op, id)
	}
	bead.Status = status
	f.beads[id] = bead
	return nil
}

func (f *Fake) addLabels(id string, labels []string) {
	for _, label := range labels {
		if !contains(f.labels[id], label) {
			f.labels[id] = append(f.labels[id], label)
		}
	}
}

func (f *Fake) removeLabels(id string, labels []string) {
	for _, remove := range labels {
		kept := f.labels[id][:0]
		for _, label := range f.labels[id] {
			if label != remove {
				kept = append(kept, label)
			}
		}
		f.labels[id] = kept
	}
}

func (f *Fake) allBeads() []bd.Bead {
	beads := make([]bd.Bead, 0, len(f.beads))
	for _, id := range f.order() {
		beads = append(beads, cloneBead(f.beads[id]))
	}
	return beads
}

func (f *Fake) order() []string {
	return append([]string(nil), f.ids...)
}

func seededPrefix(id string) string {
	if index := strings.LastIndex(id, "."); index > 0 {
		return id[:index]
	}
	return id
}

func cloneBead(bead bd.Bead) bd.Bead {
	bead.Dependencies = append([]bd.Dependency(nil), bead.Dependencies...)
	return bead
}

func notFound(op, id string) error {
	return &bd.Error{Op: op, Args: []string{id}, Kind: bd.KindNotFound, ExitCode: 1, Detail: "bead not found"}
}

func contains(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}
