package bdtest

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"

	"github.com/connorfranc/magicite/internal/bd"
)

func TestReadsUseSeedOrderAndNotFoundError(t *testing.T) {
	fake := New()
	fake.Seed(
		bd.Bead{ID: "work", Title: "first", IssueType: "task", Dependencies: []bd.Dependency{{ID: "dep"}}},
		bd.Bead{ID: "epic", IssueType: "epic"},
		bd.Bead{ID: "human", IssueType: "task"},
	)
	fake.SetLabels("human", "human")

	if _, err := fake.Show(context.Background(), "missing"); !bd.IsNotFound(err) {
		t.Fatalf("Show(missing) error = %v, want not found", err)
	}
	list, err := fake.List(context.Background(), false)
	if err != nil || ids(list) != "work,epic,human" {
		t.Fatalf("List() = %#v, %v", list, err)
	}
	ready, err := fake.Ready(context.Background())
	if err != nil || ids(ready) != "work" {
		t.Fatalf("Ready() = %#v, %v", ready, err)
	}
	query, err := fake.Query(context.Background(), "status=open", true)
	if err != nil || ids(query) != "work,epic,human" {
		t.Fatalf("Query() = %#v, %v", query, err)
	}
	deps, err := fake.Deps(context.Background(), "work")
	if err != nil || !reflect.DeepEqual(deps, []bd.Dependency{{ID: "dep"}}) {
		t.Fatalf("Deps() = %#v, %v", deps, err)
	}
	labels, err := fake.Labels(context.Background(), "human")
	if err != nil || !reflect.DeepEqual(labels, []string{"human"}) {
		t.Fatalf("Labels() = %#v, %v", labels, err)
	}

	calls := fake.Calls()
	if got, want := calls[3], (Call{Op: "query", Args: []string{"status=open", "true"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("query call = %#v, want %#v", got, want)
	}
}

func TestWritesMutateStateAndRecordCalls(t *testing.T) {
	fake := New()
	fake.Seed(bd.Bead{ID: "work", IssueType: "task"})
	id, err := fake.Create(context.Background(), bd.CreateRequest{Title: "new", Body: "body", Design: "design", Acceptance: "done", Labels: []string{"one"}, Priority: "2"})
	if err != nil || id != "work.1" {
		t.Fatalf("Create() = %q, %v", id, err)
	}
	if err := fake.Update(context.Background(), id, bd.UpdateRequest{Status: "blocked", Assignee: "me", Body: "new body", AddLabels: []string{"two"}, RemoveLabels: []string{"one"}}); err != nil {
		t.Fatal(err)
	}
	if err := fake.Claim(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := fake.Release(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := fake.Close(context.Background(), id, "finished"); err != nil {
		t.Fatal(err)
	}
	if err := fake.LabelAdd(context.Background(), id, "three"); err != nil {
		t.Fatal(err)
	}
	if err := fake.LabelRemove(context.Background(), id, "two"); err != nil {
		t.Fatal(err)
	}
	if err := fake.Defer(context.Background(), id, "2026-09-01"); err != nil {
		t.Fatal(err)
	}
	if err := fake.Undefer(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if err := fake.Comment(context.Background(), id, "note"); err != nil {
		t.Fatal(err)
	}
	if err := fake.DepAdd(context.Background(), id, "work"); err != nil {
		t.Fatal(err)
	}

	bead, ok := fake.Bead(id)
	if !ok || bead.Status != "closed" || bead.Assignee != "me" || bead.Description != "new body" || bead.Priority != 2 {
		t.Fatalf("Bead(%q) = %#v, %t", id, bead, ok)
	}
	labels, err := fake.Labels(context.Background(), id)
	if err != nil || !reflect.DeepEqual(labels, []string{"three"}) {
		t.Fatalf("Labels() = %#v, %v", labels, err)
	}
	calls := fake.Calls()
	if got, want := calls[len(calls)-3:], []Call{{Op: "comment", Args: []string{id, "note"}}, {Op: "dep-add", Args: []string{id, "work"}}, {Op: "labels", Args: []string{id}}}; !reflect.DeepEqual(got, want) {
		t.Errorf("tail calls = %#v, want %#v", got, want)
	}
}

func TestFailurePrecedesMutationAndCallsAreCopies(t *testing.T) {
	fake := New()
	fake.Seed(bd.Bead{ID: "work", Status: "open"})
	want := errors.New("stop")
	fake.Fail("close", want)
	if err := fake.Close(context.Background(), "work", "reason"); !errors.Is(err, want) {
		t.Fatalf("Close() error = %v, want %v", err, want)
	}
	if bead, _ := fake.Bead("work"); bead.Status != "open" {
		t.Fatalf("failed Close changed bead: %#v", bead)
	}
	calls := fake.Calls()
	calls[0].Args[0] = "changed"
	if got := fake.Calls()[0].Args[0]; got != "work" {
		t.Errorf("Calls leaked mutable args: %q", got)
	}
	fake.Fail("close", nil)
	if err := fake.Close(context.Background(), "work", "reason"); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentUse(t *testing.T) {
	fake := New()
	fake.Seed(bd.Bead{ID: "work"})
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			_, _ = fake.Show(context.Background(), "work")
			_ = fake.LabelAdd(context.Background(), "work", "label")
			_ = fake.LabelRemove(context.Background(), "work", "label")
			_ = fake.Claim(context.Background(), "work")
		}()
	}
	group.Wait()
	if len(fake.Calls()) != 128 {
		t.Errorf("calls = %d, want 128", len(fake.Calls()))
	}
}

func ids(beads []bd.Bead) string {
	result := ""
	for i, bead := range beads {
		if i > 0 {
			result += ","
		}
		result += bead.ID
	}
	return result
}
