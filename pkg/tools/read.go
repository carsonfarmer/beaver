package tools

import (
	"context"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

const (
	ReadName        = "read_file"
	ReadDescription = "Read a text file from the filesystem"
)

type ReadInput struct {
	Path  string `json:"path" description:"Absolute file path to read"`
	Limit *int64 `json:"limit,omitempty" description:"Maximum number of lines to return"`
	Line  *int64 `json:"line,omitempty" description:"Start reading from this line number (1-based)"`
}

func NewReadFileTool(client acp.Client) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(ReadName, ReadDescription,
		func(ctx context.Context, in ReadInput, tc fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := SessionIDFrom(ctx)
			acp.NewSessionStream(client, sessionID).SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(tc.ID),
				Title:      ReadName + " " + RelPath(ctx, in.Path),
				Locations:  []acp.ToolCallLocation{{Path: in.Path, Line: in.Line}},
			}))
			resp, err := client.ReadTextFile(ctx, &acp.ReadTextFileRequest{
				Path:      in.Path,
				Limit:     in.Limit,
				Line:      in.Line,
				SessionID: sessionID,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(resp.Content), nil
		})
}
