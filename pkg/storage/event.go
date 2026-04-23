// Package storage provides durable, append-only session histories for an
// ACP agent. A session is a named tip plus an append log of immutable events
// created within it. Events form a single-parent chain; parent pointers may
// cross sessions to enable zero-copy forks and mid-chain rewinds. Replay
// follows parent pointers, not file order.
package storage

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	acp "github.com/ironpark/go-acp"
)

// EventID addresses an immutable event by its owning session and its
// sequence number within that session's append log. The zero value means
// "no event" (e.g. the parent of a root event).
type EventID struct {
	Session acp.SessionID
	N       int64
}

// IsZero reports whether e is the zero EventID.
func (e EventID) IsZero() bool { return e.Session == "" }

// String formats as "<session>#<n>". Zero returns "".
func (e EventID) String() string {
	if e.IsZero() {
		return ""
	}
	return string(e.Session) + "#" + strconv.FormatInt(e.N, 10)
}

// ParseEventID parses a fully-qualified "<session>#<n>" into an EventID.
// The empty string parses to the zero EventID.
func ParseEventID(s string) (EventID, error) {
	if s == "" {
		return EventID{}, nil
	}
	sid, num, ok := strings.Cut(s, "#")
	if !ok || sid == "" {
		return EventID{}, fmt.Errorf("event id %q: want session#n", s)
	}
	n, err := strconv.ParseInt(num, 10, 64)
	if err != nil {
		return EventID{}, fmt.Errorf("event id %q: %w", s, err)
	}
	return EventID{Session: acp.SessionID(sid), N: n}, nil
}

// MarshalText / UnmarshalText encode EventID as the string form, which
// encoding/json picks up automatically so the on-disk JSON stays a plain
// string field rather than a nested object.
func (e EventID) MarshalText() ([]byte, error) { return []byte(e.String()), nil }

func (e *EventID) UnmarshalText(text []byte) error {
	parsed, err := ParseEventID(string(text))
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

// Event is an immutable node in the session history chain. The first event
// in a session file is the header and carries Info; all subsequent events
// carry Update. Parent may point to an earlier event in the same session
// (rewind), in another session (fork), or be zero (root).
type Event struct {
	ID        EventID            `json:"id"`
	Parent    EventID            `json:"parent,omitzero"`
	Timestamp time.Time          `json:"ts"`
	Info      *acp.SessionInfo   `json:"info,omitempty"`
	Update    *acp.SessionUpdate `json:"update,omitempty"`
}
