package session

import (
	"encoding/json"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/storage"
	acp "github.com/ironpark/go-acp"
)

// Project replays an event chain to produce the session's runtime state.
//
// The chain is assumed to be in chronological order (root first, tip last)
// — typically the output of [storage.Lineage]. Events in the chain are the
// coalesced form: each AgentMessageChunk, AgentThoughtChunk, and ToolCall
// is already a complete unit, so the projection only groups assistant
// parts into messages and flushes on role boundaries.
func Project(events []storage.Event) *State {
	p := &projector{}
	for i := range events {
		p.apply(&events[i])
	}
	p.flush()
	return &p.state
}

type projector struct {
	state State
	role  fantasy.MessageRole
	parts []fantasy.MessagePart
}

func (p *projector) flush() {
	if len(p.parts) == 0 {
		return
	}
	p.state.History = append(p.state.History, fantasy.Message{
		Role:    p.role,
		Content: p.parts,
	})
	p.parts = nil
	p.role = ""
}

func (p *projector) start(role fantasy.MessageRole) {
	if p.role != role {
		p.flush()
		p.role = role
	}
}

func (p *projector) apply(ev *storage.Event) {
	if ev.Info != nil {
		if ev.Info.Cwd != "" {
			p.state.Cwd = ev.Info.Cwd
		}
		if ev.Info.Title != "" {
			p.state.Title = ev.Info.Title
		}
		if ev.Info.UpdatedAt != "" {
			p.state.UpdatedAt = ev.Info.UpdatedAt
		}
		return
	}
	if ev.Update == nil {
		return
	}
	acp.MatchSessionUpdate(ev.Update, acp.SessionUpdateMatcher[any]{
		UserMessageChunk: func(c acp.SessionUpdateUserMessageChunk) any {
			p.start(fantasy.MessageRoleUser)
			appendContent(&p.parts, c.Content)
			return nil
		},
		AgentMessageChunk: func(c acp.SessionUpdateAgentMessageChunk) any {
			p.start(fantasy.MessageRoleAssistant)
			appendContent(&p.parts, c.Content)
			return nil
		},
		AgentThoughtChunk: func(c acp.SessionUpdateAgentThoughtChunk) any {
			p.start(fantasy.MessageRoleAssistant)
			if t, ok := c.Content.AsText(); ok {
				p.parts = append(p.parts, fantasy.ReasoningPart{Text: t.Text})
			}
			return nil
		},
		ToolCall: func(tc acp.SessionUpdateToolCall) any {
			p.start(fantasy.MessageRoleAssistant)
			p.parts = append(p.parts, fantasy.ToolCallPart{
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
			p.flush()
			var output string
			if len(tcu.RawOutput) > 0 {
				json.Unmarshal(tcu.RawOutput, &output)
			}
			p.state.History = append(p.state.History, fantasy.Message{
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
				p.state.Title = i.Title
			}
			if i.UpdatedAt != "" {
				p.state.UpdatedAt = i.UpdatedAt
			}
			return nil
		},
		ConfigOptionUpdate: func(c acp.SessionUpdateConfigOptionUpdate) any {
			applyConfigOptions(&p.state, c.ConfigOptions)
			return nil
		},
		UsageUpdate: func(u acp.SessionUpdateUsageUpdate) any {
			p.state.UsageUsed = u.Used
			p.state.UsageSize = u.Size
			return nil
		},
		Default: func() any { return nil },
	})
}

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

func applyConfigOptions(s *State, opts []acp.SessionConfigOption) {
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
