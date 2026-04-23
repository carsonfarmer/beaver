package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/ironpark/go-acp"
)

// FileArchive persists each session as a JSONL file under dir. The file is
// both the durable unit and the catalog entry: to list sessions, enumerate
// files; to open a session, read its file. Open files stay open for the
// archive's lifetime; Close releases them.
type FileArchive struct {
	dir   string
	mu    sync.Mutex
	cache map[acp.SessionID]*fileSession
}

type fileSession struct {
	mu     sync.RWMutex
	id     acp.SessionID
	path   string
	events []Event
	f      *os.File
	enc    *json.Encoder
}

// NewFileArchive opens (or creates) an archive rooted at dir.
func NewFileArchive(dir string) (*FileArchive, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileArchive{dir: dir, cache: make(map[acp.SessionID]*fileSession)}, nil
}

func (a *FileArchive) path(id acp.SessionID) string {
	return filepath.Join(a.dir, string(id)+".jsonl")
}

func (a *FileArchive) Create(id acp.SessionID, parent EventID, info acp.SessionInfo) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.cache[id]; ok {
		return fmt.Errorf("session %q already exists", id)
	}
	path := a.path(id)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("session %q already exists", id)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	header := Event{
		ID:        EventID{Session: id, N: 0},
		Parent:    parent,
		Timestamp: time.Now().UTC(),
		Info:      &info,
	}
	enc := json.NewEncoder(f)
	if err := enc.Encode(header); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	a.cache[id] = &fileSession{
		id:     id,
		path:   path,
		events: []Event{header},
		f:      f,
		enc:    enc,
	}
	return nil
}

func (a *FileArchive) Delete(id acp.SessionID) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.cache[id]; ok {
		s.f.Close()
		delete(a.cache, id)
	}
	err := os.Remove(a.path(id))
	if err != nil && os.IsNotExist(err) {
		return nil
	}
	return err
}

func (a *FileArchive) List() ([]acp.SessionInfo, error) {
	entries, err := os.ReadDir(a.dir)
	if err != nil {
		return nil, err
	}
	var out []acp.SessionInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		id := acp.SessionID(strings.TrimSuffix(e.Name(), ".jsonl"))
		s, err := a.open(id)
		if err != nil {
			continue
		}
		s.mu.RLock()
		if len(s.events) > 0 && s.events[0].Info != nil {
			out = append(out, *s.events[0].Info)
		}
		s.mu.RUnlock()
	}
	return out, nil
}

func (a *FileArchive) Append(id acp.SessionID, parent EventID, update acp.SessionUpdate) (EventID, error) {
	s, err := a.open(id)
	if err != nil {
		return EventID{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	eid := EventID{Session: id, N: int64(len(s.events))}
	if parent.IsZero() && len(s.events) > 0 {
		parent = s.events[len(s.events)-1].ID
	}
	ev := Event{
		ID:        eid,
		Parent:    parent,
		Timestamp: time.Now().UTC(),
		Update:    &update,
	}
	if err := s.enc.Encode(ev); err != nil {
		return EventID{}, err
	}
	s.events = append(s.events, ev)
	return eid, nil
}

func (a *FileArchive) Tip(id acp.SessionID) EventID {
	s, err := a.open(id)
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

func (a *FileArchive) Events(id acp.SessionID) ([]Event, error) {
	s, err := a.open(id)
	if err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Event, len(s.events))
	copy(out, s.events)
	return out, nil
}

func (a *FileArchive) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, s := range a.cache {
		s.f.Close()
	}
	a.cache = nil
	return nil
}

// open returns a cached session or loads one from disk.
func (a *FileArchive) open(id acp.SessionID) (*fileSession, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.cache[id]; ok {
		return s, nil
	}
	path := a.path(id)
	r, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	var events []Event
	dec := json.NewDecoder(r)
	for dec.More() {
		var ev Event
		if err := dec.Decode(&ev); err != nil {
			r.Close()
			return nil, err
		}
		events = append(events, ev)
	}
	r.Close()
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	s := &fileSession{
		id:     id,
		path:   path,
		events: events,
		f:      f,
		enc:    json.NewEncoder(f),
	}
	a.cache[id] = s
	return s, nil
}
