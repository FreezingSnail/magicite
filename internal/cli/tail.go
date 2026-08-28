package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/FreezingSnail/magicite/internal/client"
	"github.com/FreezingSnail/magicite/internal/wire"
)

const (
	initialTailBackoff = 100 * time.Millisecond
	maximumTailBackoff = 5 * time.Second
)

// Tail streams daemon events from Since and optionally reconnects after outages.
type Tail struct {
	Since     uint64
	Follow    bool
	Reconnect int
	JSON      bool

	live bool
}

func init() { RegisterTail() }

// RegisterTail adds the tail command to the CLI.
func RegisterTail() {
	Register(Command{Name: "tail", Usage: "tail [--since SEQ] [--follow] [--reconnect N] [--json]", Summary: "stream daemon events", Run: tail})
}

func tail(ctx context.Context, e *Env, args []string) int {
	flags := flag.NewFlagSet("tail", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	since := flags.Uint64("since", 0, "resume after sequence")
	follow := flags.Bool("follow", true, "follow new events")
	reconnect := flags.Int("reconnect", -1, "maximum reconnect attempts")
	asJSON := flags.Bool("json", e.JSON, "write JSON")
	if flags.Parse(args) != nil || flags.NArg() != 0 || *reconnect < -1 {
		return commandUsage(e, "tail")
	}

	live := true
	flags.Visit(func(f *flag.Flag) {
		if f.Name == "since" {
			live = false
		}
	})
	return (Tail{Since: *since, Follow: *follow, Reconnect: *reconnect, JSON: *asJSON, live: live}).Run(ctx, e)
}

// Run renders events while retaining only the next resume position and retry state.
func (t Tail) Run(ctx context.Context, e *Env) int {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil {
		return 1
	}
	if e.Client == nil {
		return Fail(e, errors.New("tail: client is required"))
	}
	if t.Reconnect < -1 {
		return Fail(e, fmt.Errorf("tail: reconnect must be non-negative"))
	}

	position := t.Since
	live := t.live
	backoff := initialTailBackoff
	attempts := 0
	for {
		hadEvent := false
		streamSince := position
		if live {
			streamSince = math.MaxUint64
		}
		_, err := e.Client.Stream(ctx, streamSince, func(event wire.Event) error {
			if !live && event.Seq <= position {
				return nil
			}
			if !live && event.Seq > position+1 {
				if err := renderMissed(e.Out, t.JSON, position+1, event.Seq-1); err != nil {
					return err
				}
			}
			if err := RenderEvent(e.Out, event, t.JSON); err != nil {
				return err
			}
			position = event.Seq
			live = false
			hadEvent = true
			return nil
		}, t.Follow)
		if ctx.Err() != nil {
			return 0
		}
		if err == nil && !t.Follow {
			return 0
		}
		if err != nil {
			var streamErr *client.Error
			if !errors.As(err, &streamErr) || streamErr.Code != wire.CodeUnavailable {
				return Fail(e, err)
			}
		}
		if !t.Follow {
			return Fail(e, err)
		}
		if err == nil || hadEvent {
			backoff = initialTailBackoff
			attempts = 0
		}
		if t.Reconnect >= 0 && attempts >= t.Reconnect {
			return Fail(e, &client.Error{Code: wire.CodeUnavailable, Message: "tail reconnect attempts exhausted"})
		}
		attempts++
		if !sleep(ctx, backoff) {
			return 0
		}
		backoff *= 2
		if backoff > maximumTailBackoff {
			backoff = maximumTailBackoff
		}
	}
}

func sleep(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func renderMissed(w io.Writer, asJSON bool, first, last uint64) error {
	return RenderEvent(w, wire.Event{
		Time:  time.Now(),
		Kind:  wire.KindWarn,
		Level: "warn",
		Fields: map[string]string{
			"missed_first_seq": strconv.FormatUint(first, 10),
			"missed_last_seq":  strconv.FormatUint(last, 10),
		},
	}, asJSON)
}

// RenderEvent writes ev as a line-oriented JSON object or readable plain text.
func RenderEvent(w io.Writer, ev wire.Event, asJSON bool) error {
	if asJSON {
		return json.NewEncoder(w).Encode(ev)
	}

	values := []string{ev.Time.Format(time.RFC3339Nano), ev.Level, string(ev.Kind)}
	for _, column := range []struct {
		name  string
		value string
	}{
		{"repo", ev.Repo},
		{"task", ev.Task},
		{"seat", ev.Seat},
	} {
		if column.value != "" {
			values = append(values, column.name+"="+column.value)
		}
	}
	keys := make([]string, 0, len(ev.Fields))
	for key := range ev.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		values = append(values, key+"="+ev.Fields[key])
	}
	return EmitLine(w, "%s", strings.Join(values, " "))
}
