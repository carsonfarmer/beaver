package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/carsonfarmer/beaver/pkg/extensions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	"github.com/carsonfarmer/beaver/pkg/storage"
	acp "github.com/ironpark/go-acp"
)

func TestCancel_NoActivePrompt(t *testing.T) {
	a := newTestAgent(t)
	id := createTestSession(t, a, "/tmp")
	if err := a.Cancel(context.Background(), &acp.CancelNotification{SessionID: id}); err != nil {
		t.Fatal(err)
	}
}

func TestCancel_CancelsActivePrompt(t *testing.T) {
	a := newTestAgent(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	id := acp.SessionID("sess")
	setupSession(t, a, id, &session.State{Cancel: cancel})
	a.Cancel(context.Background(), &acp.CancelNotification{SessionID: id})
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}

func TestPrompt_SessionNotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.Prompt(context.Background(), &acp.PromptRequest{SessionID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPrompt_NoModel(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := acp.SessionID("sess")
	setupSession(t, a, id, &session.State{Model: ""})
	_, err := a.Prompt(context.Background(), &acp.PromptRequest{SessionID: id})
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestPrompt_ModelResolveError(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := acp.SessionID("sess")
	setupSession(t, a, id, &session.State{Model: "nonexistent/model"})
	_, err := a.Prompt(context.Background(), &acp.PromptRequest{SessionID: id})
	if err == nil {
		t.Fatal("expected error for unresolvable model")
	}
}

func TestPrompt_Success(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := createTestSession(t, a, "/tmp")
	sess, _ := a.cachedSession(id)
	sess.Model = "mock/model"

	resp, err := a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("hi")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StopReason != acp.StopReasonEndTurn {
		t.Fatalf("expected end_turn, got %v", resp.StopReason)
	}
	sess, _ = a.cachedSession(id)
	if len(sess.History) < 2 {
		t.Fatalf("expected at least 2 messages in history, got %d", len(sess.History))
	}
}

func TestPrompt_AppendsUserMessage(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := createTestSession(t, a, "/tmp")
	sess, _ := a.cachedSession(id)
	sess.Model = "mock/model"

	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("first")},
	})
	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("second")},
	})

	sess, _ = a.cachedSession(id)
	if len(sess.History) < 4 {
		t.Fatalf("expected at least 4 messages, got %d", len(sess.History))
	}
}

func TestPrompt_SendsUsageUpdate(t *testing.T) {
	reg := &mockRegistry{
		model:    &mockLanguageModel{text: "hi"},
		defaults: llm.Defaults{Model: "mock/model"},
		opts:     llm.ModelOptions{ContextWindow: 128000},
	}
	a := New(WithRegistry(reg), WithStorage(storage.NewMemArchive()))
	mc := &mockClient{}
	a.SetClient(mc)
	id := acp.SessionID("sess")
	setupSession(t, a, id, &session.State{Model: "mock/model"})

	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("hello")},
	})
	if !mc.sessionUpdateCalled {
		t.Fatal("expected usage update notification")
	}
}

func TestNewSession_DiscoversContext(t *testing.T) {
	reg := newTestRegistry()
	reg.model = &mockLanguageModel{text: "ok"}
	mc := &mockClient{
		files: map[string]string{"/project/AGENTS.md": "# Rules"},
	}
	a := New(WithRegistry(reg), WithStorage(storage.NewMemArchive()))
	a.SetClient(mc)
	a.SetProviders(
		extensions.BasePrompt(extensions.DefaultPrompt),
		extensions.AgentsMd(mc),
	)

	resp, err := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: "/project"})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := a.cachedSession(resp.SessionID)
	if !strings.Contains(sess.SystemPrompt, "# Rules") {
		t.Fatal("expected context in system prompt after session creation")
	}
}

func TestNewSession_DiscoversSkills(t *testing.T) {
	reg := newTestRegistry()
	reg.model = &mockLanguageModel{text: "ok"}
	mc := &mockClient{
		termOut: "my-skill\n",
		files: map[string]string{
			"/project/.agents/skills/my-skill/SKILL.md": "---\nname: my-skill\ndescription: Does things.\n---\nBody.",
		},
	}
	a := New(WithRegistry(reg), WithStorage(storage.NewMemArchive()))
	a.SetClient(mc)
	a.SetProviders(
		extensions.BasePrompt(extensions.DefaultPrompt),
		extensions.Skills(mc),
	)

	resp, err := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: "/project"})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := a.cachedSession(resp.SessionID)
	if !strings.Contains(sess.SystemPrompt, "my-skill") {
		t.Fatal("expected skill in system prompt after session creation")
	}
}

func TestPrompt_RewindToParentMessage(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := createTestSession(t, a, "/tmp")
	sess, _ := a.cachedSession(id)
	sess.Model = "mock/model"

	resp1, _ := a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("first")},
	})
	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("second")},
	})

	_, err := a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("third")},
		Meta:      map[string]any{"parent_message_id": resp1.UserMessageID},
	})
	if err != nil {
		t.Fatal(err)
	}

	events, _ := storage.Lineage(a.archive, a.archive.Tip(id))

	var firstEvt storage.EventID
	for _, ev := range events {
		if ev.Update == nil {
			continue
		}
		if c, ok := ev.Update.AsUserMessageChunk(); ok && c.MessageID == resp1.UserMessageID {
			firstEvt = ev.ID
			break
		}
	}
	if firstEvt.IsZero() {
		t.Fatal("first message event not found")
	}

	var found bool
	for _, ev := range events {
		if ev.Update == nil {
			continue
		}
		if c, ok := ev.Update.AsUserMessageChunk(); ok && c.MessageID != resp1.UserMessageID {
			if ev.Parent == firstEvt {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("expected a later user message chunk to parent off the first message's event")
	}
}

func TestPrompt_RewindInvalidMessageID(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := createTestSession(t, a, "/tmp")
	sess, _ := a.cachedSession(id)
	sess.Model = "mock/model"

	_, err := a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("hi")},
		Meta:      map[string]any{"parent_message_id": "does-not-exist"},
	})
	if err == nil {
		t.Fatal("expected error when parent_message_id not found")
	}
}
