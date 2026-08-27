package agent

import (
	"reflect"
	"sync"
	"testing"
)

type storeState struct {
	value int
}

func TestStoreCopiesStateAndPreservesHandleOrder(t *testing.T) {
	store := NewStore[storeState]("run")
	input := storeState{value: 1}
	handle, returned := store.Add(input)
	input.value = 2
	returned.value = 3

	if got, want := string(handle), "run-1"; got != want {
		t.Fatalf("handle = %q, want %q", got, want)
	}
	if got, ok := store.Get(handle); !ok || got.value != 1 {
		t.Fatalf("Get() = %#v, %v, want value 1, true", got, ok)
	}

	second, _ := store.Add(storeState{value: 4})
	if got, want := store.Handles(), []Handle{handle, second}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Handles() = %v, want %v", got, want)
	}
}

func TestStoreUpdateDeleteAndAliases(t *testing.T) {
	store := NewStore[storeState]("run")
	handle, _ := store.Add(storeState{value: 1})

	if !store.Update(handle, func(state *storeState) { state.value = 2 }) {
		t.Fatal("Update() = false, want true")
	}
	if got, _ := store.Get(handle); got.value != 2 {
		t.Fatalf("updated value = %d, want 2", got.value)
	}
	if store.Update("missing", func(*storeState) { t.Fatal("missing state updated") }) {
		t.Fatal("Update(missing) = true, want false")
	}

	if !store.Alias("backend-1", handle) || !store.Alias("backend-1", handle) {
		t.Fatal("Alias() should accept new and idempotent bindings")
	}
	if got, ok := store.Resolve("backend-1"); !ok || got != handle {
		t.Fatalf("Resolve(alias) = %q, %v, want %q, true", got, ok, handle)
	}
	if got, ok := store.Resolve(string(handle)); !ok || got != handle {
		t.Fatalf("Resolve(handle) = %q, %v, want %q, true", got, ok, handle)
	}
	other, _ := store.Add(storeState{})
	if store.Alias("backend-1", other) {
		t.Fatal("Alias() rebinding should fail")
	}
	if store.Alias("missing", "unknown") {
		t.Fatal("Alias() for unknown handle should fail")
	}

	deleted, ok := store.Delete(handle)
	if !ok || deleted.value != 2 {
		t.Fatalf("Delete() = %#v, %v, want value 2, true", deleted, ok)
	}
	if _, ok := store.Delete(handle); ok {
		t.Fatal("second Delete() = true, want false")
	}
	if _, ok := store.Resolve("backend-1"); ok {
		t.Fatal("deleted alias still resolves")
	}
	if got, want := store.Handles(), []Handle{other}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Handles() after delete = %v, want %v", got, want)
	}
}

func TestStoreConcurrentAccess(t *testing.T) {
	store := NewStore[storeState]("run")
	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(value int) {
			defer wg.Done()
			handle, _ := store.Add(storeState{value: value})
			store.Update(handle, func(state *storeState) { state.value++ })
			store.Get(handle)
			store.Delete(handle)
		}(i)
	}
	wg.Wait()
	if got := store.Handles(); len(got) != 0 {
		t.Fatalf("Handles() = %v, want empty", got)
	}
}
