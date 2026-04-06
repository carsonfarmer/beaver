// Package tools defines the Fantasy agent tools that proxy to ACP client methods.
package tools

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

type pathInput struct {
	Path string `json:"path" description:"Absolute file path to read"`
}

type writeInput struct {
	Path    string `json:"path" description:"Absolute file path to write"`
	Content string `json:"content" description:"File content to write"`
}

type execInput struct {
	Command string   `json:"command" description:"Command to execute"`
	Args    []string `json:"args,omitempty" description:"Command arguments"`
}

// RunCommand executes a terminal command via the ACP client and returns its output.
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

// ForSession returns the set of agent tools bound to the given client and session.
func ForSession(client acp.Client, sessionID acp.SessionID) []fantasy.AgentTool {
	return []fantasy.AgentTool{
		fantasy.NewAgentTool("read_file", "Read a text file from the filesystem",
			func(ctx context.Context, in pathInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				resp, err := client.ReadTextFile(ctx, &acp.ReadTextFileRequest{Path: in.Path, SessionID: sessionID})
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.NewTextResponse(resp.Content), nil
			}),
		fantasy.NewAgentTool("write_file", "Write content to a text file on the filesystem",
			func(ctx context.Context, in writeInput, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
				// Read existing content before writing so we can show a proper diff.
				var oldContent string
				if resp, err := client.ReadTextFile(ctx, &acp.ReadTextFileRequest{Path: in.Path, SessionID: sessionID}); err == nil {
					oldContent = resp.Content
				}
				_, err := client.WriteTextFile(ctx, &acp.WriteTextFileRequest{Path: in.Path, Content: in.Content, SessionID: sessionID})
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse("file written successfully"),
					struct{ OldContent string `json:"oldContent"` }{oldContent},
				), nil
			}),
		fantasy.NewAgentTool("execute", "Execute a terminal command and return its output",
			func(ctx context.Context, in execInput, tc fantasy.ToolCall) (fantasy.ToolResponse, error) {
				resp, err := client.CreateTerminal(ctx, &acp.CreateTerminalRequest{
					Command: in.Command, Args: in.Args, SessionID: sessionID,
				})
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				handle := acp.NewTerminalHandle(resp.TerminalID, sessionID, client)
				defer handle.Release(ctx)

				// Embed terminal in the in-progress tool call so the client
				// displays live output as the command runs.
				client.SessionUpdate(ctx, &acp.SessionNotification{
					SessionID: sessionID,
					Update: acp.NewSessionUpdateToolCallUpdate(acp.ToolCallUpdate{
						ToolCallID: acp.ToolCallID(tc.ID),
						Content: []acp.ToolCallContent{
							acp.NewToolCallContentTerminal(resp.TerminalID),
						},
					}),
				})

				handle.WaitForExit(ctx)
				out, err := handle.CurrentOutput(ctx)
				if err != nil {
					return fantasy.NewTextErrorResponse(err.Error()), nil
				}
				return fantasy.WithResponseMetadata(
					fantasy.NewTextResponse(out.Output),
					struct {
						TerminalID string `json:"terminalId"`
					}{resp.TerminalID},
				), nil
			}),
	}
}

// Kind returns the ACP ToolKind for a given tool name.
func Kind(name string) acp.ToolKind {
	switch name {
	case "read_file":
		return acp.ToolKindRead
	case "write_file":
		return acp.ToolKindEdit
	case "execute":
		return acp.ToolKindExecute
	default:
		return acp.ToolKindOther
	}
}

// Title returns a display title for a tool call, incorporating relevant
// input like file paths (e.g. "read_file src/foo.txt").
func Title(toolName, inputJSON string, cwd ...string) string {
	var parsed struct {
		Path    string   `json:"path"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if json.Unmarshal([]byte(inputJSON), &parsed) == nil {
		switch toolName {
		case "read_file", "write_file":
			if parsed.Path != "" {
				return toolName + " " + relativePath(parsed.Path, cwd...)
			}
		case "execute":
			if parsed.Command != "" {
				if len(parsed.Args) > 0 {
					return parsed.Command + " " + strings.Join(parsed.Args, " ")
				}
				return parsed.Command
			}
		}
	}
	return toolName
}

// Locations extracts file locations from a tool call's input JSON
// so the client UI can follow along (e.g. highlight the file being read).
func Locations(toolName, inputJSON string) []acp.ToolCallLocation {
	var parsed struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal([]byte(inputJSON), &parsed); err != nil || parsed.Path == "" {
		return nil
	}
	switch toolName {
	case "read_file", "write_file":
		return []acp.ToolCallLocation{{Path: parsed.Path}}
	}
	return nil
}

// ResultContent builds the appropriate ToolCallContent slice for a completed
// tool call, using the original input, the tool's result, and any client metadata.
func ResultContent(toolName string, input json.RawMessage, result fantasy.ToolResultOutputContent, metadata string) []acp.ToolCallContent {
	if IsError(result) {
		return textContent(TextResult(result))
	}

	switch toolName {
	case "write_file":
		var parsed struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(input, &parsed); err == nil && parsed.Path != "" {
			var meta struct {
				OldContent string `json:"oldContent"`
			}
			json.Unmarshal([]byte(metadata), &meta)
			return []acp.ToolCallContent{
				acp.NewToolCallContentDiff(parsed.Path, parsed.Content, meta.OldContent),
			}
		}
	case "execute":
		var meta struct {
			TerminalID string `json:"terminalId"`
		}
		if json.Unmarshal([]byte(metadata), &meta) == nil && meta.TerminalID != "" {
			// Include both the terminal reference (for live display) and
			// the text output (for session replay when the terminal is gone).
			return []acp.ToolCallContent{
				acp.NewToolCallContentTerminal(meta.TerminalID),
				acp.NewToolCallContentContent(acp.NewContentBlockText(TextResult(result))),
			}
		}
	}

	return textContent(TextResult(result))
}

func textContent(text string) []acp.ToolCallContent {
	return []acp.ToolCallContent{
		acp.NewToolCallContentContent(acp.NewContentBlockText(text)),
	}
}

// IsError reports whether a tool result represents an error.
func IsError(r fantasy.ToolResultOutputContent) bool {
	_, ok := r.(fantasy.ToolResultOutputContentError)
	return ok
}

// relativePath returns path relative to cwd if possible, otherwise the original path.
func relativePath(path string, cwd ...string) string {
	if len(cwd) > 0 && cwd[0] != "" {
		if rel, err := filepath.Rel(cwd[0], path); err == nil {
			return rel
		}
	}
	return path
}

// TextResult extracts the text string from a tool result.
func TextResult(r fantasy.ToolResultOutputContent) string {
	switch v := r.(type) {
	case fantasy.ToolResultOutputContentText:
		return v.Text
	case fantasy.ToolResultOutputContentError:
		return v.Error.Error()
	default:
		return ""
	}
}
