package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	acp "github.com/ironpark/go-acp"
)

// FileStore is a file-backed acp.SessionStore. Each session is stored as
// a single JSON file at <dir>/<id>.json. An in-memory cache avoids
// repeated disk reads; files are written on every Set.
type FileStore struct {
	dir   string
	mu    sync.RWMutex
	cache map[acp.SessionID]*Session
}

// NewFileStore creates a FileStore rooted at dir.
func NewFileStore(dir string) *FileStore {
	os.MkdirAll(dir, 0o755)
	return &FileStore{dir: dir, cache: make(map[acp.SessionID]*Session)}
}

func (s *FileStore) path(id acp.SessionID) string {
	return filepath.Join(s.dir, string(id)+".json")
}

func (s *FileStore) Get(id acp.SessionID) (*Session, bool) {
	s.mu.RLock()
	sess, ok := s.cache[id]
	s.mu.RUnlock()
	if ok {
		return sess, true
	}
	data, err := os.ReadFile(s.path(id))
	if err != nil {
		return nil, false
	}
	sess = &Session{}
	if json.Unmarshal(data, sess) != nil {
		return nil, false
	}
	s.mu.Lock()
	s.cache[id] = sess
	s.mu.Unlock()
	return sess, true
}

func (s *FileStore) Set(id acp.SessionID, sess *Session) {
	sess.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	s.cache[id] = sess
	s.mu.Unlock()
	data, err := json.Marshal(sess)
	if err != nil {
		return
	}
	os.WriteFile(s.path(id), data, 0o644)
}

func (s *FileStore) Delete(id acp.SessionID) {
	s.mu.Lock()
	delete(s.cache, id)
	s.mu.Unlock()
	os.Remove(s.path(id))
}

func (s *FileStore) List() []acp.SessionID {
	entries, _ := os.ReadDir(s.dir)
	ids := make([]acp.SessionID, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			ids = append(ids, acp.SessionID(strings.TrimSuffix(e.Name(), ".json")))
		}
	}
	return ids
}
