package eventlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	acp "github.com/ironpark/go-acp"
)

// JSONLLog stores events as newline-delimited JSON in a single file.
type JSONLLog struct {
	mu     sync.Mutex
	path   string
	lastID uuid.UUID
}

// init scans the log to recover lastID after a fresh open.
func (l *JSONLLog) init() error {
	f, err := os.Open(l.path)
	if err != nil {
		return err
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	for dec.More() {
		var ev Event
		if err := dec.Decode(&ev); err != nil {
			return err
		}
		l.lastID = ev.ID
	}
	return nil
}

func (l *JSONLLog) Append(_ context.Context, update acp.SessionUpdate) error {
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

	f, err := os.OpenFile(l.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(Event{ID: id, ParentID: parent, Update: &update}); err != nil {
		return err
	}
	l.lastID = id
	return nil
}

func (l *JSONLLog) Read(_ context.Context, tip uuid.UUID) ([]Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	f, err := os.Open(l.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	byID := make(map[uuid.UUID]Event)
	dec := json.NewDecoder(f)
	for dec.More() {
		var ev Event
		if err := dec.Decode(&ev); err != nil {
			return nil, err
		}
		byID[ev.ID] = ev
	}

	if tip == uuid.Nil {
		tip = l.lastID
	}
	if tip == uuid.Nil {
		return nil, nil
	}
	return walkChain(byID, tip), nil
}

// JSONLStore manages JSONL-backed event logs on disk.
// Each session gets a file at <dir>/<id>.jsonl.
type JSONLStore struct {
	dir   string
	mu    sync.RWMutex
	cache map[acp.SessionID]*JSONLLog
}

// NewJSONLStore creates a store rooted at dir.
func NewJSONLStore(dir string) *JSONLStore {
	os.MkdirAll(dir, 0o755)
	return &JSONLStore{dir: dir, cache: make(map[acp.SessionID]*JSONLLog)}
}

func (s *JSONLStore) path(id acp.SessionID) string {
	return filepath.Join(s.dir, string(id)+".jsonl")
}

func (s *JSONLStore) Create(id acp.SessionID, info acp.SessionInfo) (EventLog, error) {
	path := s.path(id)

	if _, err := os.Stat(path); err == nil {
		return nil, fmt.Errorf("log %q already exists", id)
	}

	rootID, err := uuid.NewV7()
	if err != nil {
		return nil, err
	}

	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := json.NewEncoder(f).Encode(Event{ID: rootID, Info: &info}); err != nil {
		f.Close()
		return nil, err
	}
	f.Close()

	l := &JSONLLog{path: path, lastID: rootID}
	s.mu.Lock()
	s.cache[id] = l
	s.mu.Unlock()
	return l, nil
}

func (s *JSONLStore) Open(id acp.SessionID) (EventLog, error) {
	s.mu.RLock()
	l, ok := s.cache[id]
	s.mu.RUnlock()
	if ok {
		return l, nil
	}

	path := s.path(id)
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	l = &JSONLLog{path: path}
	if err := l.init(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	s.cache[id] = l
	s.mu.Unlock()
	return l, nil
}

func (s *JSONLStore) Delete(id acp.SessionID) error {
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
	return os.Remove(s.path(id))
}

func (s *JSONLStore) List() ([]acp.SessionID, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var ids []acp.SessionID
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			ids = append(ids, acp.SessionID(strings.TrimSuffix(e.Name(), ".jsonl")))
		}
	}
	return ids, nil
}
