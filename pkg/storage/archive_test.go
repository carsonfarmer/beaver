package storage

import (
	"testing"

	acp "github.com/ironpark/go-acp"
)

// archiveFactory lets tests run against every Archive implementation.
type archiveFactory struct {
	name string
	new  func(t *testing.T) Archive
}

func archiveImpls(t *testing.T) []archiveFactory {
	t.Helper()
	return []archiveFactory{
		{"mem", func(t *testing.T) Archive { return NewMemArchive() }},
		{"file", func(t *testing.T) Archive {
			a, err := NewFileArchive(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { a.Close() })
			return a
		}},
	}
}

func textUpdate(s string) acp.SessionUpdate {
	return acp.NewSessionUpdateUserMessageChunk(acp.NewContentBlockText(s), "")
}

func TestArchiveCreateAppendEvents(t *testing.T) {
	for _, f := range archiveImpls(t) {
		t.Run(f.name, func(t *testing.T) {
			a := f.new(t)
			id := acp.SessionID("s1")
			if err := a.Create(id, EventID{}, acp.SessionInfo{SessionID: id, Cwd: "/tmp"}); err != nil {
				t.Fatal(err)
			}
			e1, err := a.Append(id, EventID{}, textUpdate("hello"))
			if err != nil {
				t.Fatal(err)
			}
			if e1.N != 1 {
				t.Fatalf("first append N = %d, want 1", e1.N)
			}
			e2, _ := a.Append(id, EventID{}, textUpdate("world"))
			if e2.N != 2 {
				t.Fatalf("second append N = %d, want 2", e2.N)
			}
			if tip := a.Tip(id); tip != e2 {
				t.Fatalf("Tip = %v, want %v", tip, e2)
			}
			events, err := a.Events(id)
			if err != nil {
				t.Fatal(err)
			}
			if len(events) != 3 {
				t.Fatalf("len events = %d, want 3 (header + 2 appends)", len(events))
			}
			if events[0].Info == nil {
				t.Fatal("event 0 should be the header (Info set)")
			}
			if events[1].Parent != events[0].ID {
				t.Fatalf("event 1 parent = %v, want %v", events[1].Parent, events[0].ID)
			}
			if events[2].Parent != events[1].ID {
				t.Fatalf("event 2 parent = %v, want %v", events[2].Parent, events[1].ID)
			}
		})
	}
}

func TestArchiveCreateDuplicate(t *testing.T) {
	for _, f := range archiveImpls(t) {
		t.Run(f.name, func(t *testing.T) {
			a := f.new(t)
			if err := a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"}); err != nil {
				t.Fatal(err)
			}
			if err := a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"}); err == nil {
				t.Fatal("expected duplicate Create to error")
			}
		})
	}
}

func TestArchiveExplicitParentRewind(t *testing.T) {
	for _, f := range archiveImpls(t) {
		t.Run(f.name, func(t *testing.T) {
			a := f.new(t)
			a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"})
			e1, _ := a.Append("s1", EventID{}, textUpdate("a"))
			e2, _ := a.Append("s1", EventID{}, textUpdate("b"))
			e3, _ := a.Append("s1", e1, textUpdate("rewound"))
			if e3.N != 3 {
				t.Fatalf("N = %d, want 3", e3.N)
			}
			events, _ := a.Events("s1")
			if events[3].Parent != e1 {
				t.Fatalf("rewind parent = %v, want %v", events[3].Parent, e1)
			}
			_ = e2
		})
	}
}

func TestArchiveListReturnsHeaders(t *testing.T) {
	for _, f := range archiveImpls(t) {
		t.Run(f.name, func(t *testing.T) {
			a := f.new(t)
			a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1", Cwd: "/a"})
			a.Create("s2", EventID{}, acp.SessionInfo{SessionID: "s2", Cwd: "/b"})
			list, err := a.List()
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 2 {
				t.Fatalf("List len = %d, want 2", len(list))
			}
			cwds := map[string]bool{}
			for _, s := range list {
				cwds[s.Cwd] = true
			}
			if !cwds["/a"] || !cwds["/b"] {
				t.Fatalf("list cwds = %v", cwds)
			}
		})
	}
}

func TestArchiveDelete(t *testing.T) {
	for _, f := range archiveImpls(t) {
		t.Run(f.name, func(t *testing.T) {
			a := f.new(t)
			a.Create("s1", EventID{}, acp.SessionInfo{SessionID: "s1"})
			if err := a.Delete("s1"); err != nil {
				t.Fatal(err)
			}
			if _, err := a.Events("s1"); err == nil {
				t.Fatal("expected Events after Delete to error")
			}
			// Double-delete is a no-op.
			if err := a.Delete("s1"); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestArchiveTipZeroForUnknown(t *testing.T) {
	for _, f := range archiveImpls(t) {
		t.Run(f.name, func(t *testing.T) {
			a := f.new(t)
			if tip := a.Tip("nope"); !tip.IsZero() {
				t.Fatalf("Tip for unknown = %v, want zero", tip)
			}
		})
	}
}
