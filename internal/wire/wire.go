package wire

import (
	"encoding/json"
	"time"
)

const Schema = 1

type Request struct {
	Schema  int             `json:"schema"`
	ID      string          `json:"id"`
	Command string          `json:"command"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type Response struct {
	Schema int             `json:"schema"`
	ID     string          `json:"id"`
	Result json.RawMessage `json:"result,omitempty"`
	Err    *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
}

type Code string

const (
	CodeBadRequest     Code = "bad_request"
	CodeUnknownCommand Code = "unknown_command"
	CodeNotFound       Code = "not_found"
	CodeConflict       Code = "conflict"
	CodeUnavailable    Code = "unavailable"
	CodeSchemaMismatch Code = "schema_mismatch"
	CodeInternal       Code = "internal"
)

func (c Code) ExitStatus() int {
	switch c {
	case CodeBadRequest, CodeUnknownCommand:
		return 2
	case CodeUnavailable:
		return 3
	case CodeNotFound:
		return 4
	case CodeConflict:
		return 5
	case CodeSchemaMismatch:
		return 6
	default:
		return 1
	}
}

type Kind string

const (
	KindPickup   Kind = "pickup"
	KindComplete Kind = "complete"
	KindLand     Kind = "land"
	KindClose    Kind = "close"
	KindReview   Kind = "review"
	KindVerdict  Kind = "verdict"
	KindRecovery Kind = "recovery"
	KindWarn     Kind = "warn"
	KindError    Kind = "error"
)

func ParseKind(s string) (Kind, bool) {
	kind := Kind(s)
	switch kind {
	case KindPickup, KindComplete, KindLand, KindClose, KindReview, KindVerdict, KindRecovery, KindWarn, KindError:
		return kind, true
	default:
		return "", false
	}
}

type Event struct {
	Schema int               `json:"schema"`
	Seq    uint64            `json:"seq"`
	Time   time.Time         `json:"time"`
	Kind   Kind              `json:"kind"`
	Level  string            `json:"level"`
	Repo   string            `json:"repo,omitempty"`
	Task   string            `json:"task,omitempty"`
	Seat   string            `json:"seat,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

type SubscribeParams struct {
	Since  uint64 `json:"since"`
	Follow *bool  `json:"follow,omitempty"`
}
