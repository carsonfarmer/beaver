package agent

import (
	"context"

	"github.com/carsonfarmer/beaver/pkg/extensions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	"github.com/carsonfarmer/beaver/pkg/storage"

	acp "github.com/ironpark/go-acp"
)

func (a *Agent) NewSession(ctx context.Context, req *acp.NewSessionRequest) (*acp.NewSessionResponse, error) {
	id := acp.GenerateSessionID()
	sess := &session.State{
		Cwd:          req.Cwd,
		Model:        a.registry.Defaults().Model,
		ThoughtLevel: a.registry.Defaults().ThoughtLevel,
	}

	if err := a.archive.Create(id, storage.EventID{}, acp.SessionInfo{SessionID: id, Cwd: req.Cwd}); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}

	if err := a.initSession(ctx, id, sess); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}

	resp := &acp.NewSessionResponse{
		SessionID:     id,
		ConfigOptions: llm.SessionOptions(a.registry, sess.Model, sess.ThoughtLevel),
	}
	if _, err := a.archive.Append(id, storage.EventID{}, acp.NewSessionUpdateConfigOptionUpdate(resp.ConfigOptions)); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	a.setSession(id, sess)
	return resp, nil
}

func (a *Agent) LoadSession(ctx context.Context, req *acp.LoadSessionRequest) (*acp.LoadSessionResponse, error) {
	sess, err := a.loadState(req.SessionID)
	if err != nil {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	if err := a.initSession(ctx, req.SessionID, sess); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	go a.replayEvents(ctx, req.SessionID)
	return &acp.LoadSessionResponse{
		ConfigOptions: llm.SessionOptions(a.registry, sess.Model, sess.ThoughtLevel),
	}, nil
}

func (a *Agent) ListSessions(_ context.Context, _ *acp.ListSessionsRequest) (*acp.ListSessionsResponse, error) {
	infos, err := a.archive.List()
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	sessions := make([]acp.SessionInfo, 0, len(infos))
	for _, info := range infos {
		// Prefer cached runtime state (has evolved Title/UpdatedAt).
		if sess, ok := a.cachedSession(info.SessionID); ok {
			sessions = append(sessions, acp.SessionInfo{
				SessionID: info.SessionID,
				Cwd:       sess.Cwd,
				Title:     sess.Title,
				UpdatedAt: sess.UpdatedAt,
			})
			continue
		}
		if sess, err := a.loadState(info.SessionID); err == nil {
			sessions = append(sessions, acp.SessionInfo{
				SessionID: info.SessionID,
				Cwd:       sess.Cwd,
				Title:     sess.Title,
				UpdatedAt: sess.UpdatedAt,
			})
			continue
		}
		sessions = append(sessions, info)
	}
	return &acp.ListSessionsResponse{Sessions: sessions}, nil
}

func (a *Agent) ForkSession(ctx context.Context, req *acp.ForkSessionRequest) (*acp.ForkSessionResponse, error) {
	srcTip := a.archive.Tip(req.SessionID)
	if srcTip.IsZero() {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}

	parent := srcTip
	if s, ok := req.Meta["fork_at_event_id"].(string); ok && s != "" {
		p, err := storage.ParseEventID(s)
		if err != nil {
			return nil, acp.ErrInvalidParams(nil, err.Error())
		}
		parent = p
	}

	newID := acp.GenerateSessionID()
	if err := a.archive.Create(newID, parent, acp.SessionInfo{
		SessionID: newID,
		Cwd:       req.Cwd,
		Meta:      map[string]any{"parent_session_id": string(req.SessionID)},
	}); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	sess, err := a.loadState(newID)
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	if err := a.initSession(ctx, newID, sess); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	return &acp.ForkSessionResponse{SessionID: newID}, nil
}

func (a *Agent) ResumeSession(ctx context.Context, req *acp.ResumeSessionRequest) (*acp.ResumeSessionResponse, error) {
	if _, ok := a.cachedSession(req.SessionID); ok {
		return &acp.ResumeSessionResponse{}, nil
	}
	sess, err := a.loadState(req.SessionID)
	if err != nil {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	if err := a.initSession(ctx, req.SessionID, sess); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	return &acp.ResumeSessionResponse{}, nil
}

func (a *Agent) CloseSession(_ context.Context, req *acp.CloseSessionRequest) (*acp.CloseSessionResponse, error) {
	if sess, ok := a.cachedSession(req.SessionID); ok && sess.Cancel != nil {
		sess.Cancel()
	}
	a.deleteSession(req.SessionID)
	return &acp.CloseSessionResponse{}, nil
}

// initSession runs context providers once to build the system prompt.
func (a *Agent) initSession(ctx context.Context, sid acp.SessionID, sess *session.State) error {
	if len(a.providers) == 0 {
		return nil
	}
	prompt, err := extensions.Assemble(ctx, sess.Cwd, sid, a.providers)
	if err != nil {
		return err
	}
	sess.SystemPrompt = prompt
	return nil
}
