package agent

import (
	"context"
	"sync"

	acp "github.com/ironpark/go-acp"
)

// BroadcastClient fans out acp.Client calls to all registered clients.
// SessionUpdate notifications go to everyone.
// Requests (ReadTextFile, etc.) go to everyone; the first response wins.
type BroadcastClient struct {
	mu      sync.RWMutex
	clients []acp.Client
}

func NewBroadcastClient() *BroadcastClient {
	return &BroadcastClient{}
}

func (b *BroadcastClient) Add(c acp.Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.clients = append(b.clients, c)
}

func (b *BroadcastClient) Remove(c acp.Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, existing := range b.clients {
		if existing == c {
			b.clients = append(b.clients[:i], b.clients[i+1:]...)
			return
		}
	}
}

func (b *BroadcastClient) SessionUpdate(ctx context.Context, params *acp.SessionNotification) error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, c := range b.clients {
		c.SessionUpdate(ctx, params) // best-effort broadcast
	}
	return nil
}

func (b *BroadcastClient) RequestPermission(ctx context.Context, params *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.RequestPermissionResponse, error) {
		return c.RequestPermission(ctx, params)
	})
}

func (b *BroadcastClient) ReadTextFile(ctx context.Context, params *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.ReadTextFileResponse, error) {
		return c.ReadTextFile(ctx, params)
	})
}

func (b *BroadcastClient) WriteTextFile(ctx context.Context, params *acp.WriteTextFileRequest) (*acp.WriteTextFileResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.WriteTextFileResponse, error) {
		return c.WriteTextFile(ctx, params)
	})
}

func (b *BroadcastClient) CreateTerminal(ctx context.Context, params *acp.CreateTerminalRequest) (*acp.CreateTerminalResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.CreateTerminalResponse, error) {
		return c.CreateTerminal(ctx, params)
	})
}

func (b *BroadcastClient) TerminalOutput(ctx context.Context, params *acp.TerminalOutputRequest) (*acp.TerminalOutputResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.TerminalOutputResponse, error) {
		return c.TerminalOutput(ctx, params)
	})
}

func (b *BroadcastClient) ReleaseTerminal(ctx context.Context, params *acp.ReleaseTerminalRequest) (*acp.ReleaseTerminalResponse, error) {
	return b.first(ctx, func(c acacp.Client) (*acp.ReleaseTerminalResponse, error) {
		return c.ReleaseTerminal(ctx, params)
	})
}

func (b *BroadcastClient) WaitForTerminalExit(ctx context.Context, params *acp.WaitForTerminalExitRequest) (*acp.WaitForTerminalExitResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.WaitForTerminalExitResponse, error) {
		return c.WaitForTerminalExit(ctx, params)
	})
}

func (b *BroadcastClient) KillTerminalCommand(ctx context.Context, params *acp.KillTerminalRequest) (*acp.KillTerminalResponse, error) {
	return b.first(ctx, func(c acp.Client) (*acp.KillTerminalResponse, error) {
		return c.KillTerminalCommand(ctx, params)
	})
}

// first sends the request to all clients concurrently and returns the first successful response.
func first[T any](ctx context.Context, clients []acp.Client, fn func(acp.Client) (T, error)) (T, error) {
	var zero T
	if len(clients) == 0 {
		return zero, acp.ErrInternalError(nil, "no clients connected")
	}

	type result struct {
		val T
		err error
	}
	ch := make(chan result, len(clients))

	for _, c := range clients {
		go func(client acp.Client) {
			val, err := fn(client)
			ch <- result{val, err}
		}(c)
	}

	var lastErr error
	for range clients {
		select {
		case r := <-ch:
			if r.err == nil {
				return r.val, nil
			}
			lastErr = r.err
		case <-ctx.Done():
			return zero, ctx.Err()
		}
	}
	return zero, lastErr
}

func (b *BroadcastClient) first[T any](ctx context.Context, fn func(acp.Client) (T, error)) (T, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return first(ctx, b.clients, fn)
}
