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

