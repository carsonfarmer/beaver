// Package tools defines the agent tools that proxy to ACP client methods.
package tools

import (
	"context"
	"path/filepath"

	acp "github.com/ironpark/go-acp"
)

// ToolKinds maps tool names to their ACP tool kind.
var ToolKinds = map[string]acp.ToolKind{
	ReadName:    acp.ToolKindRead,
	WriteName:   acp.ToolKindEdit,
	ExecuteName: acp.ToolKindExecute,
	PlanName:    acp.ToolKindThink,
}

// --- Session ID context ---

type sessionIDKey struct{}
type cwdKey struct{}

// WithSessionID returns a context carrying the given session ID.
func WithSessionID(ctx context.Context, id acp.SessionID) context.Context {
	return context.WithValue(ctx, sessionIDKey{}, id)
}

// SessionIDFrom extracts the session ID from the context.
func SessionIDFrom(ctx context.Context) acp.SessionID {
	id, _ := ctx.Value(sessionIDKey{}).(acp.SessionID)
	return id
}

// WithCwd returns a context carrying the working directory.
func WithCwd(ctx context.Context, cwd string) context.Context {
	return context.WithValue(ctx, cwdKey{}, cwd)
}

// RelPath returns path relative to the context's cwd, falling back to the absolute path.
func RelPath(ctx context.Context, path string) string {
	if cwd, _ := ctx.Value(cwdKey{}).(string); cwd != "" {
		if rel, err := filepath.Rel(cwd, path); err == nil {
			return rel
		}
	}
	return path
}

