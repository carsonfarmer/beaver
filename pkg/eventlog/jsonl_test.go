package eventlog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/uuid"
	acp "github.com/ironpark/go-acp"
)

// helper: create a log with a basic primer.
func newLog(t *testing.T, store *JSONLStore, id acp.SessionID) EventLog {
	t.Helper()
	log, err := store.Create(id, acp.SessionInfo{SessionID: id, Cwd: "/tmp"})
	if err != nil {
		t.Fatal(err)
	}
	return log
}

func TestJSONLLog_AppendReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONLStore(dir)
	ctx := context.Background()

	log := newLog(t, store, "s1")

	log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("hello"), "m1"))
	log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText(" world"), "m1"))

	events, err := log.Read(ctx, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	// Primer + 2 chunks.
	if len(events) != 3 {
		t.Fatalf("Read = %d events, want 3", len(events))
	}
	if events[0].Info == nil {
		t.Fatal("event 0 should be the primer")
	}
	if events[0].Info.Cwd != "/tmp" {
		t.Fatalf("primer cwd = %q", events[0].Info.Cwd)
	}

	chunk, ok := events[1].Update.AsAgentMessageChunk()
	if !ok {
		t.Fatal("expected AgentMessageChunk at events[1]")
	}
	text, ok := chunk.Content.AsText()
	if !ok || text.Text != "hello" {
		t.Fatalf("text = %q, want %q", text.Text, "hello")
	}
}

func TestJSONLLog_ReadFromTip(t *testing.T) {
	// Read(tip) returns the chain from root to tip, inclusive.
	dir := t.TempDir()
	store := NewJSONLStore(dir)
	ctx := context.Background()

	log := newLog(t, store, "s1")
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
	// ParentID chain: each event's parent is the previous one.
	for i := 1; i < len(events); i++ {
		if events[i].ParentID != events[i-1].ID {
			t.Fatalf("event %d parent = %s, want %s", i, events[i].ParentID, events[i-1].ID)
		}
	}
	if events[0].ParentID != uuid.Nil {
		t.Fatalf("root parent = %s, want nil", events[0].ParentID)
	}
}

func TestJSONLLog_Durability(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONLStore(dir)
	ctx := context.Background()

	log := newLog(t, store, "s1")
	log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("persisted"), "m1"))

	store2 := NewJSONLStore(dir)
	log2, err := store2.Open("s1")
	if err != nil {
		t.Fatal(err)
	}
	events, _ := log2.Read(ctx, uuid.Nil)
	// Primer + 1 chunk.
	if len(events) != 2 {
		t.Fatalf("got %d events after reopen, want 2", len(events))
	}
}

func TestJSONLLog_AllUpdateVariants(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONLStore(dir)
	ctx := context.Background()
	log := newLog(t, store, "s1")

	status := acp.ToolCallStatusInProgress
	kind := acp.ToolKindRead

	updates := []acp.SessionUpdate{
		acp.NewSessionUpdateUserMessageChunk(acp.NewContentBlockText("hi"), "u1"),
		acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("hey"), "a1"),
		acp.NewSessionUpdateAgentThoughtChunk(acp.NewContentBlockText("thinking"), "t1"),
		acp.NewSessionUpdateToolCall(acp.ToolCall{
			ToolCallID: "tc1", Title: "read_file foo.go", Kind: &kind, Status: &status,
		}),
		acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
			ToolCallID: "tc1", RawOutput: json.RawMessage(`"content"`),
		}),
		acp.NewSessionUpdatePlan([]acp.PlanEntry{
			{Content: "step 1", Status: acp.PlanEntryStatusPending, Priority: acp.PlanEntryPriorityMedium},
		}),
		acp.NewSessionUpdateCurrentModeUpdate("ask"),
		acp.NewSessionUpdateSessionInfoUpdate("My Title", "2026-04-09T00:00:00Z"),
		acp.NewSessionUpdateUsageUpdate(acp.UsageUpdate{Used: 100, Size: 8000}),
	}

	for _, u := range updates {
		if err := log.Append(ctx, u); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	events, err := log.Read(ctx, uuid.Nil)
	if err != nil {
		t.Fatal(err)
	}
	// Primer + all updates.
	if len(events) != len(updates)+1 {
		t.Fatalf("got %d events, want %d", len(events), len(updates)+1)
	}

	ups := events[1:] // skip primer
	if _, ok := ups[0].Update.AsUserMessageChunk(); !ok {
		t.Error("event 0: expected UserMessageChunk")
	}
	if _, ok := ups[1].Update.AsAgentMessageChunk(); !ok {
		t.Error("event 1: expected AgentMessageChunk")
	}
	if _, ok := ups[2].Update.AsAgentThoughtChunk(); !ok {
		t.Error("event 2: expected AgentThoughtChunk")
	}
	if _, ok := ups[3].Update.AsToolCall(); !ok {
		t.Error("event 3: expected ToolCall")
	}
	if _, ok := ups[4].Update.AsToolCallUpdate(); !ok {
		t.Error("event 4: expected ToolCallUpdate")
	}
	if _, ok := ups[5].Update.AsPlan(); !ok {
		t.Error("event 5: expected Plan")
	}
	if _, ok := ups[6].Update.AsCurrentModeUpdate(); !ok {
		t.Error("event 6: expected CurrentModeUpdate")
	}
	if _, ok := ups[7].Update.AsSessionInfoUpdate(); !ok {
		t.Error("event 7: expected SessionInfoUpdate")
	}
	if _, ok := ups[8].Update.AsUsageUpdate(); !ok {
		t.Error("event 8: expected UsageUpdate")
	}
}

func TestJSONLStore_CreateOpenDeleteList(t *testing.T) {
	dir := t.TempDir()
	store := NewJSONLStore(dir)

	store.Create("s1", acp.SessionInfo{SessionID: "s1"})
	store.Create("s2", acp.SessionInfo{SessionID: "s2"})

	_, err := store.Create("s1", acp.SessionInfo{SessionID: "s1"})
	if err == nil {
		t.Fatal("expected error on duplicate Create")
	}

	ids, _ := store.List()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	if len(ids) != 2 || ids[0] != "s1" || ids[1] != "s2" {
		t.Fatalf("List() = %v", ids)
	}

	store.Delete("s1")
	ids, _ = store.List()
	if len(ids) != 1 || ids[0] != "s2" {
		t.Fatalf("List() after delete = %v", ids)
	}

	_, err = store.Open("s1")
	if err == nil {
		t.Fatal("expected error on Open after Delete")
	}

	if _, err := os.Stat(filepath.Join(dir, "s1.jsonl")); !os.IsNotExist(err) {
		t.Fatal("file not deleted")
	}
}
