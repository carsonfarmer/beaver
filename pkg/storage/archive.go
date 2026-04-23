package storage

import (
	acp "github.com/ironpark/go-acp"
)

// Archive is the catalog and durable log for session event histories.
//
// Three concerns, one interface:
//   - Catalog ops (Create, Delete, List) over the set of sessions.
//   - Session log ops (Append, Tip, Events) on a single session's records.
//   - Resource cleanup (Close).
//
// Lineage traversal — walking parent pointers across sessions to replay a
// full chain — lives in the free function [Lineage], not on this interface.
// The catalog is the set of sessions themselves, not the lineage graph.
type Archive interface {
	Create(id acp.SessionID, parent EventID, info acp.SessionInfo) error
	Delete(id acp.SessionID) error
	List() ([]acp.SessionInfo, error)

	Append(id acp.SessionID, parent EventID, update acp.SessionUpdate) (EventID, error)
	Tip(id acp.SessionID) EventID
	Events(id acp.SessionID) ([]Event, error)

	Close() error
}
