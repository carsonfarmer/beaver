package session

import (
	"context"

	"charm.land/fantasy"
)

// Session holds per-session state derived from the event log.
type Session struct {
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
