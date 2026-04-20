package agent

import (
	"context"
	"sync"

	"github.com/carsonfarmer/beaver/pkg/eventlog"
	"github.com/carsonfarmer/beaver/pkg/instructions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	"github.com/google/uuid"

	acp "github.com/ironpark/go-acp"
)

// Agent implements acp.Agent and the optional session lifecycle interfaces.
type Agent struct {
	registry     llm.ModelRegistry
	store        eventlog.Store
	client       acp.Client
	instructions *instructions.Instructions

	mu       sync.RWMutex
	sessions map[acp.SessionID]*session.Session
}

// New creates a new Agent with the given dependencies.
func New(registry llm.ModelRegistry, store eventlog.Store, insts *instructions.Instructions) *Agent {
	return &Agent{
		registry:     registry,
		store:        store,
		instructions: insts,
		sessions:     make(map[acp.SessionID]*session.Session),
	}
}

// SetClient sets the ACP client for outbound calls and notifications.
func (a *Agent) SetClient(c acp.Client) { a.client = c }

func (a *Agent) Initialize(_ context.Context, _ *acp.InitializeRequest) (*acp.InitializeResponse, error) {
	return &acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(acp.CurrentProtocolVersion),
		AgentCapabilities: &acp.AgentCapabilities{
			LoadSession:     true,
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
	client := &eventlog.LoggingClient{Client: a.client, Store: a.store}
	acp.NewSessionStream(client, req.SessionID).SendConfigUpdate(ctx, opts)
	return &acp.SetSessionConfigOptionResponse{ConfigOptions: opts}, nil
}

// --- cache and store helpers ---

func (a *Agent) cachedSession(id acp.SessionID) (*session.Session, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	s, ok := a.sessions[id]
	return s, ok
}

func (a *Agent) loadedSession(id acp.SessionID) (*session.Session, error) {
	if s, ok := a.cachedSession(id); ok {
		return s, nil
	}
	return a.loadState(id)
}

func (a *Agent) setSession(id acp.SessionID, s *session.Session) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sessions[id] = s
}

func (a *Agent) deleteSession(id acp.SessionID) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.sessions, id)
}

func (a *Agent) loadState(id acp.SessionID) (*session.Session, error) {
	log, err := a.store.Open(id)
	if err != nil {
		return nil, err
	}
	events, err := log.Read(context.Background(), uuid.Nil)
	if err != nil {
		return nil, err
	}
	sess := eventlog.Reduce(events)
	a.setSession(id, sess)
	return sess, nil
}

func (a *Agent) replayEvents(ctx context.Context, sid acp.SessionID) {
	log, err := a.store.Open(sid)
	if err != nil {
		return
	}
	events, err := log.Read(ctx, uuid.Nil)
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
