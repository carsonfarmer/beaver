package eventlog

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	acp "github.com/ironpark/go-acp"
)

// MemLog is an in-memory EventLog backed by a slice.
type MemLog struct {
	mu     sync.RWMutex
	events []Event
	lastID uuid.UUID
}

func (l *MemLog) Append(_ context.Context, update acp.SessionUpdate) error {
	id, err := uuid.NewV7()
	if err != nil {
		return err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	parent := l.lastID
	if p, ok := parentFromMeta(update); ok {
		parent = p
	}
	l.events = append(l.events, Event{ID: id, ParentID: parent, Update: &update})
	l.lastID = id
	return nil
}

func (l *MemLog) Read(_ context.Context, tip uuid.UUID) ([]Event, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	if tip == uuid.Nil {
		tip = l.lastID
	}
	if tip == uuid.Nil {
		return nil, nil
	}
	byID := make(map[uuid.UUID]Event, len(l.events))
	for _, ev := range l.events {
		byID[ev.ID] = ev
	}
	return walkChain(byID, tip), nil
}

// MemStore is an in-memory Store backed by a map.
type MemStore struct {
	mu   sync.RWMutex
	logs map[acp.SessionID]*MemLog
}

// NewMemStore creates a new empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{logs: make(map[acp.SessionID]*MemLog)}
}

func (s *MemStore) Create(id acp.SessionID, info acp.SessionInfo) (EventLog, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.logs[id]; ok {
		return nil, fmt.Errorf("log %q already exists", id)
	}
	rootID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}
	l := &MemLog{
		events: []Event{{ID: rootID, Info: &info}},
		lastID: rootID,
	}
	s.logs[id] = l
	return l, nil
}

func (s *MemStore) Open(id acp.SessionID) (EventLog, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	l, ok := s.logs[id]
	if !ok {
		return nil, fmt.Errorf("log %q not found", id)
	}
	return l, nil
}

func (s *MemStore) Delete(id acp.SessionID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.logs, id)
	return nil
}

func (s *MemStore) List() ([]acp.SessionID, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := make([]acp.SessionID, 0, len(s.logs))
	for id := range s.logs {
		ids = append(ids, id)
	}
	return ids, nil
}
