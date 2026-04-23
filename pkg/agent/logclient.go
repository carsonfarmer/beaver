package agent

import (
	"context"

	"github.com/carsonfarmer/beaver/pkg/storage"

	acp "github.com/ironpark/go-acp"
)

// LoggingClient wraps an acp.Client so every outbound SessionUpdate is also
// appended to the session's durable log before forwarding to the underlying
// client. Streaming delta variants (AgentMessageChunk, AgentThoughtChunk)
// are skipped — callers persist the coalesced content explicitly on stream
// end to keep the log at full-unit granularity.
type LoggingClient struct {
	acp.Client
	Archive storage.Archive
}

func (c *LoggingClient) SessionUpdate(ctx context.Context, n *acp.SessionNotification) error {
	acp.MatchSessionUpdate(&n.Update, acp.SessionUpdateMatcher[any]{
		AgentMessageChunk: func(acp.SessionUpdateAgentMessageChunk) any { return nil },
		AgentThoughtChunk: func(acp.SessionUpdateAgentThoughtChunk) any { return nil },
		Default: func() any {
			c.Archive.Append(n.SessionID, storage.EventID{}, n.Update)
			return nil
		},
	})
	return c.Client.SessionUpdate(ctx, n)
}
