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
