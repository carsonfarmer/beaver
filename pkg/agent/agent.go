package agent

import (
	"context"
	"sync"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/extensions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	"github.com/carsonfarmer/beaver/pkg/storage"

	acp "github.com/ironpark/go-acp"
)

// Agent implements acp.Agent and the optional session lifecycle interfaces.
type Agent struct {
	client    acp.Client
	registry  llm.ModelRegistry
	archive   storage.Archive
	tools     []fantasy.AgentTool
	providers []extensions.Provider

	mu       sync.RWMutex
	sessions map[acp.SessionID]*session.State
}

// New creates a new Agent with the given options.
func New(opts ...Option) *Agent {
	a := &Agent{
		sessions: make(map[acp.SessionID]*session.State),
	}
	for _, opt := range opts {
		opt(a)
	}
	return a
}

// SetClient sets the ACP client for outbound calls and notifications.
// This is required when the client cannot be provided at construction time
// due to a circular dependency (e.g., the connection wraps the agent).
func (a *Agent) SetClient(c acp.Client) { a.client = c }

// SetTools registers the agent's tool set. Replaces any previously set tools.
func (a *Agent) SetTools(tools ...fantasy.AgentTool) { a.tools = tools }

// SetProviders registers context providers that contribute to the system prompt.
// Replaces any previously set providers.
func (a *Agent) SetProviders(providers ...extensions.Provider) { a.providers = providers }

func (a *Agent) Initialize(_ context.Context, _ *acp.InitializeRequest) (*acp.InitializeResponse, error) {
	return &acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(acp.CurrentProtocolVersion),
		AgentCapabilities: &acp.AgentCapabilities{
			LoadSession: true,
			MCPCapabilities: &acp.MCPCapabilities{},
			PromptCapabilities: &acp.PromptCapabilities{
				Image:           true,
				EmbeddedContext: true,
			},
			SessionCapabilities: &acp.SessionCapabilities{
				List:   &acp.SessionListCapabilities{},
				Fork:   &acp.SessionForkCapabilities{},
				Resume: &acp.SessionResumeCapabilities{},
			},
		},
		AgentInfo:   &acp.Implementation{Name: "beaver", Title: "Beaver", Version: "0.1.0"},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

func (a *Agent) Authenticate(_ context.Context, _ *acp.AuthenticateRequest) (*acp.AuthenticateResponse, error) {
	return &acp.AuthenticateResponse{}, nil
}

func (a *Agent) SetSessionMode(_ context.Context, _ *acp.SetSessionModeRequest) (*acp.SetSessionModeResponse, error) {
	return &acp.SetSessionModeResponse{}, nil
}

func (a *Agent) SetSessionConfigOption(ctx context.Context, req *acp.SetSessionConfigOptionRequest) (*acp.SetSessionConfigOptionResponse, error) {
	sess, err := a.loadedSession(req.SessionID)
	if err != nil {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	switch req.ConfigID {
	case llm.SessionConfigModel:
		sess.Model = string(req.Value)
	case llm.SessionConfigThoughtLevel:
		if !llm.ValidThoughtLevel(req.Value) {
			return nil, acp.ErrInvalidParams(nil, "invalid thought level: "+string(req.Value))
		}
		sess.ThoughtLevel = string(req.Value)
	}
	opts := llm.SessionOptions(a.registry, sess.Model, sess.ThoughtLevel)
	client := &LoggingClient{Client: a.client, Archive: a.archive}
	acp.NewSessionStream(client, req.SessionID).SendConfigUpdate(ctx, opts)
	return &acp.SetSessionConfigOptionResponse{ConfigOptions: opts}, nil
}

// --- cache and archive helpers ---

func (a *Agent) cachedSession(id acp.SessionID) (*session.State, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.sessions[id]
	return s, ok
}

func (a *Agent) loadedSession(id acp.SessionID) (*session.State, error) {
	if s, ok := a.cachedSession(id); ok {
		return s, nil
	}
	return a.loadState(id)
}

func (a *Agent) setSession(id acp.SessionID, s *session.State) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[id] = s
}

func (a *Agent) deleteSession(id acp.SessionID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, id)
}

// loadState rebuilds the session's runtime state by walking its lineage
// and folding events through the projection.
func (a *Agent) loadState(id acp.SessionID) (*session.State, error) {
	tip := a.archive.Tip(id)
	if tip.IsZero() {
		return nil, errSessionNotFound
	}
	events, err := storage.Lineage(a.archive, tip)
	if err != nil {
		return nil, err
	}
	state := session.Project(events)
	a.setSession(id, state)
	return state, nil
}

// replayEvents streams the session's full lineage back to the client, used
// on LoadSession so the client can rebuild its UI.
func (a *Agent) replayEvents(ctx context.Context, sid acp.SessionID) {
	tip := a.archive.Tip(sid)
	if tip.IsZero() {
		return
	}
	events, err := storage.Lineage(a.archive, tip)
	if err != nil {
		return
	}
	stream := acp.NewSessionStream(a.client, sid)
	for _, ev := range events {
		if ev.Update == nil {
			continue
		}
		stream.SendUpdate(ctx, *ev.Update)
	}
}
