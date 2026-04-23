package storage

import (
	"fmt"
	"sync"
	"time"

	acp "github.com/ironpark/go-acp"
)

// MemArchive is a non-durable in-memory archive. Events live in the process
// and are lost on exit. Useful for ephemeral sessions and for tests that
// want to avoid touching the filesystem.
type MemArchive struct {
	mu       sync.Mutex
	sessions map[acp.SessionID]*memSession
}

type memSession struct {
	mu     sync.RWMutex
	id     acp.SessionID
	events []Event
}

// NewMemArchive returns an empty in-memory archive.
func NewMemArchive() *MemArchive {
	return &MemArchive{sessions: make(map[acp.SessionID]*memSession)}
}

func (a *MemArchive) Create(id acp.SessionID, parent EventID, info acp.SessionInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.sessions[id]; ok {
		return fmt.Errorf("session %q already exists", id)
	}
	header := Event{
		ID:        EventID{Session: id, N: 0},
		Parent:    parent,
		Timestamp: time.Now().UTC(),
		Info:      &info,
	}
	a.sessions[id] = &memSession{id: id, events: []Event{header}}
	return nil
}

func (a *MemArchive) Delete(id acp.SessionID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, id)
	return nil
}

func (a *MemArchive) List() ([]acp.SessionInfo, error) {
	a.mu.Lock()
	sessions := make([]*memSession, 0, len(a.sessions))
	for _, s := range a.sessions {
		sessions = append(sessions, s)
	}
	a.mu.Unlock()
	var out []acp.SessionInfo
	for _, s := range sessions {
		s.mu.RLock()
		if len(s.events) > 0 && s.events[0].Info != nil {
			out = append(out, *s.events[0].Info)
		}
		s.mu.RUnlock()
	}
	return out, nil
}

func (a *MemArchive) Append(id acp.SessionID, parent EventID, update acp.SessionUpdate) (EventID, error) {
	s, err := a.session(id)
	if err != nil {
		return EventID{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	eid := EventID{Session: id, N: int64(len(s.events))}
	if parent.IsZero() && len(s.events) > 0 {
		parent = s.events[len(s.events)-1].ID
	}
	s.events = append(s.events, Event{
		ID:        eid,
		Parent:    parent,
		Timestamp: time.Now().UTC(),
		Update:    &update,
	})
	return eid, nil
}

func (a *MemArchive) Tip(id acp.SessionID) EventID {
	s, err := a.session(id)
	if err != nil {
		return EventID{}
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.events) == 0 {
		return EventID{}
	}
	return s.events[len(s.events)-1].ID
}

func (a *MemArchive) Events(id acp.SessionID) ([]Event, error) {
	s, err := a.session(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out, nil
}

func (a *MemArchive) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions = nil
	return nil
}

func (a *MemArchive) session(id acp.SessionID) (*memSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	s, ok := a.sessions[id]
	if !ok {
		return nil, fmt.Errorf("session %q not found", id)
	}
	return s, nil
}
