package cli

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

func TestRunUsageAndGlobalFlags(t *testing.T) {
	for _, test := range []struct {
		name string
		args []string
		code int
		err  bool
	}{
		{"empty", nil, 0, false},
		{"help", []string{"--help"}, 0, false},
		{"version", []string{"--version"}, 0, false},
		{"unknown", []string{"missing"}, 2, true},
		{"invalid flag", []string{"--missing"}, 2, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out, err bytes.Buffer
			if got := Run(context.Background(), test.args, &out, &err); got != test.code {
				t.Fatalf("Run(%v) = %d, want %d", test.args, got, test.code)
			}
			if test.err && err.Len() == 0 {
				t.Fatal("stderr empty")
			}
			if !test.err && out.Len() == 0 {
				t.Fatal("stdout empty")
			}
		})
	}
}

func TestRenderers(t *testing.T) {
	var table bytes.Buffer
	if err := EmitTable(&table, []string{"id", "name"}, [][]string{{"1", "long"}}); err != nil {
		t.Fatal(err)
	}
	if got, want := table.String(), "ID  NAME\n1   long\n"; got != want {
		t.Fatalf("table = %q, want %q", got, want)
	}
	table.Reset()
	if err := EmitTable(&table, []string{"id"}, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := table.String(), "ID\n"; got != want {
		t.Fatalf("empty table = %q, want %q", got, want)
	}

	var jsonOut bytes.Buffer
	if err := EmitJSON(&jsonOut, "status", map[string]bool{"running": true}); err != nil {
		t.Fatal(err)
	}
	if got, want := jsonOut.String(), "{\"schema\":1,\"kind\":\"status\",\"data\":{\"running\":true}}\n"; got != want {
		t.Fatalf("JSON = %q, want %q", got, want)
	}
}

func TestFailUsesClientExitStatus(t *testing.T) {
	var err bytes.Buffer
	env := &Env{Err: &err}
	failure := &client.Error{Code: wire.CodeUnavailable, Message: "daemon socket unavailable"}
	if got := Fail(env, failure); got != 3 {
		t.Fatalf("Fail() = %d, want 3", got)
	}
	if got, want := err.String(), "magicite: unavailable: daemon socket unavailable\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
	if errors.Is(failure, nil) {
		t.Fatal("sanity")
	}
}
