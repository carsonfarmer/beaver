package agent

import (
	"context"

	acp "github.com/ironpark/go-acp"
)

type clientKey struct{}

// ContextWithClient returns a context carrying the given acp.Client.
func ContextWithClient(ctx context.Context, client acp.Client) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

// ClientFrom extracts the acp.Client from the context.
func ClientFrom(ctx context.Context) acp.Client {
	c, _ := ctx.Value(clientKey{}).(acp.Client)
	return c
}
