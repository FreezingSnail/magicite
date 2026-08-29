package server

import (
	"time"

	"github.com/FreezingSnail/magicite/internal/wire"
)

// Project converts a structured log record into the wire event vocabulary.
func Project(level, kind string, fields map[string]string) (wire.Event, bool) {
	projectedKind, ok := wire.ParseKind(kind)
	if !ok {
		if level != "warn" && level != "error" {
			return wire.Event{}, false
		}
		projectedKind = wire.KindWarn
	}

	event := wire.Event{Kind: projectedKind, Level: level}
	if len(fields) == 0 {
		return event, true
	}

	event.Fields = make(map[string]string, len(fields))
	for key, value := range fields {
		switch key {
		case "repo":
			event.Repo = value
		case "task":
			event.Task = value
		case "seat":
			event.Seat = value
		case "time":
			if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
				event.Time = parsed
			} else {
				event.Fields[key] = value
			}
		default:
			event.Fields[key] = value
		}
	}
	if len(event.Fields) == 0 {
		event.Fields = nil
	}
	return event, true
}
