// Package extensions discovers project context and builds the system prompt.
// See https://agents.md and https://agentskills.io
package extensions

import (
	"context"
	"strings"

	acp "github.com/ironpark/go-acp"
)

// DefaultPrompt is the default base system prompt.
const DefaultPrompt = "You are a helpful coding assistant. Use the provided tools to read files, write files, and execute commands."

// Provider contributes a section to the system prompt.
// Return an empty string to contribute nothing for this session.
type Provider func(ctx context.Context, cwd string, sid acp.SessionID) (string, error)

// Assemble runs all providers and joins their output into a single prompt.
func Assemble(ctx context.Context, cwd string, sid acp.SessionID, providers []Provider) (string, error) {
	var parts []string
	for _, p := range providers {
		s, err := p(ctx, cwd, sid)
		if err != nil {
			return "", err
		}
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "\n\n"), nil
}
