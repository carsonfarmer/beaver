package agent

import (
	"context"
	"testing"

	"github.com/carsonfarmer/beaver/pkg/storage"
	acp "github.com/ironpark/go-acp"
)

func TestNewSession(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	resp, err := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: "/tmp/project"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SessionID == "" {
		t.Fatal("expected session ID")
	}
	sess, ok := a.cachedSession(resp.SessionID)
	if !ok {
		t.Fatal("expected session in cache")
	}
	if sess.Cwd != "/tmp/project" {
		t.Fatalf("expected cwd %q, got %q", "/tmp/project", sess.Cwd)
	}
	if sess.Model != "test/model" {
		t.Fatalf("expected default model %q, got %q", "test/model", sess.Model)
	}
	if a.archive.Tip(resp.SessionID).IsZero() {
		t.Fatal("expected session log to exist")
	}
}

func TestLoadSession(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := createTestSession(t, a, "/tmp")

	a.deleteSession(id)

	_, err := a.LoadSession(context.Background(), &acp.LoadSessionRequest{SessionID: id})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.cachedSession(id); !ok {
		t.Fatal("expected session in cache after load")
	}
}

func TestLoadSession_NotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.LoadSession(context.Background(), &acp.LoadSessionRequest{SessionID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListSessions(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	createTestSession(t, a, "/a")
	createTestSession(t, a, "/b")

	resp, err := a.ListSessions(context.Background(), &acp.ListSessionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(resp.Sessions))
	}
}

func TestListSessions_Empty(t *testing.T) {
	a := newTestAgent(t)
	resp, err := a.ListSessions(context.Background(), &acp.ListSessionsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Sessions) != 0 {
		t.Fatalf("expected 0, got %d", len(resp.Sessions))
	}
}

func TestForkSession(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := createTestSession(t, a, "/old")
	sess, _ := a.cachedSession(id)
	sess.Model = "openai/gpt-4.1"

	resp, err := a.ForkSession(context.Background(), &acp.ForkSessionRequest{SessionID: id, Cwd: "/new"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SessionID == "" || resp.SessionID == id {
		t.Fatal("expected new session ID")
	}
	forked, ok := a.cachedSession(resp.SessionID)
	if !ok {
		t.Fatal("expected forked session in cache")
	}
	if forked.Cwd != "/new" {
		t.Fatalf("expected cwd %q, got %q", "/new", forked.Cwd)
	}
}

func TestForkSession_NotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.ForkSession(context.Background(), &acp.ForkSessionRequest{SessionID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestForkSession_AtEventID(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := createTestSession(t, a, "/tmp")
	sess, _ := a.cachedSession(id)
	sess.Model = "mock/model"

	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id, Prompt: []acp.ContentBlock{acp.NewContentBlockText("first")},
	})
	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id, Prompt: []acp.ContentBlock{acp.NewContentBlockText("second")},
	})

	events, _ := storage.Lineage(a.archive, a.archive.Tip(id))

	var firstUserEvt storage.EventID
	for _, ev := range events {
		if ev.Update == nil {
			continue
		}
		if _, ok := ev.Update.AsUserMessageChunk(); ok {
			firstUserEvt = ev.ID
			break
		}
	}
	if firstUserEvt.IsZero() {
		t.Fatal("no user message event found")
	}

	resp, err := a.ForkSession(context.Background(), &acp.ForkSessionRequest{
		SessionID: id,
		Cwd:       "/new",
		Meta:      map[string]any{"fork_at_event_id": firstUserEvt.String()},
	})
	if err != nil {
		t.Fatal(err)
	}

	forkEvents, _ := storage.Lineage(a.archive, a.archive.Tip(resp.SessionID))
	var srcPos int
	for i, ev := range events {
		if ev.ID == firstUserEvt {
			srcPos = i
			break
		}
	}
	// Zero-copy fork: source events [0..srcPos] + fork header.
	wantLen := srcPos + 2
	if len(forkEvents) != wantLen {
		t.Fatalf("fork has %d events, want %d", len(forkEvents), wantLen)
	}
}

func TestForkSession_InvalidEventID(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := createTestSession(t, a, "/tmp")
	_, err := a.ForkSession(context.Background(), &acp.ForkSessionRequest{
		SessionID: id,
		Meta:      map[string]any{"fork_at_event_id": "not-a-valid-id"},
	})
	if err == nil {
		t.Fatal("expected error on invalid event id")
	}
}

func TestResumeSession(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := createTestSession(t, a, "/tmp")

	_, err := a.ResumeSession(context.Background(), &acp.ResumeSessionRequest{SessionID: id})
	if err != nil {
		t.Fatal(err)
	}
}

func TestResumeSession_NotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.ResumeSession(context.Background(), &acp.ResumeSessionRequest{SessionID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}
