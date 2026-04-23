package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	acp "github.com/ironpark/go-acp"
)

// HTTPBridge multiplexes multiple HTTP clients through a single agent.
// Each client gets its own AgentSideConnection with per-client transports.
// Notifications route by session subscription. Requests broadcast to all clients.
type HTTPBridge struct {
	agent     *Agent
	registry  *SessionRegistry
	broadcast *BroadcastClient
	mu        sync.Mutex
	clients   map[string]*httpClient
}

// NewHTTPBridge creates a bridge for the given agent.
func NewHTTPBridge(a *Agent) *HTTPBridge {
	reg := NewSessionRegistry()
	bc := NewBroadcastClient(reg)
	a.SetClient(bc)
	return &HTTPBridge{
		agent:     a,
		registry:  reg,
		broadcast: bc,
		clients:   make(map[string]*httpClient),
	}
}

// Handler returns an http.Handler for the bridge endpoints.
//   GET /clients/{id}/events  — SSE stream for agent-to-client messages
//   POST /clients/{id}/message — client-to-agent JSON-RPC messages
func (b *HTTPBridge) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /clients/{id}/events", b.handleEvents)
	mux.HandleFunc("POST /clients/{id}/message", b.handleMessage)
	return mux
}

func (b *HTTPBridge) handleEvents(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	transport := newClientTransport()
	conn := acp.NewAgentSideConnection(b.agent, nil, nil,
		acp.WithTransport(transport),
		acp.WithMiddleware(SubscriptionMiddleware(b.registry)),
	)

	client := &httpClient{id: id, transport: transport, conn: conn}

	b.mu.Lock()
	b.clients[id] = client
	b.mu.Unlock()
	b.broadcast.Add(conn)

	ctx := ContextWithClient(r.Context(), conn)
	go conn.Start(ctx)

	defer func() {
		conn.Close()
		transport.Close()
		b.broadcast.Remove(conn)
		b.registry.UnsubscribeAll(conn)
		b.mu.Lock()
		delete(b.clients, id)
		b.mu.Unlock()
	}()

	for {
		select {
		case msg := <-transport.outbox:
			w.Write([]byte("event: message\ndata: "))
			w.Write(msg)
			w.Write([]byte("\n\n"))
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-transport.done:
			return
		}
	}
}

func (b *HTTPBridge) handleMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	b.mu.Lock()
	client := b.clients[id]
	b.mu.Unlock()

	if client == nil {
		http.Error(w, "client not connected", http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 50*1024*1024))
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}

	select {
	case client.transport.inbox <- json.RawMessage(body):
		w.WriteHeader(http.StatusAccepted)
	case <-client.transport.done:
		http.Error(w, "client disconnected", http.StatusServiceUnavailable)
	case <-r.Context().Done():
		http.Error(w, "request cancelled", http.StatusRequestTimeout)
	}
}

// --- per-client transport ---

type clientTransport struct {
	inbox  chan json.RawMessage
	outbox chan json.RawMessage
	done   chan struct{}
	once   sync.Once
}

func newClientTransport() *clientTransport {
	return &clientTransport{
		inbox:  make(chan json.RawMessage, 100),
		outbox: make(chan json.RawMessage, 100),
		done:   make(chan struct{}),
	}
}

func (t *clientTransport) ReadMessage(ctx context.Context) (json.RawMessage, error) {
	select {
	case msg := <-t.inbox:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-t.done:
		return nil, io.EOF
	}
}

func (t *clientTransport) WriteMessage(ctx context.Context, data json.RawMessage) error {
	select {
	case t.outbox <- data:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.done:
		return acp.ErrTransportClosed
	}
}

func (t *clientTransport) Close() error {
	t.once.Do(func() { close(t.done) })
	return nil
}

// --- client wrapper ---

type httpClient struct {
	id        string
	transport *clientTransport
	conn      *acp.AgentSideConnection
}
