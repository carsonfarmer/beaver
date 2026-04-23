package agent

import (
	"sync"

	acp "github.com/ironpark/go-acp"
)

// SessionRegistry tracks which clients are subscribed to which sessions.
type SessionRegistry struct {
	mu   sync.RWMutex
	subs map[acp.SessionID][]acp.Client
}

// NewSessionRegistry creates a new registry.
func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{subs: make(map[acp.SessionID][]acp.Client)}
}

// Subscribe adds a client to a session's subscriber list.
func (r *SessionRegistry) Subscribe(sid acp.SessionID, client acp.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.subs[sid] = append(r.subs[sid], client)
}

// Unsubscribe removes a client from a session's subscriber list.
func (r *SessionRegistry) Unsubscribe(sid acp.SessionID, client acp.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	clients := r.subs[sid]
	for i, c := range clients {
		if c == client {
			r.subs[sid] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(r.subs[sid]) == 0 {
		delete(r.subs, sid)
	}
}

// UnsubscribeAll removes a client from all sessions.
func (r *SessionRegistry) UnsubscribeAll(client acp.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for sid, clients := range r.subs {
		for i, c := range clients {
			if c == client {
				r.subs[sid] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		if len(r.subs[sid]) == 0 {
			delete(r.subs, sid)
		}
	}
}

// Subscribers returns clients subscribed to a session.
func (r *SessionRegistry) Subscribers(sid acp.SessionID) []acp.Client {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]acp.Client, len(r.subs[sid]))
	copy(out, r.subs[sid])
	return out
}
