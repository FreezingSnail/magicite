package dispatch

import (
	"context"

	"github.com/FreezingSnail/magicite/internal/config"
	"github.com/FreezingSnail/magicite/internal/logging"
)

// FallbackRetry starts one bounded retry when a session reaches a usage limit.
func (d *Dispatcher) FallbackRetry(ctx context.Context, session Session, handle string, outcome Outcome) bool {
	if session.FallbackAttempted {
		return false
	}

	limited := outcome == Limited
	if !limited {
		var err error
		limited, err = d.runner.UsageLimited(ctx, handle)
		if err != nil || !limited {
			return false
		}
	}

	fallbackConfig := d.config
	fallbackConfig.Crew.Backend = session.Backend
	fallback, err := config.FallbackModel(fallbackConfig, string(session.Role))
	if err != nil {
		return false
	}

	fields := dispatchFields(session.Repo, session.Task, session.Role, session.Seat)
	fields["backend"] = session.Backend
	fields["fallback"] = fallback
	d.log(logging.Warn, "usage-limit", fields)
	_ = d.beads.Comment(ctx, session.Repo, session.Task, "fallback retry is starting.")

	retry := d.spawnSessionWithResolution(ctx, session.Repo, session.Task, session.Role, session.Seat, "", session.Agent,
		config.Resolution{Backend: session.Backend, Agent: session.Agent, Model: fallback, Effort: session.Effort},
		session.Difficulty, true, false)
	if retry == "" {
		return false
	}

	if session.Role == Reviewer {
		d.gate.NoteSession(retry, session.Repo, session.Task)
	}
	return true
}
