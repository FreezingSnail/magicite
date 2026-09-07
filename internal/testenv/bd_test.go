package testenv

import (
	"encoding/json"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestBDSeedMutationAndCalls(t *testing.T) {
	env := New(t)
	fake := NewBD(t, env)
	fake.Seed(Bead{ID: "bd-1", Title: "one", Status: "open", IssueType: "task", Labels: []string{"alpha"}}, Bead{ID: "bd-2", Status: "open", IssueType: "epic"})
	if _, stderr, err := runFake(env, "update", "bd-1", "--claim"); err != nil {
		t.Fatalf("claim: %v: %s", err, stderr)
	}
	item, ok := fake.Bead("bd-1")
	if !ok || item.Status != "in_progress" {
		t.Fatalf("Bead(bd-1) = %#v, %v", item, ok)
	}
	if _, stderr, err := runFake(env, "label", "add", "bd-1", "claimed"); err != nil {
		t.Fatalf("label add: %v: %s", err, stderr)
	}
	stdout, stderr, err := runFake(env, "label", "list", "bd-1", "--json")
	if err != nil {
		t.Fatalf("label list: %v: %s", err, stderr)
	}
	var labels []string
	if err := json.Unmarshal(stdout, &labels); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(labels, []string{"alpha", "claimed"}) {
		t.Errorf("labels = %#v", labels)
	}
	calls := fake.Calls()
	if len(calls) != 3 {
		t.Fatalf("Calls() = %#v", calls)
	}
	if got, want := calls[0].Argv[len(calls[0].Argv)-3:], []string{"update", "bd-1", "--claim"}; !reflect.DeepEqual(got, want) {
		t.Errorf("claim argv = %#v", got)
	}
}

func TestBDSemanticNoAndScriptedFailure(t *testing.T) {
	env := New(t)
	fake := NewBD(t, env)
	stdout, stderr, err := runFake(env, "ready", "--exclude-type", "epic", "--exclude-label", "human", "--json")
	if err != nil || string(stdout) != "[]" || len(stderr) != 0 {
		t.Fatalf("ready = %q, %q, %v", stdout, stderr, err)
	}
	fake.FailNext("show", 9, "scripted failure")
	_, stderr, err = runFake(env, "show", "missing", "--json")
	if err == nil || !strings.Contains(string(stderr), "scripted failure") {
		t.Fatalf("show = %v, %q", err, stderr)
	}
	stdout, stderr, err = runFake(env, "show", "missing", "--json")
	if err != nil || string(stdout) != "[]" || len(stderr) != 0 {
		t.Fatalf("second show = %q, %q, %v", stdout, stderr, err)
	}
}

func TestBDStoreWritesSurviveConcurrentClaims(t *testing.T) {
	env := New(t)
	fake := NewBD(t, env)
	fake.Seed(Bead{ID: "bd-1", Status: "open", IssueType: "task"}, Bead{ID: "bd-2", Status: "open", IssueType: "task"})
	commands := []*exec.Cmd{
		exec.Command(env.Bin("bd"), "update", "bd-1", "--claim"),
		exec.Command(env.Bin("bd"), "update", "bd-2", "--claim"),
	}
	for _, command := range commands {
		command.Dir, command.Env = env.Root, env.Env()
		if err := command.Start(); err != nil {
			t.Fatal(err)
		}
	}
	for _, command := range commands {
		if err := command.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []string{"bd-1", "bd-2"} {
		item, ok := fake.Bead(id)
		if !ok || item.Status != "in_progress" {
			t.Errorf("Bead(%q) = %#v, %v", id, item, ok)
		}
	}
}

func runFake(env *Env, args ...string) ([]byte, []byte, error) {
	command := exec.Command(env.Bin("bd"), args...)
	command.Dir, command.Env = env.Root, env.Env()
	var stdout, stderr strings.Builder
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	return []byte(stdout.String()), []byte(stderr.String()), err
}
func TestBDAllRecordJSONPreservesSeedMetadata(t *testing.T) {
	env := New(t)
	fake := NewBD(t, env)
	fake.Seed(Bead{
		ID: "bd-1", Status: "closed", IssueType: "task", CreatedBy: "creator", ClosedAt: "2026-09-07T20:03:00Z", CloseReason: "implemented", DeferredUntil: "2026-09-08T00:00:00Z",
		Labels: []string{"staged", "difficulty:high"}, Comments: []string{"first", "second"},
		Dependencies: []Dependency{{ID: "bd-0", Title: "dependency", Status: "closed", DependencyType: "blocks"}},
	})
	stdout, stderr, err := runFake(env, "list", "--json", "--all")
	if err != nil {
		t.Fatalf("list all: %v: %s", err, stderr)
	}
	var beads []Bead
	if err := json.Unmarshal(stdout, &beads); err != nil {
		t.Fatal(err)
	}
	if len(beads) != 1 || beads[0].CreatedBy != "creator" || beads[0].ClosedAt == "" || beads[0].CloseReason != "implemented" || beads[0].DeferredUntil == "" || !reflect.DeepEqual(beads[0].Labels, []string{"staged", "difficulty:high"}) || !reflect.DeepEqual(beads[0].Comments, []string{"first", "second"}) || len(beads[0].Dependencies) != 1 {
		t.Fatalf("beads = %#v", beads)
	}
}
