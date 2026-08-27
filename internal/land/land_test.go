package land

import (
	"errors"
	"reflect"
	"testing"
)

func TestNewRejectsMissingPorts(t *testing.T) {
	workspace := &fakeWorkspace{}
	runner := newFakeRunner()
	for _, opts := range []Options{
		{Runner: runner},
		{Workspace: workspace},
	} {
		if _, err := New(opts); !errors.Is(err, ErrInvalidOptions) {
			t.Errorf("New(%+v) error = %v, want ErrInvalidOptions", opts, err)
		}
	}
}

func TestNewDefaultsGateAndDiscardsNilLog(t *testing.T) {
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner()})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(pipeline.gateArgv, []string{"make", "check"}) {
		t.Errorf("gate = %q, want [make check]", pipeline.gateArgv)
	}
	pipeline.warnf("ignored %s", "warning")
}

func TestNewCopiesGate(t *testing.T) {
	gate := []string{"go", "test", "./..."}
	pipeline, err := New(Options{Workspace: &fakeWorkspace{}, Runner: newFakeRunner(), Gate: gate})
	if err != nil {
		t.Fatal(err)
	}
	gate[0] = "changed"
	if pipeline.gateArgv[0] != "go" {
		t.Errorf("gate = %q, want independent copy", pipeline.gateArgv)
	}
}
