package tools

import (
	"context"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

const (
	WriteName        = "write_file"
	WriteDescription = "Write content to a text file on the filesystem"
)

type WriteInput struct {
	Path    string `json:"path" description:"Absolute file path to write"`
	Content string `json:"content" description:"File content to write"`
}

func NewWriteFileTool(client acp.Client) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(WriteName, WriteDescription,
		func(ctx context.Context, in WriteInput, tc fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := SessionIDFrom(ctx)
			stream := acp.NewSessionStream(client, sessionID)
			stream.SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(tc.ID),
				Title:      WriteName + " " + RelPath(ctx, in.Path),
				Locations:  []acp.ToolCallLocation{{Path: in.Path}},
			}))
			var oldContent string
			if resp, err := client.ReadTextFile(ctx, &acp.ReadTextFileRequest{
				Path: in.Path, SessionID: sessionID,
			}); err == nil {
				oldContent = resp.Content
			}
			if _, err := client.WriteTextFile(ctx, &acp.WriteTextFileRequest{
				Path: in.Path, Content: in.Content, SessionID: sessionID,
			}); err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			stream.SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(tc.ID),
				Content: []acp.ToolCallContent{
					acp.NewToolCallContentDiff(in.Path, in.Content, oldContent),
				},
			}))
			return fantasy.NewTextResponse("file written successfully"), nil
		})
}
