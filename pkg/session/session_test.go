package session

import "testing"

func TestSession_Info(t *testing.T) {
	sess := &Session{Cwd: "/project", Title: "My Session", UpdatedAt: "2026-04-02T00:00:00Z"}
	info := sess.Info("sess-1")
	if info.SessionID != "sess-1" {
		t.Fatalf("unexpected ID %q", info.SessionID)
	}
	if info.Cwd != "/project" {
		t.Fatalf("unexpected cwd %q", info.Cwd)
	}
	if info.Title != "My Session" {
		t.Fatalf("unexpected title %q", info.Title)
	}
}

func TestSession_Fork(t *testing.T) {
	sess := &Session{
		Cwd:          "/old",
		Model:        "openai/gpt-4.1",
		ThoughtLevel: "high",
		SystemPrompt: "some prompt",
	}
	forked := sess.Fork("/new")
	if forked.Cwd != "/new" {
		t.Fatalf("expected cwd %q, got %q", "/new", forked.Cwd)
	}
	if forked.Model != "openai/gpt-4.1" {
		t.Fatalf("expected model copied, got %q", forked.Model)
	}
	if forked.ThoughtLevel != "high" {
		t.Fatalf("expected thought level copied")
	}
	// SystemPrompt should NOT be copied (re-discovered on fork)
	if forked.SystemPrompt != "" {
		t.Fatal("expected system prompt to not be copied")
	}
}
