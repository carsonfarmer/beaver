// Package session holds the runtime state of an ACP session and the
// projection that derives that state from a storage event chain.
//
// State is derived, not authoritative. The authoritative record is the
// event history in pkg/storage. State is what the agent keeps in memory
// to make calls against an LLM; on load, it is rebuilt by replaying the
// active lineage through [Project].
package session

import (
	"context"

	"charm.land/fantasy"
)

// State is per-session runtime state derived from the event chain.
type State struct {
	Cwd          string
	Title        string
	UpdatedAt    string
	Model        string // "provider/model"
	ThoughtLevel string
	History      []fantasy.Message
	UsageUsed    int64
	UsageSize    int64

	// Not persisted — set at runtime.
	SystemPrompt string             `json:"-"`
	Cancel       context.CancelFunc `json:"-"`
}
