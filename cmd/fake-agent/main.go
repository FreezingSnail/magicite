// Command fake-agent is a hermetic agent process double for parity tests.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/FreezingSnail/magicite/internal/testenv"
)

const childArgument = "--magicite-fake-agent-child"

func main() {
	if len(os.Args) == 2 && os.Args[1] == childArgument {
		runChild()
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func run() error {
	if err := testenv.Record(os.Getenv("MAGICITE_TRACE"), os.Args, workingDirectory()); err != nil {
		return fmt.Errorf("record fake agent call: %w", err)
	}
	plan, err := parseArgs(os.Args[1:])
	if err != nil {
		return err
	}
	scenario, err := testenv.AgentScenario(os.Getenv("MAGICITE_FAKE_AGENT_STORE"), plan, os.Getenv("MAGICITE_FAKE_AGENT_SCENARIO"))
	if err != nil {
		return fmt.Errorf("read fake agent scenario: %w", err)
	}
	if scenario == "" {
		return errors.New("unrecognized fake agent scenario \"\"")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return runScenario(ctx, scenario)
}

func parseArgs(args []string) (string, error) {
	if len(args) == 0 || args[0] != "chat" {
		return "", unrecognizedArg(args, 0)
	}
	i := 1
	if i < len(args) && args[i] == "--no-interactive" {
		i++
	}
	if i < len(args) && args[i] == "--agent" {
		if i+1 >= len(args) || invalidValue(args[i+1]) {
			return "", unrecognizedArg(args, i)
		}
		i += 2
	}
	if i+1 >= len(args) || args[i] != "--model" || invalidValue(args[i+1]) {
		return "", unrecognizedArg(args, i)
	}
	i += 2
	if i < len(args) && args[i] == "--effort" {
		if i+1 >= len(args) || invalidValue(args[i+1]) {
			return "", unrecognizedArg(args, i)
		}
		i += 2
	}
	if i+1 != len(args)-1 || args[i] != "--trust-all-tools" || invalidValue(args[i+1]) {
		return "", unrecognizedArg(args, i)
	}
	return args[i+1], nil
}

func invalidValue(value string) bool { return value == "" || strings.HasPrefix(value, "--") }

func unrecognizedArg(args []string, index int) error {
	if index >= 0 && index < len(args) {
		return fmt.Errorf("unrecognized fake agent argument %q", args[index])
	}
	return errors.New("unrecognized fake agent arguments")
}

func runScenario(ctx context.Context, scenario string) error {
	switch scenario {
	case "complete":
		return complete(ctx, "ses_completed", "")
	case "review-approved":
		return complete(ctx, "ses_review_approved", "REVIEW: APPROVED")
	case "review-drift":
		return complete(ctx, "ses_review_drift", "REVIEW: DRIFT: repair the review finding")
	case "review-unparseable":
		return complete(ctx, "ses_review_unparseable", "review complete without marker")
	case "denied":
		return emitAll(ctx, "ses_denied", []string{
			`{"type":"step_start","sessionID":"ses_denied"}`,
			`{"type":"tool_use","sessionID":"ses_denied","part":{"state":{"status":"error","error":"permission denied"}}}`,
		}, 0)
	case "limited":
		return emitAll(ctx, "ses_limited", []string{
			`{"type":"step_start","sessionID":"ses_limited"}`,
			`{"type":"session.error","sessionID":"ses_limited","error":{"message":"provider usage limit reached"}}`,
		}, 0)
	case "failed":
		if err := emitAll(ctx, "ses_failed", []string{
			`{"type":"step_start","sessionID":"ses_failed"}`,
			`{"type":"step_finish","sessionID":"ses_failed","part":{"reason":"error"}}`,
		}, 0); err != nil {
			return err
		}
		return errors.New("fake agent failed")
	case "hang":
		return hang(ctx)
	case "slow":
		delay, err := slowDelay()
		if err != nil {
			return err
		}
		return emitAll(ctx, "ses_slow", []string{
			`{"type":"step_start","sessionID":"ses_slow"}`,
			`{"type":"step_finish","sessionID":"ses_slow","part":{"reason":"stop"}}`,
		}, delay)
	default:
		return fmt.Errorf("unrecognized fake agent scenario %q", scenario)
	}
}

func complete(ctx context.Context, sessionID, transcript string) error {
	lines := []string{
		fmt.Sprintf(`{"type":"step_start","sessionID":%q}`, sessionID),
	}
	if transcript != "" {
		lines = append(lines, transcript)
	}
	lines = append(lines, fmt.Sprintf(`{"type":"step_finish","sessionID":%q,"part":{"reason":"stop"}}`, sessionID))
	return emitAll(ctx, sessionID, lines, 0)
}

func emitAll(ctx context.Context, sessionID string, lines []string, delay time.Duration) error {
	for index, line := range lines {
		if index > 0 && delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
		}
		if ctx.Err() != nil {
			return nil
		}
		if err := emit(sessionID, line); err != nil {
			return err
		}
	}
	return nil
}

func emit(sessionID, line string) error {
	written, err := io.WriteString(os.Stdout, line+"\n")
	if err != nil {
		return err
	}
	if written != len(line)+1 {
		return io.ErrShortWrite
	}
	return testenv.RecordAgentEvent(os.Getenv("MAGICITE_FAKE_AGENT_EVENTS"), sessionID, line)
}

func hang(ctx context.Context) error {
	const sessionID = "ses_hang"
	if err := emit(sessionID, `{"type":"step_start","sessionID":"ses_hang"}`); err != nil {
		return err
	}
	child := exec.Command(os.Args[0], childArgument)
	child.Stdout, child.Stderr = io.Discard, io.Discard
	if err := child.Start(); err != nil {
		return fmt.Errorf("start fake agent child: %w", err)
	}
	if err := testenv.RecordAgentChildPID(os.Getenv("MAGICITE_FAKE_AGENT_PIDS"), child.Process.Pid); err != nil {
		_ = child.Process.Kill()
		_ = child.Wait()
		return fmt.Errorf("record fake agent child: %w", err)
	}
	<-ctx.Done()
	_ = child.Process.Signal(syscall.SIGTERM)
	_ = child.Wait()
	return nil
}

func slowDelay() (time.Duration, error) {
	value := os.Getenv("MAGICITE_FAKE_AGENT_DELAY")
	if value == "" {
		value = os.Getenv("MAGICITE_FAKE_AGENT_SLOW_DELAY")
	}
	if value == "" {
		return 50 * time.Millisecond, nil
	}
	delay, err := time.ParseDuration(value)
	if err != nil || delay < 0 {
		return 0, fmt.Errorf("unrecognized fake agent delay %q", value)
	}
	return delay, nil
}

func runChild() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}

func workingDirectory() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	return dir
}
