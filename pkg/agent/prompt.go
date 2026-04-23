package agent

import (
	"context"
	"encoding/json"
	"strings"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/storage"
	"github.com/carsonfarmer/beaver/pkg/tools"

	acp "github.com/ironpark/go-acp"
)

func (a *Agent) Cancel(_ context.Context, req *acp.CancelNotification) error {
	if sess, ok := a.cachedSession(req.SessionID); ok && sess.Cancel != nil {
		sess.Cancel()
	}
	return nil
}

func (a *Agent) Prompt(ctx context.Context, req *acp.PromptRequest) (*acp.PromptResponse, error) {
	sess, err := a.loadedSession(req.SessionID)
	if err != nil {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	if sess.Model == "" {
		return nil, acp.ErrInvalidParams(nil, "no model configured on session")
	}

	if sess.Cancel != nil {
		sess.Cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	sess.Cancel = cancel
	defer func() {
		cancel()
		sess.Cancel = nil
	}()

	model, err := a.registry.ResolveModel(ctx, sess.Model)
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	opts := a.registry.ModelOptions(sess.Model, string(sess.ThoughtLevel))

	// Optional rewind: parent_message_id parses directly to an EventID.
	var rewindParent storage.EventID
	if s, ok := req.Meta["parent_message_id"].(string); ok && s != "" {
		p, err := storage.ParseEventID(s)
		if err != nil {
			return nil, acp.ErrInvalidParams(nil, err.Error())
		}
		rewindParent = p
	}

	ctx = tools.WithSessionID(ctx, req.SessionID)
	ctx = tools.WithCwd(ctx, sess.Cwd)

	client := &LoggingClient{Client: a.client, Archive: a.archive}
	stream := acp.NewSessionStream(client, req.SessionID)

	var userMsgID string
	if msg, ok := llm.ContentBlocksToMessage(req.Prompt); ok {
		sess.History = append(sess.History, msg)
		if sess.Title == "" {
			sess.Title = titleFromMessage(msg)
		}
		for _, part := range msg.Content {
			tp, ok := part.(fantasy.TextPart)
			if !ok {
				continue
			}
			nextID := storage.EventID{Session: req.SessionID, N: a.archive.Tip(req.SessionID).N + 1}
			upd := acp.NewSessionUpdateUserMessageChunk(
				acp.NewContentBlockText(tp.Text), nextID.String())
			id, _ := a.archive.Append(req.SessionID, rewindParent, upd)
			rewindParent = storage.EventID{}
			userMsgID = id.String()
		}
	}

	fa := fantasy.NewAgent(model,
		fantasy.WithTools(a.tools...),
		fantasy.WithSystemPrompt(sess.SystemPrompt),
	)

	// Per-part state: buffer deltas + the EventID we predicted at Start,
	// used for both streaming correlation and final append.
	type partState struct {
		buf *strings.Builder
		id  string
	}
	parts := map[string]*partState{}
	predictID := func() string {
		return storage.EventID{Session: req.SessionID, N: a.archive.Tip(req.SessionID).N + 1}.String()
	}

	result, err := fa.Stream(ctx, fantasy.AgentStreamCall{
		Messages:        sess.History,
		MaxOutputTokens: opts.MaxOutputTokens,
		ProviderOptions: opts.ProviderOptions,
		OnTextStart: func(fid string) error {
			parts[fid] = &partState{buf: &strings.Builder{}, id: predictID()}
			return nil
		},
		OnTextDelta: func(fid, delta string) error {
			p := parts[fid]
			p.buf.WriteString(delta)
			return stream.SendText(ctx, delta, acp.WithMessageID(p.id))
		},
		OnTextEnd: func(fid string) error {
			p := parts[fid]
			delete(parts, fid)
			_, err := a.archive.Append(req.SessionID, storage.EventID{}, acp.NewSessionUpdateAgentMessageChunk(
				acp.NewContentBlockText(p.buf.String()), p.id))
			return err
		},
		OnReasoningStart: func(fid string, _ fantasy.ReasoningContent) error {
			parts[fid] = &partState{id: predictID()}
			return nil
		},
		OnReasoningDelta: func(fid, delta string) error {
			return stream.SendThought(ctx, delta, acp.WithMessageID(parts[fid].id))
		},
		OnReasoningEnd: func(fid string, r fantasy.ReasoningContent) error {
			p := parts[fid]
			delete(parts, fid)
			_, err := a.archive.Append(req.SessionID, storage.EventID{}, acp.NewSessionUpdateAgentThoughtChunk(
				acp.NewContentBlockText(r.Text), p.id))
			return err
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			kind := tools.ToolKinds[tc.ToolName]
			status := acp.ToolCallStatusInProgress
			return stream.SendUpdate(ctx, acp.NewSessionUpdateToolCall(acp.ToolCall{
				ToolCallID: acp.ToolCallID(tc.ToolCallID),
				Title:      tc.ToolName,
				Kind:       &kind,
				Status:     &status,
				RawInput:   json.RawMessage(tc.Input),
			}))
		},
		OnToolResult: func(res fantasy.ToolResultContent) error {
			status := acp.ToolCallStatusCompleted
			var text string
			switch o := res.Result.(type) {
			case fantasy.ToolResultOutputContentText:
				text = o.Text
			case fantasy.ToolResultOutputContentError:
				status = acp.ToolCallStatusFailed
				if o.Error != nil {
					text = o.Error.Error()
				}
			}
			output, _ := json.Marshal(text)
			return stream.SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(res.ToolCallID),
				Status:     &status,
				RawOutput:  output,
			}))
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			return &acp.PromptResponse{StopReason: acp.StopReasonCancelled, UserMessageID: userMsgID}, nil
		}
		return nil, acp.ErrInternalError(nil, err.Error())
	}

	for _, step := range result.Steps {
		sess.History = append(sess.History, step.Messages...)
	}

	stream.SendSessionInfo(ctx, sess.Title, sess.UpdatedAt)

	if opts.ContextWindow > 0 {
		stream.SendUpdate(ctx, acp.NewSessionUpdateUsageUpdate(acp.UsageUpdate{
			Used: result.TotalUsage.InputTokens + result.TotalUsage.OutputTokens,
			Size: opts.ContextWindow,
		}))
	}

	stopReason := acp.StopReasonEndTurn
	if result.Response.FinishReason == fantasy.FinishReasonLength {
		stopReason = acp.StopReasonMaxTokens
	}
	u := result.TotalUsage
	return &acp.PromptResponse{
		StopReason: stopReason,
		Usage: &acp.Usage{
			InputTokens:       u.InputTokens,
			OutputTokens:      u.OutputTokens,
			TotalTokens:       u.TotalTokens,
			ThoughtTokens:     &u.ReasoningTokens,
			CachedReadTokens:  &u.CacheReadTokens,
			CachedWriteTokens: &u.CacheCreationTokens,
		},
		UserMessageID: userMsgID,
	}, nil
}

func titleFromMessage(msg fantasy.Message) string {
	for _, part := range msg.Content {
		if tp, ok := part.(fantasy.TextPart); ok && tp.Text != "" {
			t := tp.Text
			if i := strings.IndexByte(t, '\n'); i >= 0 {
				t = t[:i]
			}
			if len(t) > 80 {
				t = t[:80] + "…"
			}
			return t
		}
	}
	return ""
}
