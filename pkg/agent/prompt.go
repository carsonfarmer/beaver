package agent

import (
	"context"
	"encoding/json"
	"strings"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/eventlog"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/tools"
	"github.com/google/uuid"

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

	if sess.SystemPrompt == "" {
		sess.SystemPrompt = a.instructions.Discover(ctx, sess.Cwd, a.client, req.SessionID)
	}

	log, err := a.store.Open(req.SessionID)
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}

	userMsgID := req.MessageID
	if userMsgID == "" {
		userMsgID = uuid.New().String()
	}

	// Optional rewind: parent_message_id identifies a prior message to
	// branch from. Translate message_id → event_id (linear scan) so the
	// first event of this turn links to the rewind target, not the tip.
	var rewindParent uuid.UUID
	if s, ok := req.Meta["parent_message_id"].(string); ok && s != "" {
		events, err := log.Read(ctx, uuid.Nil)
		if err != nil {
			return nil, acp.ErrInternalError(nil, err.Error())
		}
		p, ok := eventlog.FindByMessageID(events, s)
		if !ok {
			return nil, acp.ErrInvalidParams(nil, "parent_message_id not found")
		}
		rewindParent = p
	}

	ctx = tools.WithSessionID(ctx, req.SessionID)
	ctx = tools.WithCwd(ctx, sess.Cwd)

	client := &eventlog.LoggingClient{Client: a.client, Store: a.store}
	stream := acp.NewSessionStream(client, req.SessionID)
	if msg, ok := llm.ContentBlocksToMessage(req.Prompt); ok {
		sess.History = append(sess.History, msg)
		if sess.Title == "" {
			sess.Title = titleFromMessage(msg)
		}
		// Log user message directly. Stamp rewind target on the first
		// chunk only; subsequent chunks chain off it via lastID.
		for _, part := range msg.Content {
			tp, ok := part.(fantasy.TextPart)
			if !ok {
				continue
			}
			upd := acp.NewSessionUpdateUserMessageChunk(
				acp.NewContentBlockText(tp.Text), userMsgID)
			if rewindParent != uuid.Nil {
				upd = eventlog.WithParentEventID(upd, rewindParent)
				rewindParent = uuid.Nil
			}
			log.Append(ctx, upd)
		}
	}

	allTools := []fantasy.AgentTool{
		tools.NewReadFileTool(client),
		tools.NewWriteFileTool(client),
		tools.NewExecuteTool(client),
		tools.NewPlanTool(client),
	}

	fa := fantasy.NewAgent(model,
		fantasy.WithTools(allTools...),
		fantasy.WithSystemPrompt(sess.SystemPrompt),
	)

	// Buffer text deltas per fantasy content-part ID; coalesced on OnTextEnd.
	textBufs := map[string]*strings.Builder{}

	result, err := fa.Stream(ctx, fantasy.AgentStreamCall{
		Messages:        sess.History,
		MaxOutputTokens: opts.MaxOutputTokens,
		ProviderOptions: opts.ProviderOptions,
		OnTextStart: func(id string) error {
			textBufs[id] = &strings.Builder{}
			return nil
		},
		OnTextDelta: func(id, delta string) error {
			textBufs[id].WriteString(delta)
			return stream.SendText(ctx, delta, acp.WithMessageID(id))
		},
		OnTextEnd: func(id string) error {
			buf := textBufs[id]
			delete(textBufs, id)
			return log.Append(ctx, acp.NewSessionUpdateAgentMessageChunk(
				acp.NewContentBlockText(buf.String()), id))
		},
		OnReasoningDelta: func(id, delta string) error {
			return stream.SendThought(ctx, delta, acp.WithMessageID(id))
		},
		OnReasoningEnd: func(id string, r fantasy.ReasoningContent) error {
			return log.Append(ctx, acp.NewSessionUpdateAgentThoughtChunk(
				acp.NewContentBlockText(r.Text), id))
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
