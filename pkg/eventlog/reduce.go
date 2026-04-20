package eventlog

import (
	"encoding/json"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	acp "github.com/ironpark/go-acp"
)

// Reduce replays events to produce a complete Session.
//
// The log contains coalesced events (not streaming deltas): each
// AgentMessageChunk, AgentThoughtChunk, and ToolCall is a complete unit.
// The reducer only needs to group assistant parts into messages and flush
// on role boundaries.
func Reduce(events []Event) *session.Session {
	r := &reducer{}
	for i := range events {
		r.apply(&events[i])
	}
	r.flush()
	return &r.sess
}

// reducer accumulates message parts until a role boundary forces a flush.
type reducer struct {
	sess  session.Session
	role  fantasy.MessageRole
	parts []fantasy.MessagePart
}

// flush emits the accumulated parts as a message and resets the buffer.
func (r *reducer) flush() {
	if len(r.parts) == 0 {
		return
	}
	r.sess.History = append(r.sess.History, fantasy.Message{
		Role:    r.role,
		Content: r.parts,
	})
	r.parts = nil
	r.role = ""
}

// start resets the buffer to begin accumulating parts for the given role.
func (r *reducer) start(role fantasy.MessageRole) {
	if r.role != role {
		r.flush()
		r.role = role
	}
}

func (r *reducer) apply(ev *Event) {
	if ev.Info != nil {
		r.sess.Cwd = ev.Info.Cwd
		if ev.Info.Title != "" {
			r.sess.Title = ev.Info.Title
		}
		if ev.Info.UpdatedAt != "" {
			r.sess.UpdatedAt = ev.Info.UpdatedAt
		}
		return
	}
	if ev.Update == nil {
		return
	}
	acp.MatchSessionUpdate(ev.Update, acp.SessionUpdateMatcher[any]{
		UserMessageChunk: func(c acp.SessionUpdateUserMessageChunk) any {
			r.start(fantasy.MessageRoleUser)
			appendContent(&r.parts, c.Content)
			return nil
		},
		AgentMessageChunk: func(c acp.SessionUpdateAgentMessageChunk) any {
			r.start(fantasy.MessageRoleAssistant)
			appendContent(&r.parts, c.Content)
			return nil
		},
		AgentThoughtChunk: func(c acp.SessionUpdateAgentThoughtChunk) any {
			r.start(fantasy.MessageRoleAssistant)
			if t, ok := c.Content.AsText(); ok {
				r.parts = append(r.parts, fantasy.ReasoningPart{Text: t.Text})
			}
			return nil
		},
		ToolCall: func(tc acp.SessionUpdateToolCall) any {
			r.start(fantasy.MessageRoleAssistant)
			r.parts = append(r.parts, fantasy.ToolCallPart{
				ToolCallID: string(tc.ToolCallID),
				ToolName:   tc.Title,
				Input:      string(tc.RawInput),
			})
			return nil
		},
		ToolCallUpdate: func(tcu acp.SessionUpdateToolCallUpdate) any {
			if tcu.Status == nil {
				return nil
			}
			status := *tcu.Status
			if status != acp.ToolCallStatusCompleted && status != acp.ToolCallStatusFailed {
				return nil
			}
			r.flush()
			var output string
			if len(tcu.RawOutput) > 0 {
				json.Unmarshal(tcu.RawOutput, &output)
			}
			r.sess.History = append(r.sess.History, fantasy.Message{
				Role: fantasy.MessageRoleTool,
				Content: []fantasy.MessagePart{fantasy.ToolResultPart{
					ToolCallID: string(tcu.ToolCallID),
					Output:     fantasy.ToolResultOutputContentText{Text: output},
				}},
			})
			return nil
		},
		SessionInfoUpdate: func(i acp.SessionUpdateSessionInfoUpdate) any {
			if i.Title != "" {
				r.sess.Title = i.Title
			}
			if i.UpdatedAt != "" {
				r.sess.UpdatedAt = i.UpdatedAt
			}
			return nil
		},
		ConfigOptionUpdate: func(c acp.SessionUpdateConfigOptionUpdate) any {
			applyConfigOptions(&r.sess, c.ConfigOptions)
			return nil
		},
		UsageUpdate: func(u acp.SessionUpdateUsageUpdate) any {
			r.sess.UsageUsed = u.Used
			r.sess.UsageSize = u.Size
			return nil
		},
		Default: func() any { return nil },
	})
}

// appendContent adds a ContentBlock to parts, mapping to the appropriate
// fantasy part type.
func appendContent(parts *[]fantasy.MessagePart, content acp.ContentBlock) {
	acp.MatchContentBlock(&content, acp.ContentBlockMatcher[any]{
		Text: func(t acp.ContentBlockText) any {
			*parts = append(*parts, fantasy.TextPart{Text: t.Text})
			return nil
		},
		Image: func(i acp.ContentBlockImage) any {
			*parts = append(*parts, fantasy.FilePart{
				Data:      []byte(i.Data),
				MediaType: i.MimeType,
			})
			return nil
		},
		Audio: func(a acp.ContentBlockAudio) any {
			*parts = append(*parts, fantasy.FilePart{
				Data:      []byte(a.Data),
				MediaType: a.MimeType,
			})
			return nil
		},
		Default: func() any { return nil },
	})
}

// applyConfigOptions merges select-typed config options into the session.
func applyConfigOptions(s *session.Session, opts []acp.SessionConfigOption) {
	for _, opt := range opts {
		sel, ok := opt.AsSelect()
		if !ok {
			continue
		}
		switch sel.ID {
		case llm.SessionConfigModel:
			s.Model = string(sel.CurrentValue)
		case llm.SessionConfigThoughtLevel:
			s.ThoughtLevel = string(sel.CurrentValue)
		}
	}
}
