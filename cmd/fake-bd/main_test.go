package main

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/connorfranc/magicite/internal/testenv"
)

func TestFakeBDRejectsUnknownSubcommand(t *testing.T) {
	env := testenv.New(t)
	testenv.NewBD(t, env)
	command := exec.Command(env.Bin("bd"), "invent")
	command.Dir, command.Env = env.Root, env.Env()
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("unknown command succeeded")
	}
	if !strings.Contains(string(output), `unknown bd subcommand "invent"`) {
		t.Errorf("output = %q", output)
	}
}

func TestFakeBDRejectsWrongReadyArgv(t *testing.T) {
	env := testenv.New(t)
	testenv.NewBD(t, env)
	command := exec.Command(env.Bin("bd"), "ready", "--json")
	command.Dir, command.Env = env.Root, env.Env()
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("wrong ready argv succeeded")
	}
	if !strings.Contains(string(output), "invalid bd ready arguments") {
		t.Errorf("output = %q", output)
	}
}

func TestFakeBDAcceptsPNotationPriority(t *testing.T) {
	env := testenv.New(t)
	fake := testenv.NewBD(t, env)
	fake.Seed(testenv.Bead{ID: "epic-1", Status: "open", IssueType: "epic"})
	command := exec.Command(env.Bin("bd"), "create", "drift fix", "--type", "task", "--silent", "--parent", "epic-1", "--labels", "drift-fix", "--priority", "P1")
	command.Dir, command.Env = env.Root, env.Env()
	output, err := command.Output()
	if err != nil {
		t.Fatalf("create P1 priority: %v", err)
	}
	created, ok := fake.Bead(strings.TrimSpace(string(output)))
	if !ok || created.Priority != 1 || created.Parent != "epic-1" {
		t.Fatalf("created bead = %#v, found = %t", created, ok)
	}
}
