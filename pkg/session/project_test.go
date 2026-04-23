package session

import (
	"encoding/json"
	"testing"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/storage"
	acp "github.com/ironpark/go-acp"
)

var testSeq int64

func event(update acp.SessionUpdate) storage.Event {
	testSeq++
	return storage.Event{ID: storage.EventID{Session: "test", N: testSeq}, Update: &update}
}

func primer(id acp.SessionID, cwd string, extra map[string]any) storage.Event {
	testSeq++
	return storage.Event{
		ID:   storage.EventID{Session: id, N: testSeq},
		Info: &acp.SessionInfo{SessionID: id, Cwd: cwd, Meta: extra},
	}
}

func TestProject_PrimerSeedsCwd(t *testing.T) {
	events := []storage.Event{primer("s1", "/home/user/project", nil)}
	state := Project(events)
	if state.Cwd != "/home/user/project" {
		t.Fatalf("Cwd = %q, want %q", state.Cwd, "/home/user/project")
	}
}

func TestProject_ConfigOptionUpdate(t *testing.T) {
	events := []storage.Event{
		primer("s1", "/tmp", nil),
		event(acp.NewSessionUpdateConfigOptionUpdate([]acp.SessionConfigOption{
			acp.NewSessionConfigOptionSelect("model", "Model", "anthropic/claude-4", nil),
			acp.NewSessionConfigOptionSelect("thought_level", "Thought", "medium", nil),
		})),
		event(acp.NewSessionUpdateConfigOptionUpdate([]acp.SessionConfigOption{
			acp.NewSessionConfigOptionSelect("model", "Model", "openai/gpt-5", nil),
		})),
	}

	state := Project(events)
	if state.Model != "openai/gpt-5" {
		t.Fatalf("Model = %q, want %q", state.Model, "openai/gpt-5")
	}
	if state.ThoughtLevel != string(llm.ThoughtMedium) {
		t.Fatalf("ThoughtLevel = %q, want %q", state.ThoughtLevel, llm.ThoughtMedium)
	}
}

func TestProject_TextMessages(t *testing.T) {
	events := []storage.Event{
		event(acp.NewSessionUpdateUserMessageChunk(acp.NewContentBlockText("hello world"), "u1")),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("hi there"), "a1")),
	}

	state := Project(events)
	if len(state.History) != 2 {
		t.Fatalf("History len = %d, want 2", len(state.History))
	}

	if state.History[0].Role != fantasy.MessageRoleUser {
		t.Fatalf("msg 0 role = %v", state.History[0].Role)
	}
	text, ok := state.History[0].Content[0].(fantasy.TextPart)
	if !ok || text.Text != "hello world" {
		t.Fatalf("msg 0 text = %q", text.Text)
	}

	if state.History[1].Role != fantasy.MessageRoleAssistant {
		t.Fatalf("msg 1 role = %v", state.History[1].Role)
	}
	text, ok = state.History[1].Content[0].(fantasy.TextPart)
	if !ok || text.Text != "hi there" {
		t.Fatalf("msg 1 text = %q", text.Text)
	}
}

func TestProject_ThoughtChunks(t *testing.T) {
	events := []storage.Event{
		event(acp.NewSessionUpdateAgentThoughtChunk(acp.NewContentBlockText("thinking..."), "a1")),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("answer"), "a1")),
	}

	state := Project(events)
	if len(state.History) != 1 {
		t.Fatalf("History len = %d, want 1", len(state.History))
	}
	msg := state.History[0]
	if msg.Role != fantasy.MessageRoleAssistant {
		t.Fatal("wrong role")
	}
	if len(msg.Content) != 2 {
		t.Fatalf("parts = %d, want 2", len(msg.Content))
	}
	if r, ok := msg.Content[0].(fantasy.ReasoningPart); !ok || r.Text != "thinking..." {
		t.Fatalf("reasoning = %v", msg.Content[0])
	}
	if tp, ok := msg.Content[1].(fantasy.TextPart); !ok || tp.Text != "answer" {
		t.Fatalf("text = %v", msg.Content[1])
	}
}

func TestProject_ToolCallAndResult(t *testing.T) {
	kind := acp.ToolKindRead
	inProgress := acp.ToolCallStatusInProgress
	completedStatus := acp.ToolCallStatusCompleted

	events := []storage.Event{
		event(acp.NewSessionUpdateUserMessageChunk(acp.NewContentBlockText("read foo.go"), "u1")),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("Let me read that."), "a1")),
		event(acp.NewSessionUpdateToolCall(acp.ToolCall{
			ToolCallID: "tc1",
			Title:      "read_file",
			Kind:       &kind,
			Status:     &inProgress,
			RawInput:   json.RawMessage(`{"path":"foo.go"}`),
		})),
		event(acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
			ToolCallID: "tc1",
			Status:     &completedStatus,
			RawOutput:  json.RawMessage(`"package main"`),
		})),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("Here's the content."), "a2")),
	}

	state := Project(events)
	if len(state.History) != 4 {
		t.Fatalf("History len = %d, want 4", len(state.History))
	}

	if state.History[0].Role != fantasy.MessageRoleUser {
		t.Fatal("msg 0: wrong role")
	}

	msg1 := state.History[1]
	if msg1.Role != fantasy.MessageRoleAssistant {
		t.Fatal("msg 1: wrong role")
	}
	if len(msg1.Content) != 2 {
		t.Fatalf("msg 1: %d parts, want 2", len(msg1.Content))
	}
	tp, ok := msg1.Content[0].(fantasy.TextPart)
	if !ok || tp.Text != "Let me read that." {
		t.Fatalf("msg 1 text = %v", msg1.Content[0])
	}
	tc, ok := msg1.Content[1].(fantasy.ToolCallPart)
	if !ok {
		t.Fatalf("msg 1 tool call = %v", msg1.Content[1])
	}
	if tc.ToolCallID != "tc1" || tc.ToolName != "read_file" || tc.Input != `{"path":"foo.go"}` {
		t.Fatalf("tool call = %+v", tc)
	}

	msg2 := state.History[2]
	if msg2.Role != fantasy.MessageRoleTool {
		t.Fatal("msg 2: wrong role")
	}
	tr, ok := msg2.Content[0].(fantasy.ToolResultPart)
	if !ok {
		t.Fatalf("msg 2 = %v", msg2.Content[0])
	}
	if tr.ToolCallID != "tc1" {
		t.Fatalf("tool result call id = %q", tr.ToolCallID)
	}
	if txt, ok := tr.Output.(fantasy.ToolResultOutputContentText); !ok || txt.Text != "package main" {
		t.Fatalf("tool result output = %v", tr.Output)
	}

	if state.History[3].Role != fantasy.MessageRoleAssistant {
		t.Fatal("msg 3: wrong role")
	}
}

func TestProject_UsageUpdate(t *testing.T) {
	events := []storage.Event{
		event(acp.NewSessionUpdateUsageUpdate(acp.UsageUpdate{Used: 500, Size: 200000})),
		event(acp.NewSessionUpdateUsageUpdate(acp.UsageUpdate{Used: 1200, Size: 200000})),
	}

	state := Project(events)
	if state.UsageUsed != 1200 || state.UsageSize != 200000 {
		t.Fatalf("Usage = (%d, %d)", state.UsageUsed, state.UsageSize)
	}
}

func TestProject_SessionInfo(t *testing.T) {
	events := []storage.Event{
		event(acp.NewSessionUpdateSessionInfoUpdate("My Chat", "2026-04-09T12:00:00Z")),
	}

	state := Project(events)
	if state.Title != "My Chat" {
		t.Fatalf("Title = %q", state.Title)
	}
	if state.UpdatedAt != "2026-04-09T12:00:00Z" {
		t.Fatalf("UpdatedAt = %q", state.UpdatedAt)
	}
}

func TestProject_MultiTurnConversation(t *testing.T) {
	kind := acp.ToolKindEdit
	inProgress := acp.ToolCallStatusInProgress
	completed := acp.ToolCallStatusCompleted

	events := []storage.Event{
		primer("s1", "/project", nil),
		event(acp.NewSessionUpdateUserMessageChunk(acp.NewContentBlockText("write hello.txt"), "u1")),
		event(acp.NewSessionUpdateAgentThoughtChunk(acp.NewContentBlockText("I'll create the file"), "a1")),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("Writing file..."), "a1")),
		event(acp.NewSessionUpdateToolCall(acp.ToolCall{
			ToolCallID: "tc1", Title: "write_file",
			Kind: &kind, Status: &inProgress,
			RawInput: json.RawMessage(`{"path":"hello.txt","content":"hello"}`),
		})),
		event(acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
			ToolCallID: "tc1", Status: &completed,
			RawOutput: json.RawMessage(`"file written successfully"`),
		})),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("Done!"), "a2")),
		event(acp.NewSessionUpdateUserMessageChunk(acp.NewContentBlockText("what did you write?"), "u2")),
		event(acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("I wrote 'hello' to hello.txt"), "a3")),
	}

	state := Project(events)

	if state.Cwd != "/project" {
		t.Fatalf("Cwd = %q, want /project", state.Cwd)
	}

	if len(state.History) != 6 {
		t.Fatalf("History len = %d, want 6", len(state.History))
	}

	roles := make([]fantasy.MessageRole, len(state.History))
	for i, m := range state.History {
		roles[i] = m.Role
	}
	expected := []fantasy.MessageRole{
		fantasy.MessageRoleUser,
		fantasy.MessageRoleAssistant,
		fantasy.MessageRoleTool,
		fantasy.MessageRoleAssistant,
		fantasy.MessageRoleUser,
		fantasy.MessageRoleAssistant,
	}
	for i, r := range roles {
		if r != expected[i] {
			t.Fatalf("msg %d: role = %v, want %v", i, r, expected[i])
		}
	}

	msg := state.History[1]
	if len(msg.Content) != 3 {
		t.Fatalf("assistant msg 1: %d parts, want 3", len(msg.Content))
	}
	if _, ok := msg.Content[0].(fantasy.ReasoningPart); !ok {
		t.Fatal("expected ReasoningPart first")
	}
	if _, ok := msg.Content[1].(fantasy.TextPart); !ok {
		t.Fatal("expected TextPart second")
	}
	if _, ok := msg.Content[2].(fantasy.ToolCallPart); !ok {
		t.Fatal("expected ToolCallPart third")
	}
}
