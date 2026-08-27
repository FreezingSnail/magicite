package exec

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	osexec "os/exec"
	"sync"
	"time"
)

// Spec describes a shell-free session process.
type Spec struct {
	Dir  string
	Name string
	Args []string
	Env  []string
}

// Session owns a running process and its streaming output pipes.
type Session struct {
	cmd    *osexec.Cmd
	stdout io.ReadCloser
	stderr io.ReadCloser

	done chan struct{}

	waitOnce sync.Once
	exitCode int
	waitErr  error

	terminateMu sync.Mutex
}

// Start starts spec in an isolated process group. The child receives only
// spec.Env; a nil environment is an explicitly empty environment.
func Start(ctx context.Context, spec Spec) (*Session, error) {
	if ctx == nil {
		return nil, errors.New("nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if spec.Name == "" {
		return nil, errors.New("empty program name")
	}
	info, err := os.Stat(spec.Dir)
	if err != nil {
		return nil, fmt.Errorf("stat working directory %q: %w", spec.Dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("working directory %q is not a directory", spec.Dir)
	}
	name, err := osexec.LookPath(spec.Name)
	if err != nil {
		return nil, fmt.Errorf("resolve program %q: %w", spec.Name, err)
	}

	cmd := osexec.Command(name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = append([]string{}, spec.Env...)
	configureProcessGroup(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("create stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start program %q: %w", name, err)
	}

	session := &Session{
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
		done:   make(chan struct{}),
	}
	go session.terminateOnCancel(ctx)
	return session, nil
}

// Stdout returns the child standard-output stream.
func (s *Session) Stdout() io.Reader {
	return sessionReader{reader: s.stdout}
}

// Stderr returns the child standard-error stream.
func (s *Session) Stderr() io.Reader {
	return sessionReader{reader: s.stderr}
}

type sessionReader struct {
	reader io.Reader
}

func (r sessionReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if errors.Is(err, os.ErrClosed) {
		return n, io.EOF
	}
	return n, err
}

// PID returns the child process ID.
func (s *Session) PID() int {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return 0
	}
	return s.cmd.Process.Pid
}

// Wait waits for the child, closes its output pipes, and records its result.
// A non-zero child exit is returned as data, not an error.
func (s *Session) Wait() (int, error) {
	s.waitOnce.Do(func() {
		err := s.cmd.Wait()
		s.exitCode = -1
		var exitErr *osexec.ExitError
		if err == nil || errors.As(err, &exitErr) {
			s.exitCode = s.cmd.ProcessState.ExitCode()
			err = nil
		}
		s.waitErr = err
		_ = s.stdout.Close()
		_ = s.stderr.Close()
		close(s.done)
	})
	return s.exitCode, s.waitErr
}

// Terminate stops the session process group, waiting grace before escalation.
// It does nothing after Wait has recorded an exit.
func (s *Session) Terminate(grace time.Duration) error {
	select {
	case <-s.done:
		return nil
	default:
	}

	s.terminateMu.Lock()
	defer s.terminateMu.Unlock()
	select {
	case <-s.done:
		return nil
	default:
	}
	go s.Wait()
	return terminateProcessGroup(context.Background(), s.cmd.Process, grace)
}

// Done closes after Wait records the exit result.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

func (s *Session) terminateOnCancel(ctx context.Context) {
	select {
	case <-ctx.Done():
		_ = s.Terminate(terminationGrace)
		_, _ = s.Wait()
	case <-s.done:
	}
}
