package storage

import (
	"testing"

	acp "github.com/ironpark/go-acp"
)

func TestFileArchiveDurability(t *testing.T) {
	dir := t.TempDir()
	a, err := NewFileArchive(dir)
	if err != nil {
		t.Fatal(err)
	}
	a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1", Cwd: "/tmp"})
	a.Append("s1", EventID{}, textUpdate("one"))
	a.Append("s1", EventID{}, textUpdate("two"))
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen: state must be rebuilt from disk.
	b, err := NewFileArchive(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	events, err := b.Events("s1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 3 {
		t.Fatalf("reopened events = %d, want 3", len(events))
	}
	if events[0].Info == nil || events[0].Info.Cwd != "/tmp" {
		t.Fatalf("header lost: %+v", events[0])
	}
	// Appending after reopen continues the sequence.
	e3, err := b.Append("s1", EventID{}, textUpdate("three"))
	if err != nil {
		t.Fatal(err)
	}
	if e3.N != 3 {
		t.Fatalf("N after reopen = %d, want 3", e3.N)
	}
}

func TestFileArchiveListDiscoversOnDisk(t *testing.T) {
	dir := t.TempDir()
	a, _ := NewFileArchive(dir)
	a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1", Cwd: "/a"})
	a.Create("s2", EventID{}, acp.SessionInfo{SessionID: "s2", Cwd: "/b"})
	a.Close()

	b, _ := NewFileArchive(dir)
	defer b.Close()
	list, err := b.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("List after reopen = %d sessions, want 2", len(list))
	}
}
