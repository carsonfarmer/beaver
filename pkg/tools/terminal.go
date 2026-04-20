package tools

import (
	"context"
	"strings"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

const (
	ExecuteName        = "execute"
	ExecuteDescription = "Execute a terminal command and return its output"
)

type ExecInput struct {
	Command string   `json:"command" description:"Command to execute"`
	Args    []string `json:"args,omitempty" description:"Command arguments"`
}

func NewExecuteTool(client acp.Client) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(ExecuteName, ExecuteDescription,
		func(ctx context.Context, in ExecInput, tc fantasy.ToolCall) (fantasy.ToolResponse, error) {
			sessionID := SessionIDFrom(ctx)
			resp, err := client.CreateTerminal(ctx, &acp.CreateTerminalRequest{
				Command: in.Command, Args: in.Args, SessionID: sessionID,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			handle := acp.NewTerminalHandle(resp.TerminalID, sessionID, client)
			defer handle.Release(ctx)

			acp.NewSessionStream(client, sessionID).SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(tc.ID),
				Title:      strings.TrimSpace(in.Command + " " + strings.Join(in.Args, " ")),
				Content: []acp.ToolCallContent{
					acp.NewToolCallContentTerminal(resp.TerminalID),
				},
			}))

			handle.WaitForExit(ctx)
			out, err := handle.CurrentOutput(ctx)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			return fantasy.NewTextResponse(out.Output), nil
		})
}

// RunCommand creates a terminal, waits for it to exit, and returns its output.
func RunCommand(ctx context.Context, client acp.Client, sessionID acp.SessionID, command string, args []string) (string, error) {
	resp, err := client.CreateTerminal(ctx, &acp.CreateTerminalRequest{
		Command: command, Args: args, SessionID: sessionID,
	})
	if err != nil {
		return "", err
	}
	handle := acp.NewTerminalHandle(resp.TerminalID, sessionID, client)
	defer handle.Release(ctx)
	handle.WaitForExit(ctx)
	out, err := handle.CurrentOutput(ctx)
	if err != nil {
		return "", err
	}
	return out.Output, nil
}
