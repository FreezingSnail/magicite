package dispatch

import (
	"context"
	"fmt"
	"time"

	"github.com/connorfranc/magicite/internal/logging"
)

const defaultPollInterval = 30 * time.Second

// StopFunc stops a dispatcher and reports when its live sessions are gone.
type StopFunc func(context.Context, bool) <-chan struct{}

// LifecycleError reports lifecycle wiring or invocation failures.
type LifecycleError struct {
	Operation string
	Reason    string
}

func (e *LifecycleError) Error() string {
	return fmt.Sprintf("dispatch: %s: %s", e.Operation, e.Reason)
}

type completionCallbackRunner interface {
	OnComplete(func(string, Outcome))
}

type phaseCallbackRunner interface {
	OnPhase(func(string, string))
}

// Start clears stale dispatch state, begins polling, and returns its stop hook.
func (d *Dispatcher) Start(ctx context.Context) (StopFunc, error) {
	if ctx == nil {
		return nil, &LifecycleError{Operation: "start", Reason: "nil context"}
	}

	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()
	if d.running {
		return d.Stop, nil
	}
	if !d.callbacksInstalled {
		complete, ok := d.runner.(completionCallbackRunner)
		if !ok {
			return nil, &LifecycleError{Operation: "start", Reason: "runner does not register completion callbacks"}
		}
		phase, ok := d.runner.(phaseCallbackRunner)
		if !ok {
			return nil, &LifecycleError{Operation: "start", Reason: "runner does not register phase callbacks"}
		}
		complete.OnComplete(func(handle string, outcome Outcome) {
			d.OnComplete(context.Background(), handle, outcome)
		})
		phase.OnPhase(func(handle, phase string) {
			d.SetPhase(handle, phase)
		})
		d.callbacksInstalled = true
	}

	d.ClearFreeze(ctx)
	interval := d.pollInterval()
	ticker := d.clock.Ticker(interval)
	tickCtx, cancel := context.WithCancel(context.Background())
	d.ticker = ticker
	d.tickCancel = cancel
	d.running = true
	d.log(logging.Info, "start", map[string]any{
		"interval": interval,
		"cap":      d.RoleCap(Implementer),
	})
	d.runTick(tickCtx)
	go d.poll(tickCtx, ticker)
	return d.Stop, nil
}

func (d *Dispatcher) pollInterval() time.Duration {
	if seconds := d.config.Fleet.PollInterval; seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return defaultPollInterval
}

func (d *Dispatcher) poll(ctx context.Context, ticker Ticker) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.Chan():
			d.runTick(ctx)
		}
	}
}

func (d *Dispatcher) runTick(ctx context.Context) {
	defer func() {
		if recovered := recover(); recovered != nil {
			d.log(logging.Error, "tick-error", map[string]any{"error": fmt.Sprint(recovered)})
		}
	}()
	d.Tick(ctx)
}

// ClearFreeze makes a restarted dispatcher eligible to pick up work again.
func (d *Dispatcher) ClearFreeze(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	d.stateMu.Lock()
	draining, inFlight, pending := d.draining, d.tickInFlight, d.pendingNotify
	d.draining = false
	d.tickInFlight = false
	d.pendingNotify = false
	d.stateMu.Unlock()
	if !draining && !inFlight && !pending {
		return
	}
	d.log(logging.Warn, "start-cleared-freeze", map[string]any{
		"draining": draining, "tick_in_flight": inFlight, "pending_notify": pending,
	})
	_ = d.beads.CancelAll(ctx)
}

// Stop halts polling. A soft stop waits for owned sessions; a hard stop kills them.
func (d *Dispatcher) Stop(ctx context.Context, hard bool) <-chan struct{} {
	if ctx == nil {
		ctx = context.Background()
	}
	d.lifecycleMu.Lock()
	if d.ticker != nil {
		d.ticker.Stop()
		d.ticker = nil
	}
	if d.tickCancel != nil {
		d.tickCancel()
		d.tickCancel = nil
	}
	d.running = false

	d.stateMu.Lock()
	d.tickInFlight = false
	if hard {
		d.draining = true
	}
	d.stateMu.Unlock()
	_ = d.beads.CancelAll(ctx)

	if hard {
		d.lifecycleMu.Unlock()
		d.HandoffLive(ctx)
		return closedDrain()
	}
	if d.Idle() {
		d.lifecycleMu.Unlock()
		return closedDrain()
	}
	d.stateMu.Lock()
	d.draining = true
	d.stateMu.Unlock()
	if d.drainDone == nil || d.drainClosed {
		d.drainDone = make(chan struct{})
		d.drainClosed = false
	}
	done := d.drainDone
	d.log(logging.Info, "stop", map[string]any{"mode": "drain", "sessions": len(d.Sessions())})
	d.lifecycleMu.Unlock()
	return done
}

func closedDrain() <-chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}

// HandoffLive aborts review state, kills session trees, and forgets live sessions.
func (d *Dispatcher) HandoffLive(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	sessions := d.Sessions()
	for _, session := range sessions {
		if session.Role == Reviewer {
			_ = d.gate.AbortReview(ctx, session.Handle, "hard stop")
		}
		_ = d.runner.Delete(ctx, session.Handle)
		if session.Role != Reviewer {
			_ = d.beads.Release(ctx, session.Repo, session.Task)
		}
		d.Remove(session.Handle)
	}
	d.completeDrain()
}

// Draining reports whether new polling pickup is suspended for a soft stop.
func (d *Dispatcher) Draining() bool {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	return d.draining
}

func (d *Dispatcher) completeDrain() {
	if !d.Idle() {
		return
	}
	d.lifecycleMu.Lock()
	defer d.lifecycleMu.Unlock()
	if d.drainDone != nil && !d.drainClosed {
		close(d.drainDone)
		d.drainClosed = true
	}
}
