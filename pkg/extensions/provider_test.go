package extensions

import (
	"context"
	"strings"
	"testing"

	acp "github.com/ironpark/go-acp"
)

func TestBasePrompt_WithCwd(t *testing.T) {
	p := BasePrompt("base prompt")
	s, err := p(context.Background(), "/project", acp.SessionID(""))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(s, "base prompt") {
		t.Fatal("expected base prompt")
	}
	if !strings.Contains(s, "/project") {
		t.Fatal("expected cwd in prompt")
	}
}

func TestBasePrompt_WithoutCwd(t *testing.T) {
	p := BasePrompt("base prompt")
	s, err := p(context.Background(), "", acp.SessionID(""))
	if err != nil {
		t.Fatal(err)
	}
	if s != "base prompt" {
		t.Fatalf("expected base prompt only, got %q", s)
	}
}

func TestAssemble_JoinsParts(t *testing.T) {
	providers := []Provider{
		func(_ context.Context, _ string, _ acp.SessionID) (string, error) {
			return "first", nil
		},
		func(_ context.Context, _ string, _ acp.SessionID) (string, error) {
			return "second", nil
		},
	}
	s, err := Assemble(context.Background(), "", acp.SessionID(""), providers)
	if err != nil {
		t.Fatal(err)
	}
	if s != "first\n\nsecond" {
		t.Fatalf("unexpected assembly: %q", s)
	}
}

func TestAssemble_SkipsEmpty(t *testing.T) {
	providers := []Provider{
		func(_ context.Context, _ string, _ acp.SessionID) (string, error) {
			return "first", nil
		},
		func(_ context.Context, _ string, _ acp.SessionID) (string, error) {
			return "", nil
		},
	}
	s, err := Assemble(context.Background(), "", acp.SessionID(""), providers)
	if err != nil {
		t.Fatal(err)
	}
	if s != "first" {
		t.Fatalf("unexpected assembly: %q", s)
	}
}

func TestAssemble_ReturnsError(t *testing.T) {
	providers := []Provider{
		func(_ context.Context, _ string, _ acp.SessionID) (string, error) {
			return "", context.Canceled
		},
	}
	_, err := Assemble(context.Background(), "", acp.SessionID(""), providers)
	if err != context.Canceled {
		t.Fatalf("expected canceled error, got %v", err)
	}
}
