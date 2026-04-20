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

