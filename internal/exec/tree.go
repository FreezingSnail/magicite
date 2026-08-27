package exec

import (
	"context"
	"errors"
	"fmt"
	"os"
	osexec "os/exec"
	"syscall"
	"time"
)

const (
	terminationGrace  = 100 * time.Millisecond
	terminationSettle = 100 * time.Millisecond
)

// SurvivorError reports a process group that remained after forced termination.
type SurvivorError struct {
	PID int
}

// Error returns the surviving process group's leader PID.
func (e *SurvivorError) Error() string {
	return fmt.Sprintf("process group %d survived termination", e.PID)
}

// configureProcessGroup gives cmd an isolated process group led by cmd itself.
func configureProcessGroup(cmd *osexec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// Terminate stops handle's process group. It first sends SIGTERM, then SIGKILL
// after a bounded grace period. A missing process group is already terminated.
func Terminate(ctx context.Context, handle *os.Process) error {
	return terminateProcessGroup(ctx, handle, terminationGrace)
}

func terminateProcessGroup(ctx context.Context, handle *os.Process, grace time.Duration) error {
	if handle == nil || handle.Pid <= 0 {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	alive, err := signalGroup(handle.Pid, syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("signal process group %d gracefully: %w", handle.Pid, err)
	}
	if !alive {
		return nil
	}
	if gone, err := waitForGroup(ctx, handle.Pid, grace); err != nil {
		return fmt.Errorf("wait for graceful process-group termination: %w", err)
	} else if gone {
		return nil
	}

	alive, err = signalGroup(handle.Pid, syscall.SIGKILL)
	if err != nil {
		return fmt.Errorf("force process group %d: %w", handle.Pid, err)
	}
	if !alive {
		return nil
	}
	gone, err := waitForGroup(context.Background(), handle.Pid, terminationSettle)
	if err != nil {
		return fmt.Errorf("wait for forced process-group termination: %w", err)
	}
	if gone {
		return nil
	}
	return &SurvivorError{PID: handle.Pid}
}

func signalGroup(pgid int, signal syscall.Signal) (bool, error) {
	err := syscall.Kill(-pgid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return false, nil
	}
	// Darwin can report EPERM while a just-killed group is being reaped. Treat
	// it as live for polling so termination remains bounded and does not hide a
	// real survivor behind a transient permission result.
	if (signal == 0 || signal == syscall.SIGKILL) && errors.Is(err, syscall.EPERM) {
		return true, nil
	}
	return err == nil, err
}

func waitForGroup(ctx context.Context, pgid int, limit time.Duration) (bool, error) {
	deadline := time.NewTimer(limit)
	defer deadline.Stop()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		alive, err := signalGroup(pgid, 0)
		if err != nil {
			return false, err
		}
		if !alive {
			return true, nil
		}

		select {
		case <-ctx.Done():
			return false, nil
		case <-deadline.C:
			return false, nil
		case <-ticker.C:
		}
	}
}
