package tui

import (
	"strings"
	"testing"

	acp "github.com/ironpark/go-acp"
)

func TestModelAccumulatesState(t *testing.T) {
	m := NewWithInitialPrompt("test prompt", nil)

	if !m.streaming {
		t.Fatal("should be streaming after NewWithInitialPrompt")
	}

	// Session title.
	m = sendUpdate(t, m, acp.NewSessionUpdateSessionInfoUpdate("Test Session", "2026-04-07T00:00:00Z"))
	if m.sessionTitle != "Test Session" {
		t.Errorf("sessionTitle = %q, want %q", m.sessionTitle, "Test Session")
	}

	// Thought chunk (accumulated but not printed when showThoughts=false).
	m = sendUpdate(t, m, acp.NewSessionUpdateAgentThoughtChunk(acp.NewContentBlockText("thinking..."), ""))
	// No panic is the test here; thoughts are printed via tea.Println.

	// Tool call — should produce print command.
	readKind := acp.ToolKindRead
	status := acp.ToolCallStatusInProgress
	m = sendUpdate(t, m, acp.NewSessionUpdateToolCall(acp.ToolCall{
		ToolCallID: "tc_1",
		Title:      "Read go.mod",
		Kind:       &readKind,
		Status:     &status,
		Locations:  []acp.ToolCallLocation{{Path: "go.mod"}},
	}))
	if len(m.toolCalls) != 1 {
		t.Errorf("toolCalls has %d entries, want 1", len(m.toolCalls))
	}

	// Tool call update with content.
	doneStatus := acp.ToolCallStatusCompleted
	m = sendUpdate(t, m, acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
		ToolCallID: "tc_1",
		Status:     &doneStatus,
		Content: []acp.ToolCallContent{
			acp.NewToolCallContentContent(acp.NewContentBlockText("module example.com/test\n\ngo 1.21")),
		},
	}))
	tc := m.toolCalls["tc_1"]
	if tc.status != acp.ToolCallStatusCompleted {
		t.Error("tool call should be completed")
	}
	if len(tc.content) != 1 {
		t.Errorf("tool call content has %d entries, want 1", len(tc.content))
	}

	// Tool call with diff.
	editKind := acp.ToolKindEdit
	m = sendUpdate(t, m, acp.NewSessionUpdateToolCall(acp.ToolCall{
		ToolCallID: "tc_2",
		Title:      "Edit main.go",
		Kind:       &editKind,
		Status:     &status,
		Content: []acp.ToolCallContent{
			acp.NewToolCallContentDiff("main.go", "new line\n", "old line\n"),
		},
	}))
	if len(m.toolCalls["tc_2"].diffs) != 1 {
		t.Error("tool call should have 1 diff")
	}

	// Agent message chunks accumulate in both buffers.
	m = sendUpdate(t, m, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("Hello "), "msg1"))
	m = sendUpdate(t, m, acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("**world**!"), "msg1"))
	if m.agentText.String() != "Hello **world**!" {
		t.Errorf("agentText = %q, want %q", m.agentText.String(), "Hello **world**!")
	}

	// Stream buffer shows raw text in View during streaming.
	view := m.View()
	if !strings.Contains(view, "Hello **world**!") {
		t.Errorf("View should contain raw streamed text, got: %q", view)
	}

	// Usage update.
	m = sendUpdate(t, m, acp.NewSessionUpdateUsageUpdate(acp.UsageUpdate{
		Size: 200_000,
		Used: 15_000,
		Cost: &acp.Cost{Amount: 0.05, Currency: "USD"},
	}))
	if m.usage == nil || m.usage.used != 15_000 {
		t.Error("usage should be set")
	}

	// Done.
	updated, _ := m.Update(PromptDoneMsg{StopReason: acp.StopReasonEndTurn})
	m = updated.(Model)
	if m.streaming {
		t.Error("should not be streaming after done")
	}
}

func sendUpdate(t *testing.T, m Model, update acp.SessionUpdate) Model {
	t.Helper()
	updated, _ := m.Update(SessionUpdateMsg{
		Update: acp.SessionNotification{
			SessionID: "test_session",
			Update:    update,
		},
	})
	return updated.(Model)
}
