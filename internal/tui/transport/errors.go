package transport

import "github.com/FreezingSnail/magicite/internal/wire"

// StreamNoticeKind classifies a non-event stream condition.
type StreamNoticeKind string

const (
	// StreamNoticeEOF reports normal stream termination.
	StreamNoticeEOF StreamNoticeKind = "eof"
	// StreamNoticeGap reports a discontinuity after event delivery began.
	StreamNoticeGap StreamNoticeKind = "gap"
	// StreamNoticeMiss reports events unavailable before initial delivery.
	StreamNoticeMiss StreamNoticeKind = "miss"
	// StreamNoticeSchemaMismatch reports an incompatible daemon protocol.
	StreamNoticeSchemaMismatch StreamNoticeKind = "schema_mismatch"
	// StreamNoticeUnavailable reports a daemon connection failure.
	StreamNoticeUnavailable StreamNoticeKind = "unavailable"
)

// StreamNotice reports a typed stream condition without exposing socket paths.
// Cursor is the highest sequence observed. MissingFrom through MissingTo,
// inclusive, identify an unavailable range for miss and gap notices. Code is
// populated for terminal protocol or transport failures.
type StreamNotice struct {
	Kind        StreamNoticeKind
	Cursor      uint64
	MissingFrom uint64
	MissingTo   uint64
	Code        wire.Code
}
