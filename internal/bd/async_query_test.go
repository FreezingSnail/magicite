package bd

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestAsyncQueriesDecodeAndBuildArgs(t *testing.T) {
	output := `[{
		"id":"first",
		"priority":3
	},{
		"id":"second",
		"priority":1
	}]`
	cases := []struct {
		name string
		args []string
		call func(*Coalescer) (any, error)
		want any
	}{
		{"ready tasks", ArgsReady(), func(q *Coalescer) (any, error) { return q.ReadyTasks(context.Background()) }, []string{"first", "second"}},
		{"ready entries", ArgsReady(), func(q *Coalescer) (any, error) { return q.ReadyEntries(context.Background()) }, []ReadyEntry{{ID: "first", Priority: 3}, {ID: "second", Priority: 1}}},
		{"in progress", ArgsQuery(InProgressQuery(), false), func(q *Coalescer) (any, error) { return q.InProgressTasks(context.Background()) }, []string{"first", "second"}},
		{"open epics", ArgsQuery(OpenEpicsQuery(), false), func(q *Coalescer) (any, error) { return q.OpenEpics(context.Background()) }, []string{"first", "second"}},
		{"epic children", ArgsQuery(EpicChildrenQuery("epic-1"), false), func(q *Coalescer) (any, error) { return q.EpicChildren(context.Background(), "epic-1") }, []string{"first", "second"}},
		{"open epic children", ArgsQuery(EpicOpenChildrenQuery("epic-1"), false), func(q *Coalescer) (any, error) { return q.EpicOpenChildren(context.Background(), "epic-1") }, []string{"first", "second"}},
		{"drift fix", ArgsQuery(DriftFixQuery(), false), func(q *Coalescer) (any, error) { return q.DriftFixTasks(context.Background()) }, []string{"first", "second"}},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			client := newFake(t, fakeEntry{Stdout: output})
			got, err := test.call(NewCoalescer(client))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Fatalf("result = %#v, want %#v", got, test.want)
			}
			calls := fakeCalls(t, client)
			wantArgv := append([]string{"-C", client.Root}, test.args...)
			if !reflect.DeepEqual(calls, [][]string{wantArgv}) {
				t.Fatalf("argv = %#v, want %#v", calls, [][]string{wantArgv})
			}
		})
	}
}

func TestAsyncQueriesReturnNonNilEmptyResults(t *testing.T) {
	client := newFake(t, fakeEntry{Stdout: "[]"})
	q := NewCoalescer(client)

	ids, err := q.ReadyTasks(context.Background())
	if err != nil || ids == nil || len(ids) != 0 {
		t.Fatalf("ReadyTasks() = %#v, %v", ids, err)
	}
	entries, err := q.ReadyEntries(context.Background())
	if err != nil || entries == nil || len(entries) != 0 {
		t.Fatalf("ReadyEntries() = %#v, %v", entries, err)
	}
}

func TestAsyncQueriesPreserveClassifiedErrors(t *testing.T) {
	client := newFake(t, fakeEntry{Stderr: "Usage: bd ready", Exit: 2})
	_, err := NewCoalescer(client).ReadyTasks(context.Background())
	var classified *Error
	if !errors.As(err, &classified) || classified.Kind != KindUsage || classified.Op != "ready" {
		t.Fatalf("ReadyTasks() error = %v, want ready usage error", err)
	}
}

func TestAsyncQueriesCoalesceConcurrentPolls(t *testing.T) {
	client := newFake(t, fakeEntry{Stdout: `[{"id":"one","priority":2}]`, DelayMS: 100})
	q := NewCoalescer(client)
	start := make(chan struct{})
	results := make(chan []string, 2)
	var group sync.WaitGroup
	for range 2 {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			ids, err := q.ReadyTasks(context.Background())
			if err != nil {
				t.Errorf("ReadyTasks() error = %v", err)
				return
			}
			results <- ids
		}()
	}
	close(start)
	group.Wait()
	close(results)

	for ids := range results {
		if !reflect.DeepEqual(ids, []string{"one"}) {
			t.Errorf("ids = %#v, want [one]", ids)
		}
	}
	if calls := fakeCalls(t, client); len(calls) != 1 {
		t.Fatalf("bd calls = %d, want 1", len(calls))
	}
}
