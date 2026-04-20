// Package eventlog provides an append-only event log for session persistence.
// Events are standard ACP SessionUpdate values wrapped in a thin envelope.
// Each event links to a parent event, forming a tree: linear usage is a
// simple chain; branches arise when an event's ParentID is not the current
// tip (e.g. a rewound user prompt).
package eventlog

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	acp "github.com/ironpark/go-acp"
)

// Event is one line of the log: UUIDv7 ID + parent link + payload. The
// UUIDv7 supplies both ordering and a millisecond-precision timestamp.
// ParentID is the predecessor in the tree (uuid.Nil only on the root).
//
// Exactly one of Info or Update is set. Info is carried only by the root
// primer; Update carries every subsequent event.
type Event struct {
	ID       uuid.UUID          `json:"id"`
	ParentID uuid.UUID          `json:"parentId,omitempty"`
	Info     *acp.SessionInfo   `json:"info,omitempty"`
	Update   *acp.SessionUpdate `json:"update,omitempty"`
}

// EventLog is an append-only, ordered event store for a single session.
type EventLog interface {
	// Append adds an ACP SessionUpdate event. ParentID defaults to the
	// current tip; an update may override via _meta.parent_event_id.
	Append(ctx context.Context, update acp.SessionUpdate) error

	// Read returns the chain of events from the root to the given tip,
	// in root-to-tip order. Pass uuid.Nil to read from the current tip.
	Read(ctx context.Context, tip uuid.UUID) ([]Event, error)
}

// Store manages EventLogs across sessions.
type Store interface {
	// Create makes a new log seeded with the primer SessionInfo.
	Create(id acp.SessionID, info acp.SessionInfo) (EventLog, error)

	// Open returns an existing log. Returns an error if the log does not exist.
	Open(id acp.SessionID) (EventLog, error)

	// Delete removes a log.
	Delete(id acp.SessionID) error

	// List returns all session IDs that have logs.
	List() ([]acp.SessionID, error)
}

// parentFromMeta extracts _meta.parent_event_id from a SessionUpdate, if set.
// Used by Append to honor rewind intent recorded on the update itself.
func parentFromMeta(update acp.SessionUpdate) (uuid.UUID, bool) {
	raw, err := json.Marshal(update)
	if err != nil {
		return uuid.Nil, false
	}
	var peek struct {
		Meta struct {
			ParentEventID string `json:"parent_event_id"`
		} `json:"_meta"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil || peek.Meta.ParentEventID == "" {
		return uuid.Nil, false
	}
	p, err := uuid.Parse(peek.Meta.ParentEventID)
	if err != nil {
		return uuid.Nil, false
	}
	return p, true
}

// walkChain traces parent links from tip back to the root and returns the
// chain in root-to-tip order.
func walkChain(byID map[uuid.UUID]Event, tip uuid.UUID) []Event {
	var chain []Event
	for cur := tip; cur != uuid.Nil; {
		ev, ok := byID[cur]
		if !ok {
			break
		}
		chain = append(chain, ev)
		cur = ev.ParentID
	}
	for i, j := 0, len(chain)-1; i < j; i, j = i+1, j-1 {
		chain[i], chain[j] = chain[j], chain[i]
	}
	return chain
}
