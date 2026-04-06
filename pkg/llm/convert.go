package llm

import (
	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

// ContentBlockToPart converts an ACP ContentBlock to a fantasy MessagePart.
func ContentBlockToPart(block acp.ContentBlock) fantasy.MessagePart {
	return acp.MatchContentBlock(&block, acp.ContentBlockMatcher[fantasy.MessagePart]{
		Text: func(t acp.ContentBlockText) fantasy.MessagePart { return fantasy.TextPart{Text: t.Text} },
		Image: func(img acp.ContentBlockImage) fantasy.MessagePart {
			return fantasy.FilePart{Data: []byte(img.Data), MediaType: img.MimeType}
		},
		Default: func() fantasy.MessagePart { return fantasy.TextPart{} },
	})
}

// ContentBlocksToMessage converts ACP ContentBlocks to a fantasy user Message.
func ContentBlocksToMessage(blocks []acp.ContentBlock) (fantasy.Message, bool) {
	if len(blocks) == 0 {
		return fantasy.Message{}, false
	}
	parts := make([]fantasy.MessagePart, len(blocks))
	for i, block := range blocks {
		parts[i] = ContentBlockToPart(block)
	}
	return fantasy.Message{Role: fantasy.MessageRoleUser, Content: parts}, true
}

// UpdateToMessage converts an ACP SessionUpdate to a fantasy Message.
// Returns ok=false for update types that don't map to conversation history.
func UpdateToMessage(update *acp.SessionUpdate) (fantasy.Message, bool) {
	msg := acp.MatchSessionUpdate(update, acp.SessionUpdateMatcher[*fantasy.Message]{
		AgentMessageChunk: func(c acp.SessionUpdateAgentMessageChunk) *fantasy.Message {
			return &fantasy.Message{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{ContentBlockToPart(c.Content)}}
		},
		AgentThoughtChunk: func(c acp.SessionUpdateAgentThoughtChunk) *fantasy.Message {
			if t, ok := c.Content.AsText(); ok {
				return &fantasy.Message{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ReasoningPart{Text: t.Text}}}
			}
			return nil
		},
		ToolCall: func(tc acp.SessionUpdateToolCall) *fantasy.Message {
			return &fantasy.Message{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{
				ToolCallID: string(tc.ToolCallID), ToolName: tc.Title,
			}}}
		},
		ToolCallUpdate: func(tc acp.SessionUpdateToolCallUpdate) *fantasy.Message {
			var text string
			if len(tc.Content) > 0 {
				if c, ok := tc.Content[0].AsContent(); ok {
					if t, ok := c.Content.Content.AsText(); ok {
						text = t.Text
					}
				}
			}
			return &fantasy.Message{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{fantasy.ToolResultPart{
				ToolCallID: string(tc.ToolCallID), Output: fantasy.ToolResultOutputContentText{Text: text},
			}}}
		},
		Default: func() *fantasy.Message { return nil },
	})
	if msg == nil {
		return fantasy.Message{}, false
	}
	return *msg, true
}

// UsageToUsage converts a fantasy Usage to an ACP Usage.
func UsageToUsage(u fantasy.Usage) acp.Usage {
	return acp.Usage{
		InputTokens:       u.InputTokens,
		OutputTokens:      u.OutputTokens,
		TotalTokens:       u.TotalTokens,
		ThoughtTokens:     ptr(u.ReasoningTokens),
		CachedReadTokens:  ptr(u.CacheReadTokens),
		CachedWriteTokens: ptr(u.CacheCreationTokens),
	}
}

func ptr[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}
