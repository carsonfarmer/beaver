package session

import (
	"os"
	"testing"

	acp "github.com/ironpark/go-acp"
)

func TestFileStore_SetAndGet(t *testing.T) {
	store := NewFileStore(t.TempDir())
	id := acp.SessionID("test-session")
	store.Set(id, &Session{Model: "openai/gpt-4.1-mini"})

	got, ok := store.Get(id)
	if !ok {
		t.Fatal("expected found")
	}
	if got.Model != "openai/gpt-4.1-mini" {
		t.Fatalf("expected model %q, got %q", "openai/gpt-4.1-mini", got.Model)
	}
}

func TestFileStore_GetNotFound(t *testing.T) {
	store := NewFileStore(t.TempDir())
	_, ok := store.Get("nonexistent")
	if ok {
		t.Fatal("expected not found")
	}
}

func TestFileStore_Delete(t *testing.T) {
	store := NewFileStore(t.TempDir())
	id := acp.SessionID("del-session")
	store.Set(id, &Session{Model: "test"})
	store.Delete(id)
	if _, ok := store.Get(id); ok {
		t.Fatal("expected deleted")
	}
	if _, err := os.Stat(store.path(id)); !os.IsNotExist(err) {
		t.Fatal("expected file removed")
	}
}

func TestFileStore_List(t *testing.T) {
	store := NewFileStore(t.TempDir())
	store.Set("sess-a", &Session{})
	store.Set("sess-b", &Session{})
	if len(store.List()) != 2 {
		t.Fatalf("expected 2, got %d", len(store.List()))
	}
}

func TestFileStore_ColdLoad(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)
	store.Set("cold", &Session{Model: "anthropic/claude-sonnet-4-5-20250514"})

	store2 := NewFileStore(dir)
	got, ok := store2.Get("cold")
	if !ok {
		t.Fatal("expected found from disk")
	}
	if got.Model != "anthropic/claude-sonnet-4-5-20250514" {
		t.Fatalf("unexpected model %q", got.Model)
	}
}
