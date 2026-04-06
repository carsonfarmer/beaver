package llm

import (
	"testing"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

func TestContentBlockToPart_Text(t *testing.T) {
	block := acp.NewContentBlockText("hello")
	part := ContentBlockToPart(block)
	tp, ok := part.(fantasy.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", part)
	}
	if tp.Text != "hello" {
		t.Fatalf("expected %q, got %q", "hello", tp.Text)
	}
}

func TestContentBlockToPart_Image(t *testing.T) {
	block := acp.NewContentBlockImage("base64data", "image/png", "")
	part := ContentBlockToPart(block)
	fp, ok := part.(fantasy.FilePart)
	if !ok {
		t.Fatalf("expected FilePart, got %T", part)
	}
	if string(fp.Data) != "base64data" {
		t.Fatalf("expected %q, got %q", "base64data", string(fp.Data))
	}
	if fp.MediaType != "image/png" {
		t.Fatalf("expected %q, got %q", "image/png", fp.MediaType)
	}
}

func TestContentBlocksToMessage(t *testing.T) {
	blocks := []acp.ContentBlock{
		acp.NewContentBlockText("hello"),
		acp.NewContentBlockText("world"),
	}
	msg, ok := ContentBlocksToMessage(blocks)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if msg.Role != fantasy.MessageRoleUser {
		t.Fatalf("expected user role, got %v", msg.Role)
	}
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(msg.Content))
	}
}

func TestContentBlocksToMessage_Empty(t *testing.T) {
	_, ok := ContentBlocksToMessage(nil)
	if ok {
		t.Fatal("expected ok=false for empty blocks")
	}
}

func TestUpdateToMessage_AgentMessageChunk(t *testing.T) {
	update := acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("hello"), "")
	msg, ok := UpdateToMessage(&update)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if msg.Role != fantasy.MessageRoleAssistant {
		t.Fatalf("expected assistant role, got %v", msg.Role)
	}
	tp, ok := msg.Content[0].(fantasy.TextPart)
	if !ok {
		t.Fatalf("expected TextPart, got %T", msg.Content[0])
	}
	if tp.Text != "hello" {
		t.Fatalf("expected %q, got %q", "hello", tp.Text)
	}
}

func TestUpdateToMessage_AgentThoughtChunk(t *testing.T) {
	update := acp.NewSessionUpdateAgentThoughtChunk(acp.NewContentBlockText("thinking..."), "")
	msg, ok := UpdateToMessage(&update)
	if !ok {
		t.Fatal("expected ok=true")
	}
	rp, ok := msg.Content[0].(fantasy.ReasoningPart)
	if !ok {
		t.Fatalf("expected ReasoningPart, got %T", msg.Content[0])
	}
	if rp.Text != "thinking..." {
		t.Fatalf("expected %q, got %q", "thinking...", rp.Text)
	}
}

func TestUpdateToMessage_ToolCall(t *testing.T) {
	kind := acp.ToolKindRead
	status := acp.ToolCallStatusInProgress
	update := acp.NewSessionUpdateToolCall(acp.ToolCall{
		ToolCallID: "tc1", Title: "read_file", Kind: &kind, Status: &status,
	})
	msg, ok := UpdateToMessage(&update)
	if !ok {
		t.Fatal("expected ok=true")
	}
	tcp, ok := msg.Content[0].(fantasy.ToolCallPart)
	if !ok {
		t.Fatalf("expected ToolCallPart, got %T", msg.Content[0])
	}
	if tcp.ToolCallID != "tc1" || tcp.ToolName != "read_file" {
		t.Fatalf("unexpected tool call: %+v", tcp)
	}
}

func TestUpdateToMessage_ToolCallUpdate(t *testing.T) {
	status := acp.ToolCallStatusCompleted
	update := acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
		ToolCallID: "tc1",
		Status:     &status,
		Content:    []acp.ToolCallContent{acp.NewToolCallContentContent(acp.NewContentBlockText("file contents"))},
	})
	msg, ok := UpdateToMessage(&update)
	if !ok {
		t.Fatal("expected ok=true")
	}
	trp, ok := msg.Content[0].(fantasy.ToolResultPart)
	if !ok {
		t.Fatalf("expected ToolResultPart, got %T", msg.Content[0])
	}
	if trp.ToolCallID != "tc1" {
		t.Fatalf("expected tool call ID %q, got %q", "tc1", trp.ToolCallID)
	}
	if txt, ok := trp.Output.(fantasy.ToolResultOutputContentText); !ok || txt.Text != "file contents" {
		t.Fatalf("unexpected output: %+v", trp.Output)
	}
}

func TestUpdateToMessage_Unhandled(t *testing.T) {
	update := acp.NewSessionUpdatePlan(nil)
	_, ok := UpdateToMessage(&update)
	if ok {
		t.Fatal("expected ok=false for plan update")
	}
}

func TestUsageToUsage(t *testing.T) {
	u := UsageToUsage(fantasy.Usage{
		InputTokens:         100,
		OutputTokens:        50,
		TotalTokens:         150,
		ReasoningTokens:     20,
		CacheReadTokens:     10,
		CacheCreationTokens: 5,
	})
	if u.InputTokens != 100 {
		t.Fatalf("expected 100, got %d", u.InputTokens)
	}
	if u.OutputTokens != 50 {
		t.Fatalf("expected 50, got %d", u.OutputTokens)
	}
	if u.TotalTokens != 150 {
		t.Fatalf("expected 150, got %d", u.TotalTokens)
	}
	if u.ThoughtTokens == nil || *u.ThoughtTokens != 20 {
		t.Fatalf("expected 20, got %v", u.ThoughtTokens)
	}
	if u.CachedReadTokens == nil || *u.CachedReadTokens != 10 {
		t.Fatalf("expected 10, got %v", u.CachedReadTokens)
	}
	if u.CachedWriteTokens == nil || *u.CachedWriteTokens != 5 {
		t.Fatalf("expected 5, got %v", u.CachedWriteTokens)
	}
}

func TestUsageToUsage_ZerosAreNil(t *testing.T) {
	u := UsageToUsage(fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15})
	if u.ThoughtTokens != nil {
		t.Fatal("expected nil for zero reasoning tokens")
	}
	if u.CachedReadTokens != nil {
		t.Fatal("expected nil for zero cache read tokens")
	}
	if u.CachedWriteTokens != nil {
		t.Fatal("expected nil for zero cache write tokens")
	}
}
