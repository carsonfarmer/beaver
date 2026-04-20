package eventlog

import (
	"context"

	acp "github.com/ironpark/go-acp"
)

// LoggingClient wraps an acp.Client and appends every SessionUpdate to the
// session's event log before forwarding to the underlying client.
//
// Delta chunk variants (AgentMessageChunk, AgentThoughtChunk) are skipped;
// callers persist the coalesced content explicitly on stream end.
type LoggingClient struct {
	acp.Client
	Store Store
}

func (c *LoggingClient) SessionUpdate(ctx context.Context, n *acp.SessionNotification) error {
	acp.MatchSessionUpdate(&n.Update, acp.SessionUpdateMatcher[any]{
		AgentMessageChunk: func(acp.SessionUpdateAgentMessageChunk) any { return nil },
		AgentThoughtChunk: func(acp.SessionUpdateAgentThoughtChunk) any { return nil },
		Default: func() any {
			if log, err := c.Store.Open(n.SessionID); err == nil {
				log.Append(ctx, n.Update)
			}
			return nil
		},
	})
	return c.Client.SessionUpdate(ctx, n)
}
