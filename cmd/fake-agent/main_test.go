package main

import (
	"bufio"
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/connorfranc/magicite/internal/testenv"
)

func TestScenariosEmitCompleteNDJSONWithExpectedExit(t *testing.T) {
	tests := []struct {
		scenario string
		exitOK   bool
		complete bool
	}{
		{scenario: "complete", exitOK: true, complete: true},
		{scenario: "denied", exitOK: true},
		{scenario: "limited", exitOK: true},
		{scenario: "failed"},
	}
	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			env := testenv.New(t)
			fake := testenv.NewAgent(t, env, "kiro")
			fake.Scenario(test.scenario)
			command := exec.Command(env.Bin("kiro"), "chat", "--no-interactive", "--model", "model", "--trust-all-tools", "plan")
			command.Dir, command.Env = env.Root, env.Env()
			stdout, err := command.Output()
			if (err == nil) != test.exitOK {
				t.Fatalf("exit = %v, want success %t", err, test.exitOK)
			}
			hasCompletion := false
			scanner := bufio.NewScanner(strings.NewReader(string(stdout)))
			for scanner.Scan() {
				var event struct {
					Type string `json:"type"`
					Part struct {
						Reason string `json:"reason"`
					} `json:"part"`
				}
				if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
					t.Fatalf("invalid event %q: %v", scanner.Text(), err)
				}
				hasCompletion = hasCompletion || (event.Type == "step_finish" && event.Part.Reason == "stop")
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			if hasCompletion != test.complete {
				t.Errorf("completion = %t, want %t; stream=%q", hasCompletion, test.complete, stdout)
			}
		})
	}
}

func TestRejectsUnknownArgumentsAndScenarios(t *testing.T) {
	env := testenv.New(t)
	fake := testenv.NewAgent(t, env, "kiro")
	command := exec.Command(env.Bin("kiro"), "invent")
	command.Dir, command.Env = env.Root, env.Env()
	output, err := command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "unrecognized fake agent argument") {
		t.Fatalf("unknown argv = %v, %q", err, output)
	}
	fake.Scenario("invented")
	command = exec.Command(env.Bin("kiro"), "chat", "--model", "model", "--trust-all-tools", "plan")
	command.Dir, command.Env = env.Root, env.Env()
	output, err = command.CombinedOutput()
	if err == nil || !strings.Contains(string(output), "unrecognized fake agent scenario") {
		t.Fatalf("unknown scenario = %v, %q", err, output)
	}
}

func TestSlowUsesConfiguredDelay(t *testing.T) {
	env := testenv.New(t)
	fake := testenv.NewAgent(t, env, "kiro")
	fake.Scenario("slow")
	command := exec.Command(env.Bin("kiro"), "chat", "--model", "model", "--trust-all-tools", "plan")
	command.Dir, command.Env = env.Root, append(env.Env(), "MAGICITE_FAKE_AGENT_DELAY=20ms")
	started := time.Now()
	if output, err := command.Output(); err != nil || strings.Count(string(output), "\n") != 2 {
		t.Fatalf("slow = %q, %v", output, err)
	}
	if elapsed := time.Since(started); elapsed < 20*time.Millisecond {
		t.Errorf("slow elapsed %v, want at least 20ms", elapsed)
	}
}
