package agent

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/eventlog"
	"github.com/carsonfarmer/beaver/pkg/instructions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	acp "github.com/ironpark/go-acp"
)

type mockClient struct {
	sessionUpdateCalled bool
	files               map[string]string
	termOut             string
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

type mockLanguageModel struct{ text string }

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
	return New(newTestRegistry(), eventlog.NewMemStore(), instructions.New())
}

func newTestAgentWithModel(t *testing.T) *Agent {
	t.Helper()
	reg := newTestRegistry()
	reg.model = &mockLanguageModel{text: "Hello from mock!"}
	a := New(reg, eventlog.NewMemStore(), instructions.New())
	a.SetClient(&mockClient{})
	return a
}

// createTestSession creates a session via the agent and returns its ID.
func createTestSession(t *testing.T, a *Agent, cwd string) acp.SessionID {
	t.Helper()
	resp, err := a.NewSession(context.Background(), &acp.NewSessionRequest{Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	return resp.SessionID
}

// setupSession creates a log and caches a session with the given fields.
func setupSession(t *testing.T, a *Agent, id acp.SessionID, sess *session.Session) {
	t.Helper()
	_, err := a.store.Create(id, acp.SessionInfo{SessionID: id, Cwd: sess.Cwd})
	if err != nil {
		t.Fatal(err)
	}
	a.setSession(id, sess)
}
