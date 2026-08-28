package backends

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/connorfranc/magicite/internal/agent"
	"github.com/connorfranc/magicite/internal/config"
)

func TestRegisterAddsBothBackends(t *testing.T) {
	reg := agent.NewRegistry()
	if err := Register(reg, config.Default()); err != nil {
		t.Fatal(err)
	}
	want := []string{"kiro", "opencode"}
	if got := reg.Names(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}

	for _, name := range want {
		if _, err := reg.Lookup(name); err != nil {
			t.Errorf("Lookup(%q): %v", name, err)
		}
	}
}

func TestRegisterReturnsFirstErrorAndPreservesEarlierRegistration(t *testing.T) {
	reg := agent.NewRegistry()
	if err := Register(reg, config.Default()); err != nil {
		t.Fatal(err)
	}

	err := Register(reg, config.Default())
	if !errors.Is(err, agent.ErrDuplicateBackend) {
		t.Fatalf("Register duplicate error = %v, want ErrDuplicateBackend", err)
	}
	if got, want := reg.Names(), []string{"kiro", "opencode"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() after failed Register = %v, want %v", got, want)
	}
}

func TestNewSucceedsWhenExecutablesAreMissing(t *testing.T) {
	runtime, err := New(config.Default())
	if err != nil {
		t.Fatal(err)
	}

	_, err = runtime.Run(context.Background(), "definitely-missing-backend", agent.RunSpec{})
	if !errors.Is(err, agent.ErrUnknownBackend) {
		t.Fatalf("Run unknown backend error = %v, want ErrUnknownBackend", err)
	}
}

func TestMissingReturnsSortedUnavailableBackends(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	reg := agent.NewRegistry()
	for _, adapter := range []agent.Adapter{
		&testAdapter{name: "zebra", executable: "missing-zebra-backend"},
		&testAdapter{name: "present", executable: executable},
		&testAdapter{name: "alpha", executable: "missing-alpha-backend"},
	} {
		if err := reg.Register(adapter); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := Missing(reg), []string{"alpha", "zebra"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Missing() = %v, want %v", got, want)
	}
}

func TestMissingNilRegistry(t *testing.T) {
	if got := Missing(nil); got != nil {
		t.Fatalf("Missing(nil) = %v, want nil", got)
	}
}

type testAdapter struct {
	name       string
	executable string
}

func (a *testAdapter) Name() string       { return a.name }
func (a *testAdapter) Executable() string { return a.executable }
func (a *testAdapter) Run(context.Context, agent.RunSpec) (agent.Handle, error) {
	return "", nil
}
func (a *testAdapter) Complete(context.Context, agent.Handle) (agent.Status, error) {
	return agent.StatusCompleted, nil
}
func (a *testAdapter) Diff(context.Context, agent.Handle) ([]agent.FileDiff, error) {
	return nil, nil
}
func (a *testAdapter) Output(context.Context, agent.Handle) (string, error) { return "", nil }
func (a *testAdapter) Delete(context.Context, agent.Handle) error           { return nil }
func (a *testAdapter) UsageLimited(context.Context, agent.Handle) bool      { return false }
