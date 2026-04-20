package agent

import (
	"context"

	"github.com/carsonfarmer/beaver/pkg/eventlog"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	"github.com/google/uuid"

	acp "github.com/ironpark/go-acp"
)

func (a *Agent) NewSession(ctx context.Context, req *acp.NewSessionRequest) (*acp.NewSessionResponse, error) {
	id := acp.GenerateSessionID()
	sess := &session.Session{
		Cwd:          req.Cwd,
		Model:        a.registry.Defaults().Model,
		ThoughtLevel: a.registry.Defaults().ThoughtLevel,
	}

	log, err := a.store.Create(id, acp.SessionInfo{SessionID: id, Cwd: req.Cwd})
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}

	resp := &acp.NewSessionResponse{
		SessionID:     id,
		ConfigOptions: llm.SessionOptions(a.registry, sess.Model, sess.ThoughtLevel),
	}
	if err := log.Append(ctx, acp.NewSessionUpdateConfigOptionUpdate(resp.ConfigOptions)); err != nil {
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
	go a.replayEvents(ctx, req.SessionID)
	return &acp.LoadSessionResponse{
		ConfigOptions: llm.SessionOptions(a.registry, sess.Model, sess.ThoughtLevel),
	}, nil
}

func (a *Agent) ListSessions(_ context.Context, _ *acp.ListSessionsRequest) (*acp.ListSessionsResponse, error) {
	ids, err := a.store.List()
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	sessions := make([]acp.SessionInfo, 0, len(ids))
	for _, id := range ids {
		sess, ok := a.cachedSession(id)
		if !ok {
			var err error
			sess, err = a.loadState(id)
			if err != nil {
				continue
			}
		}
		sessions = append(sessions, acp.SessionInfo{
			SessionID: id,
			Cwd:       sess.Cwd,
			Title:     sess.Title,
			UpdatedAt: sess.UpdatedAt,
		})
	}
	return &acp.ListSessionsResponse{Sessions: sessions}, nil
}

func (a *Agent) ForkSession(ctx context.Context, req *acp.ForkSessionRequest) (*acp.ForkSessionResponse, error) {
	srcLog, err := a.store.Open(req.SessionID)
	if err != nil {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}

	// Optional: fork at a specific event ID (rewind). Defaults to tip.
	var forkAt uuid.UUID
	if s, ok := req.Meta["fork_at_event_id"].(string); ok && s != "" {
		p, err := uuid.Parse(s)
		if err != nil {
			return nil, acp.ErrInvalidParams(nil, "invalid fork_at_event_id")
		}
		forkAt = p
	}
	srcEvents, err := srcLog.Read(ctx, forkAt)
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}

	newID := acp.GenerateSessionID()
	newLog, err := a.store.Create(newID, acp.SessionInfo{
		SessionID: newID,
		Cwd:       req.Cwd,
		Meta:      map[string]any{"parent_session_id": string(req.SessionID)},
	})
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	// Skip source's primer; fork supplies its own. Source event IDs don't
	// apply in the new log, so strip any parent_event_id marker — the chain
	// is preserved by appending in order (lastID-based parenting).
	for _, ev := range srcEvents[1:] {
		if ev.Update == nil {
			continue
		}
		newLog.Append(ctx, eventlog.WithParentEventID(*ev.Update, uuid.Nil))
	}

	if _, err := a.loadState(newID); err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	return &acp.ForkSessionResponse{SessionID: newID}, nil
}

func (a *Agent) ResumeSession(_ context.Context, req *acp.ResumeSessionRequest) (*acp.ResumeSessionResponse, error) {
	if _, ok := a.cachedSession(req.SessionID); !ok {
		if _, err := a.loadState(req.SessionID); err != nil {
			return nil, acp.ErrInvalidParams(nil, "session not found")
		}
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
