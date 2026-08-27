package agent

import (
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestRegistryRegisterValidatesAdapter(t *testing.T) {
	registry := NewRegistry()

	for _, adapter := range []Adapter{
		nil,
		&fakeAdapter{executable: "agent"},
		&fakeAdapter{name: "agent"},
	} {
		if err := registry.Register(adapter); !errors.Is(err, ErrInvalidAdapter) {
			t.Errorf("Register(%#v) error = %v, want ErrInvalidAdapter", adapter, err)
		}
	}

	adapter := &fakeAdapter{name: "agent", executable: "agent"}
	if err := registry.Register(adapter); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(adapter); !errors.Is(err, ErrDuplicateBackend) {
		t.Errorf("duplicate Register() error = %v, want ErrDuplicateBackend", err)
	}
}

func TestRegistryLookupDefaultAndSortedNames(t *testing.T) {
	registry := NewRegistry()
	for _, name := range []string{"zebra", DefaultBackend, "alpha"} {
		if err := registry.Register(&fakeAdapter{name: name, executable: "agent"}); err != nil {
			t.Fatal(err)
		}
	}

	adapter, err := registry.Lookup("")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := adapter.Name(), DefaultBackend; got != want {
		t.Errorf("default adapter = %q, want %q", got, want)
	}
	if _, err := registry.Lookup("missing"); !errors.Is(err, ErrUnknownBackend) {
		t.Errorf("unknown Lookup() error = %v, want ErrUnknownBackend", err)
	}
	if got, want := registry.Names(), []string{"alpha", DefaultBackend, "zebra"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Names() = %v, want %v", got, want)
	}
}

func TestRegistryAvailable(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry()
	if err := registry.Register(&fakeAdapter{name: "present", executable: executable}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Register(&fakeAdapter{name: "missing", executable: "magicite-definitely-missing-executable"}); err != nil {
		t.Fatal(err)
	}
	if err := registry.Available("present"); err != nil {
		t.Errorf("Available(present) error = %v", err)
	}
	if err := registry.Available("missing"); !errors.Is(err, ErrExecutableMissing) {
		t.Errorf("Available(missing) error = %v, want ErrExecutableMissing", err)
	}
	if err := registry.Available("unknown"); !errors.Is(err, ErrUnknownBackend) {
		t.Errorf("Available(unknown) error = %v, want ErrUnknownBackend", err)
	}
}
