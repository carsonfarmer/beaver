package eventlog

import (
	"context"
	"testing"

	"github.com/google/uuid"
	acp "github.com/ironpark/go-acp"
)

func TestMemLog_AppendAndRead(t *testing.T) {
	store := NewMemStore()
	log, err := store.Create("s1", acp.SessionInfo{SessionID: "s1", Cwd: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("hello"), "m1"))
	log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText(" world"), "m1"))

	events, err := log.Read(ctx, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	// Primer + 2 chunks.
	if len(events) != 3 {
		t.Fatalf("Read returned %d events, want 3", len(events))
	}
	if events[0].Info == nil {
		t.Fatal("event 0 should be the primer")
	}
	for i, ev := range events {
		if ev.ID == uuid.Nil {
			t.Fatalf("event %d has nil ID", i)
		}
	}
}

func TestMemLog_ReadFromTip(t *testing.T) {
	// Read(tip) returns the chain from root to tip, inclusive.
	store := NewMemStore()
	log, _ := store.Create("s1", acp.SessionInfo{SessionID: "s1"})
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("x"), ""))
	}

	all, _ := log.Read(ctx, uuid.Nil)
	// Primer + 5 chunks.
	if len(all) != 6 {
		t.Fatalf("Read(nil) = %d, want 6", len(all))
	}
	// Rewinding to the 3rd event (all[2]) gives events 0..2.
	events, _ := log.Read(ctx, all[2].ID)
	if len(events) != 3 {
		t.Fatalf("Read(from 3rd) = %d events, want 3", len(events))
	}
	if events[2].ID != all[2].ID {
		t.Fatalf("last event ID = %s, want %s", events[2].ID, all[2].ID)
	}
	for i := 1; i < len(events); i++ {
		if events[i].ParentID != events[i-1].ID {
			t.Fatalf("event %d parent = %s, want %s", i, events[i].ParentID, events[i-1].ID)
		}
	}
	if events[0].ParentID != uuid.Nil {
		t.Fatalf("root parent = %s, want nil", events[0].ParentID)
	}

	future := uuid.Must(uuid.NewV7())
	events, _ = log.Read(ctx, future)
	if len(events) != 0 {
		t.Fatalf("Read(future) returned %d events, want 0", len(events))
	}
}

func TestMemStore_CreateOpenDeleteList(t *testing.T) {
	store := NewMemStore()

	_, err := store.Create("s1", acp.SessionInfo{SessionID: "s1"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = store.Create("s1", acp.SessionInfo{SessionID: "s1"})
	if err == nil {
		t.Fatal("expected error on duplicate Create")
	}

	if _, err := store.Open("s1"); err != nil {
		t.Fatal(err)
	}

	_, err = store.Open("nope")
	if err == nil {
		t.Fatal("expected error on Open non-existent")
	}

	ids, _ := store.List()
	if len(ids) != 1 || ids[0] != "s1" {
		t.Fatalf("List() = %v, want [s1]", ids)
	}

	store.Delete("s1")
	ids, _ = store.List()
	if len(ids) != 0 {
		t.Fatalf("List() after Delete = %v, want []", ids)
	}
}
