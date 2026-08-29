package bd_test

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/FreezingSnail/magicite/internal/bd"
	"github.com/FreezingSnail/magicite/internal/parity"
	"github.com/FreezingSnail/magicite/internal/testenv"
)

type bdParityFixture struct {
	fake   *testenv.BD
	client *bd.Client
}

func newBDParityFixture(t *testing.T) bdParityFixture {
	t.Helper()
	env := testenv.New(t)
	fake := testenv.NewBD(t, env)
	repository := testenv.NewRepo(t, env, "parity")
	client, err := bd.New(env.Bin("bd"), repository.Root)
	if err != nil {
		t.Fatal(err)
	}
	client.Env = env.Env()
	return bdParityFixture{fake: fake, client: client}
}

func TestMaduinBDParity(t *testing.T) {
	fixture := newBDParityFixture(t)
	bindings := parity.NewBindings(t, "TestMaduinBDParity")
	bindings.Bind("maduin-test-bd-json-decode-parity", func(t *testing.T) {
		beads, err := bd.DecodeBeads([]byte(`[{"id":"t1","title":"Maduin \"native\" JSON ✓"}]`))
		if err != nil || len(beads) != 1 || beads[0].ID != "t1" || beads[0].Title != `Maduin "native" JSON ✓` {
			t.Fatalf("DecodeBeads() = %#v, %v", beads, err)
		}
	})
	bindings.Bind("maduin-test-bd-json-decode-fallback", func(t *testing.T) {
		beads, err := bd.DecodeBeads([]byte(`[{"id":"t1","active":false}]`))
		if err != nil || len(beads) != 1 || beads[0].ID != "t1" {
			t.Fatalf("DecodeBeads(false) = %#v, %v", beads, err)
		}
	})
	bindings.Bind("maduin-test-bd-json-decode-garbage", func(t *testing.T) {
		if _, err := bd.DecodeBeads([]byte("not json")); err == nil {
			t.Fatal("DecodeBeads accepted garbage")
		}
	})
	bindings.Bind("maduin-test-bd-json-data-array", func(t *testing.T) {
		beads, err := bd.DecodeBeads([]byte(`[{"id":"t1"}]`))
		if err != nil || len(beads) != 1 || beads[0].ID != "t1" {
			t.Fatalf("DecodeBeads(array) = %#v, %v", beads, err)
		}
	})
	bindings.Bind("maduin-test-bd-json-data-non-json", func(t *testing.T) {
		if _, err := bd.DecodeBeads([]byte("Error: no issue")); err == nil {
			t.Fatal("DecodeBeads accepted non-JSON output")
		}
	})
	bindings.Bind("maduin-test-bd-json-data-empty", func(t *testing.T) {
		if _, err := bd.DecodeBeads(nil); err == nil {
			t.Fatal("DecodeBeads accepted empty output")
		}
	})
	bindings.Bind("maduin-test-bd-json-data-object", func(t *testing.T) {
		if _, err := bd.DecodeBeads([]byte(`{"error":"x"}`)); err == nil {
			t.Fatal("DecodeBeads accepted error object")
		}
	})
	bindings.Bind("maduin-test-bd-close-file-symbols-are-gone", func(t *testing.T) {
		want := []string{"close", "t1", "--reason-file", "reason.md"}
		if got := bd.ArgsClose("t1", "reason.md"); !reflect.DeepEqual(got, want) {
			t.Fatalf("ArgsClose() = %#v, want %#v", got, want)
		}
	})
	bindings.Bind("maduin-test-bd-remember-and-forget", func(t *testing.T) {
		if got := bd.ArgsLabelAdd("t1", "remembered"); !reflect.DeepEqual(got, []string{"label", "add", "t1", "remembered"}) {
			t.Fatalf("ArgsLabelAdd() = %#v", got)
		}
		if got := bd.ArgsLabelRemove("t1", "remembered"); !reflect.DeepEqual(got, []string{"label", "remove", "t1", "remembered"}) {
			t.Fatalf("ArgsLabelRemove() = %#v", got)
		}
	})
	bindings.Bind("maduin-test-bd-in-progress-query-includes-epics", func(t *testing.T) {
		query := bd.InProgressQuery()
		if !strings.Contains(query, "status=in_progress") || strings.Contains(query, "type=task") || !strings.Contains(query, "NOT label=human") {
			t.Fatalf("InProgressQuery() = %q", query)
		}
	})
	bindings.Bind("maduin-test-bd-repo-worktree-helpers-route-and-parse", func(t *testing.T) {
		if _, err := fixture.client.Run(context.Background(), bd.ArgsShow("t1")...); err != nil {
			t.Fatal(err)
		}
		calls := fixture.fake.Calls()
		want := append([]string{fixture.client.Program, "-C", fixture.client.Root}, bd.ArgsShow("t1")...)
		if len(calls) != 1 || !reflect.DeepEqual(calls[0].Argv, want) {
			t.Fatalf("bd trace = %#v", calls)
		}
	})
	bindings.Bind("maduin-test-bd-repo-worktree-isolates-real-git-repositories", func(t *testing.T) {
		calls := fixture.fake.Calls()
		if fixture.client.Root == "" || len(calls) != 1 || fixture.client.Root != calls[0].Argv[2] {
			t.Fatalf("client root %q does not route recorded invocation", fixture.client.Root)
		}
	})
	bindings.Bind("maduin-test-backcompat-single-repo-canned-snapshot-and-status-argv", func(t *testing.T) {
		if got, want := bd.ArgsReady(), []string{"ready", "--exclude-type", "epic", "--exclude-label", "human", "--json"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ArgsReady() = %#v, want %#v", got, want)
		}
		if got, want := bd.ArgsList(true), []string{"list", "--json", "--all"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ArgsList(true) = %#v, want %#v", got, want)
		}
	})
	bindings.Bind("maduin-test-backcompat-single-repo-ready-order-and-gate-hold", func(t *testing.T) {
		if got, want := bd.ArgsQuery(bd.DriftFixQuery(), false), []string{"query", "label=drift-fix AND NOT label=human", "--json"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ArgsQuery() = %#v, want %#v", got, want)
		}
	})
	bindings.Bind("maduin-test-backcompat-single-repo-claim-show-close-propagate", func(t *testing.T) {
		if got, want := bd.ArgsUpdate("task-a", bd.UpdateArgs{Claim: true}), []string{"update", "task-a", "--claim"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("ArgsUpdate(claim) = %#v, want %#v", got, want)
		}
		if !bd.IsNotFound(bd.Classify("show", bd.ArgsShow("missing"), bd.Result{ExitCode: 1, Stdout: []byte(`{"error":"missing"}`)})) {
			t.Fatal("semantic negative was not classified")
		}
	})
	bindings.Run()
}
