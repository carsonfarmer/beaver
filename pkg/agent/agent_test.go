package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/instructions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	acp "github.com/ironpark/go-acp"
)

type mockClient struct {
	sessionUpdateCalled bool
	files               map[string]string // path → content for ReadTextFile
	termOut             string            // output for TerminalOutput
}

func (m *mockClient) SessionUpdate(_ context.Context, _ *acp.SessionNotification) error {
	m.sessionUpdateCalled = true
	return nil
}
func (m *mockClient) RequestPermission(_ context.Context, _ *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	return nil, nil
}
func (m *mockClient) ReadTextFile(_ context.Context, req *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	if m.files != nil {
		if c, ok := m.files[req.Path]; ok {
			return &acp.ReadTextFileResponse{Content: c}, nil
		}
		return nil, fmt.Errorf("not found: %s", req.Path)
	}
	return &acp.ReadTextFileResponse{}, nil
}
func (m *mockClient) WriteTextFile(_ context.Context, _ *acp.WriteTextFileRequest) (*acp.WriteTextFileResponse, error) {
	return &acp.WriteTextFileResponse{}, nil
}
func (m *mockClient) CreateTerminal(_ context.Context, _ *acp.CreateTerminalRequest) (*acp.CreateTerminalResponse, error) {
	return &acp.CreateTerminalResponse{TerminalID: "t"}, nil
}
func (m *mockClient) TerminalOutput(_ context.Context, _ *acp.TerminalOutputRequest) (*acp.TerminalOutputResponse, error) {
	return &acp.TerminalOutputResponse{Output: m.termOut}, nil
}
func (m *mockClient) ReleaseTerminal(_ context.Context, _ *acp.ReleaseTerminalRequest) (*acp.ReleaseTerminalResponse, error) {
	return &acp.ReleaseTerminalResponse{}, nil
}
func (m *mockClient) WaitForTerminalExit(_ context.Context, _ *acp.WaitForTerminalExitRequest) (*acp.WaitForTerminalExitResponse, error) {
	return &acp.WaitForTerminalExitResponse{}, nil
}
func (m *mockClient) KillTerminalCommand(_ context.Context, _ *acp.KillTerminalRequest) (*acp.KillTerminalResponse, error) {
	return &acp.KillTerminalResponse{}, nil
}

// mockLanguageModel implements fantasy.LanguageModel for testing.
type mockLanguageModel struct {
	text string
}

func (m *mockLanguageModel) Generate(_ context.Context, _ fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{
		Content:      fantasy.ResponseContent{fantasy.TextContent{Text: m.text}},
		FinishReason: fantasy.FinishReasonStop,
		Usage:        fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
	}, nil
}

func (m *mockLanguageModel) Stream(_ context.Context, _ fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "t1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: m.text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"}) {
			return
		}
		yield(fantasy.StreamPart{
			Type:         fantasy.StreamPartTypeFinish,
			Usage:        fantasy.Usage{InputTokens: 10, OutputTokens: 5, TotalTokens: 15},
			FinishReason: fantasy.FinishReasonStop,
		})
	}, nil
}

func (m *mockLanguageModel) GenerateObject(_ context.Context, _ fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLanguageModel) StreamObject(_ context.Context, _ fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLanguageModel) Provider() string { return "mock" }
func (m *mockLanguageModel) Model() string    { return "mock-model" }

// mockRegistry implements llm.ModelRegistry for testing.
type mockRegistry struct {
	model    fantasy.LanguageModel
	defaults llm.Defaults
	opts     llm.ModelOptions
}

func (r *mockRegistry) ResolveModel(_ context.Context, _ string) (fantasy.LanguageModel, error) {
	if r.model != nil {
		return r.model, nil
	}
	return nil, fmt.Errorf("no model configured")
}

func (r *mockRegistry) ModelOptions(_, _ string) llm.ModelOptions { return r.opts }
func (r *mockRegistry) Defaults() llm.Defaults                   { return r.defaults }
func (r *mockRegistry) AvailableModelGroups() []llm.ModelGroup    { return nil }

func newTestRegistry() *mockRegistry {
	return &mockRegistry{
		defaults: llm.Defaults{Model: "test/model", ThoughtLevel: "medium"},
	}
}

func newTestAgent(t *testing.T) *Agent {
	t.Helper()
	return New(newTestRegistry(), acp.NewMemoryStore[*session.Session](), instructions.New())
}

func TestSetSessionConfigOption_Model(t *testing.T) {
	a := newTestAgent(t)
	id := acp.SessionID("test-sess")
	a.store.Set(id, &session.Session{})

	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: id, ConfigID: session.ConfigModel, Value: "openai/gpt-4.1-mini",
	})
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.store.Get(id)
	if s.Model != "openai/gpt-4.1-mini" {
		t.Fatalf("expected model %q, got %q", "openai/gpt-4.1-mini", s.Model)
	}
}

func TestSetSessionConfigOption_ThoughtLevel(t *testing.T) {
	a := newTestAgent(t)
	id := acp.SessionID("test-sess")
	a.store.Set(id, &session.Session{})

	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: id, ConfigID: session.ConfigThoughtLevel, Value: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.store.Get(id)
	if s.ThoughtLevel != "high" {
		t.Fatalf("expected %q, got %q", "high", s.ThoughtLevel)
	}
}

func TestSetSessionConfigOption_SessionNotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: "nonexistent", ConfigID: session.ConfigModel, Value: "openai/gpt-4.1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetSessionConfigOption_UnknownConfigID(t *testing.T) {
	a := newTestAgent(t)
	id := acp.SessionID("test-sess")
	a.store.Set(id, &session.Session{Model: "openai/gpt-4.1"})

	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: id, ConfigID: "unknown", Value: "whatever",
	})
	if err != nil {
		t.Fatal("unknown config IDs should not error")
	}
	s, _ := a.store.Get(id)
	if s.Model != "openai/gpt-4.1" {
		t.Fatal("model should be unchanged")
	}
}

func TestInitialize(t *testing.T) {
	resp, err := newTestAgent(t).Initialize(context.Background(), &acp.InitializeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AgentInfo.Name != "beaver" {
		t.Fatalf("expected %q, got %q", "beaver", resp.AgentInfo.Name)
	}
}

func TestAuthenticate(t *testing.T) {
	_, err := newTestAgent(t).Authenticate(context.Background(), &acp.AuthenticateRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetSessionMode(t *testing.T) {
	_, err := newTestAgent(t).SetSessionMode(context.Background(), &acp.SetSessionModeRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCancel_NoActivePrompt(t *testing.T) {
	a := newTestAgent(t)
	a.store.Set("sess", &session.Session{})
	if err := a.Cancel(context.Background(), &acp.CancelNotification{SessionID: "sess"}); err != nil {
		t.Fatal(err)
	}
}

func TestCancel_CancelsActivePrompt(t *testing.T) {
	a := newTestAgent(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	a.store.Set("sess", &session.Session{Cancel: cancel})
	a.Cancel(context.Background(), &acp.CancelNotification{SessionID: "sess"})
	if ctx.Err() == nil {
		t.Fatal("expected context to be cancelled")
	}
}

func TestSetClient(t *testing.T) {
	a := newTestAgent(t)
	mc := &mockClient{}
	a.SetClient(mc)
	if a.client != mc {
		t.Fatal("expected client to be set")
	}
}

func TestConfigOptions(t *testing.T) {
	reg := newTestRegistry()
	sess := &session.Session{Model: "test/model", ThoughtLevel: "medium"}
	opts := session.ConfigOptions(reg, sess)
	if len(opts) != 2 {
		t.Fatalf("expected 2, got %d", len(opts))
	}
}

func TestNewSession(t *testing.T) {
	a := newTestAgent(t)
	resp, err := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: "/tmp/project"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SessionID == "" {
		t.Fatal("expected session ID")
	}
	sess, ok := a.store.Get(resp.SessionID)
	if !ok {
		t.Fatal("expected session in store")
	}
	if sess.Cwd != "/tmp/project" {
		t.Fatalf("expected cwd %q, got %q", "/tmp/project", sess.Cwd)
	}
	if sess.Model != "test/model" {
		t.Fatalf("expected default model %q, got %q", "test/model", sess.Model)
	}
}

func TestPrompt_DiscoversContextLazily(t *testing.T) {
	reg := newTestRegistry()
	reg.model = &mockLanguageModel{text: "ok"}
	a := New(reg, acp.NewMemoryStore[*session.Session](), instructions.New())
	a.SetClient(&mockClient{
		files: map[string]string{"/project/AGENTS.md": "# Rules"},
	})
	resp, _ := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: "/project"})
	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: resp.SessionID,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("hi")},
	})
	sess, _ := a.store.Get(resp.SessionID)
	if !strings.Contains(sess.SystemPrompt, "# Rules") {
		t.Fatal("expected context in system prompt after first prompt")
	}
}

func TestPrompt_DiscoversSkillsLazily(t *testing.T) {
	reg := newTestRegistry()
	reg.model = &mockLanguageModel{text: "ok"}
	a := New(reg, acp.NewMemoryStore[*session.Session](), instructions.New())
	a.SetClient(&mockClient{
		termOut: "my-skill\n",
		files: map[string]string{
			"/project/.agents/skills/my-skill/SKILL.md": "---\nname: my-skill\ndescription: Does things.\n---\nBody.",
		},
	})
	resp, _ := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: "/project"})
	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: resp.SessionID,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("hi")},
	})
	sess, _ := a.store.Get(resp.SessionID)
	if !strings.Contains(sess.SystemPrompt, "my-skill") {
		t.Fatal("expected skill in system prompt after first prompt")
	}
}

func TestLoadSession(t *testing.T) {
	a := newTestAgent(t)
	id := acp.SessionID("load-test")
	a.store.Set(id, &session.Session{Model: "openai/gpt-4.1"})

	_, err := a.LoadSession(context.Background(), &acp.LoadSessionRequest{SessionID: id})
	if err != nil {
		t.Fatal(err)
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
	a.store.Set("sess-a", &session.Session{Cwd: "/a"})
	a.store.Set("sess-b", &session.Session{Cwd: "/b"})

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
	id := acp.SessionID("fork-src")
	a.store.Set(id, &session.Session{Model: "openai/gpt-4.1", Cwd: "/old"})

	resp, err := a.ForkSession(context.Background(), &acp.ForkSessionRequest{SessionID: id, Cwd: "/new"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.SessionID == "" || resp.SessionID == id {
		t.Fatal("expected new session ID")
	}
	forked, ok := a.store.Get(resp.SessionID)
	if !ok {
		t.Fatal("expected forked session in store")
	}
	if forked.Cwd != "/new" {
		t.Fatalf("expected cwd %q, got %q", "/new", forked.Cwd)
	}
	if forked.Model != "openai/gpt-4.1" {
		t.Fatalf("expected model copied, got %q", forked.Model)
	}
}

func TestForkSession_NotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.ForkSession(context.Background(), &acp.ForkSessionRequest{SessionID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestResumeSession(t *testing.T) {
	a := newTestAgent(t)
	id := acp.SessionID("resume-test")
	a.store.Set(id, &session.Session{})

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

func TestPrompt_SessionNotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.Prompt(context.Background(), &acp.PromptRequest{SessionID: "nonexistent"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestPrompt_NoModel(t *testing.T) {
	a := newTestAgent(t)
	a.store.Set("sess", &session.Session{Model: ""})
	_, err := a.Prompt(context.Background(), &acp.PromptRequest{SessionID: "sess"})
	if err == nil {
		t.Fatal("expected error for empty model")
	}
}

func TestPrompt_ModelResolveError(t *testing.T) {
	a := newTestAgent(t)
	a.store.Set("sess", &session.Session{Model: "nonexistent/model"})
	_, err := a.Prompt(context.Background(), &acp.PromptRequest{SessionID: "sess"})
	if err == nil {
		t.Fatal("expected error for unresolvable model")
	}
}

func newTestAgentWithModel(t *testing.T) *Agent {
	t.Helper()
	reg := newTestRegistry()
	reg.model = &mockLanguageModel{text: "Hello from mock!"}
	a := New(reg, acp.NewMemoryStore[*session.Session](), instructions.New())
	a.SetClient(&mockClient{})
	return a
}

func TestPrompt_Success(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := acp.SessionID("prompt-test")
	a.store.Set(id, &session.Session{Model: "mock/model"})

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
	sess, _ := a.store.Get(id)
	if len(sess.History) < 2 {
		t.Fatalf("expected at least 2 messages in history, got %d", len(sess.History))
	}
}

func TestPrompt_AppendsUserMessage(t *testing.T) {
	a := newTestAgentWithModel(t)
	id := acp.SessionID("msg-test")
	a.store.Set(id, &session.Session{Model: "mock/model"})

	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("first")},
	})
	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: id,
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("second")},
	})

	sess, _ := a.store.Get(id)
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
	a := New(reg, acp.NewMemoryStore[*session.Session](), instructions.New())
	mc := &mockClient{}
	a.SetClient(mc)
	a.store.Set("sess", &session.Session{Model: "mock/model"})

	a.Prompt(context.Background(), &acp.PromptRequest{
		SessionID: "sess",
		Prompt:    []acp.ContentBlock{acp.NewContentBlockText("hello")},
	})
	if !mc.sessionUpdateCalled {
		t.Fatal("expected usage update notification")
	}
}
