// Package client implements an ACP client that handles filesystem, terminal,
// and permission operations locally, suitable for use as a standalone CLI client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	acp "github.com/ironpark/go-acp"
)

var terminalCounter atomic.Int64

// OutputMode controls how session updates are rendered.
type OutputMode int

const (
	OutputText  OutputMode = iota // Full text: thoughts, tool calls, agent text
	OutputJSON                    // JSON lines: every session update as JSON
	OutputQuiet                   // Quiet: agent text only
	OutputTUI                     // TUI: forward to callback for bubbletea rendering
)

// PermissionMode controls how permission requests are handled.
type PermissionMode int

const (
	PermissionApproveAll   PermissionMode = iota // Auto-approve everything
	PermissionApproveReads                       // Auto-approve reads, deny writes/executes
	PermissionDenyAll                            // Deny everything
)

// UpdateFunc is a callback invoked for each session update in TUI mode.
type UpdateFunc func(*acp.SessionNotification)

// Client implements the acp.Client interface with real filesystem and
// terminal operations.
type Client struct {
	outputMode     OutputMode
	permissionMode PermissionMode
	cwd            string
	onUpdate       UpdateFunc
	terminals      sync.Map // map[string]*terminal
}

type terminal struct {
	cmd      *exec.Cmd
	output   bytes.Buffer
	mu       sync.Mutex
	done     chan struct{}
	exitCode int
}

// New creates a new Client rooted at the given working directory.
func New(cwd string, output OutputMode, perms PermissionMode) *Client {
	return &Client{cwd: cwd, outputMode: output, permissionMode: perms}
}

// SetOnUpdate sets the callback for TUI mode session updates.
func (c *Client) SetOnUpdate(fn UpdateFunc) {
	c.onUpdate = fn
}

// --- Session updates ---

func (c *Client) SessionUpdate(_ context.Context, params *acp.SessionNotification) error {
	switch c.outputMode {
	case OutputJSON:
		b, _ := json.Marshal(params)
		fmt.Fprintln(os.Stdout, string(b))
	case OutputQuiet:
		c.renderQuiet(params)
	case OutputTUI:
		if c.onUpdate != nil {
			c.onUpdate(params)
		}
	default:
		c.renderText(params)
	}
	return nil
}

func (c *Client) renderText(params *acp.SessionNotification) {
	acp.MatchSessionUpdate(&params.Update, acp.SessionUpdateMatcher[struct{}]{
		AgentMessageChunk: func(v acp.SessionUpdateAgentMessageChunk) struct{} {
			if text, ok := v.Content.AsText(); ok {
				fmt.Print(text.Text)
			}
			return struct{}{}
		},
		AgentThoughtChunk: func(v acp.SessionUpdateAgentThoughtChunk) struct{} {
			if text, ok := v.Content.AsText(); ok {
				fmt.Fprintf(os.Stderr, "\033[2m%s\033[0m", text.Text)
			}
			return struct{}{}
		},
		ToolCall: func(v acp.SessionUpdateToolCall) struct{} {
			fmt.Fprintf(os.Stderr, "\n\033[33m> %s\033[0m\n", v.Title)
			return struct{}{}
		},
		ToolCallUpdate: func(v acp.SessionUpdateToolCallUpdate) struct{} {
			return struct{}{}
		},
		Default: func() struct{} { return struct{}{} },
	})
}

func (c *Client) renderQuiet(params *acp.SessionNotification) {
	acp.MatchSessionUpdate(&params.Update, acp.SessionUpdateMatcher[struct{}]{
		AgentMessageChunk: func(v acp.SessionUpdateAgentMessageChunk) struct{} {
			if text, ok := v.Content.AsText(); ok {
				fmt.Print(text.Text)
			}
			return struct{}{}
		},
		Default: func() struct{} { return struct{}{} },
	})
}

// --- Permissions ---

func (c *Client) RequestPermission(_ context.Context, params *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	switch c.permissionMode {
	case PermissionApproveAll:
		return c.selectAllow(params)
	case PermissionDenyAll:
		return c.selectDeny(params)
	case PermissionApproveReads:
		if isReadPermission(params) {
			return c.selectAllow(params)
		}
		return c.selectDeny(params)
	}
	return c.selectAllow(params)
}

// SetPermissionMode updates the permission mode.
func (c *Client) SetPermissionMode(mode PermissionMode) {
	c.permissionMode = mode
}

func (c *Client) selectAllow(params *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	for _, opt := range params.Options {
		if opt.Kind == acp.PermissionOptionKindAllowOnce || opt.Kind == acp.PermissionOptionKindAllowAlways {
			return &acp.RequestPermissionResponse{
				Outcome: acp.NewRequestPermissionOutcomeSelected(opt.OptionID),
			}, nil
		}
	}
	return &acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeCancelled(),
	}, nil
}

func (c *Client) selectDeny(params *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	for _, opt := range params.Options {
		if opt.Kind == acp.PermissionOptionKindRejectOnce || opt.Kind == acp.PermissionOptionKindRejectAlways {
			return &acp.RequestPermissionResponse{
				Outcome: acp.NewRequestPermissionOutcomeSelected(opt.OptionID),
			}, nil
		}
	}
	return &acp.RequestPermissionResponse{
		Outcome: acp.NewRequestPermissionOutcomeCancelled(),
	}, nil
}

func isReadPermission(params *acp.RequestPermissionRequest) bool {
	if params.ToolCall.Kind != nil {
		switch *params.ToolCall.Kind {
		case acp.ToolKindRead, acp.ToolKindSearch:
			return true
		}
	}
	return false
}

// --- Filesystem ---

func (c *Client) ReadTextFile(_ context.Context, params *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	path := params.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.cwd, path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return &acp.ReadTextFileResponse{Content: string(data)}, nil
}

func (c *Client) WriteTextFile(_ context.Context, params *acp.WriteTextFileRequest) (*acp.WriteTextFileResponse, error) {
	path := params.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(c.cwd, path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(params.Content), 0o644); err != nil {
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	return &acp.WriteTextFileResponse{}, nil
}

// --- Terminal ---

func (c *Client) CreateTerminal(_ context.Context, params *acp.CreateTerminalRequest) (*acp.CreateTerminalResponse, error) {
	id := fmt.Sprintf("term_%d", terminalCounter.Add(1))

	cmd := exec.Command(params.Command, params.Args...)
	cmd.Dir = c.cwd

	t := &terminal{cmd: cmd, done: make(chan struct{})}
	cmd.Stdout = &lockedWriter{buf: &t.output, mu: &t.mu}
	cmd.Stderr = &lockedWriter{buf: &t.output, mu: &t.mu}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("exec %s: %w", params.Command, err)
	}

	go func() {
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				t.exitCode = exitErr.ExitCode()
			} else {
				t.exitCode = 1
			}
		}
		close(t.done)
	}()

	c.terminals.Store(id, t)
	return &acp.CreateTerminalResponse{TerminalID: id}, nil
}

func (c *Client) TerminalOutput(_ context.Context, params *acp.TerminalOutputRequest) (*acp.TerminalOutputResponse, error) {
	v, ok := c.terminals.Load(params.TerminalID)
	if !ok {
		return nil, fmt.Errorf("terminal %s not found", params.TerminalID)
	}
	t := v.(*terminal)
	t.mu.Lock()
	out := t.output.String()
	t.mu.Unlock()
	return &acp.TerminalOutputResponse{Output: out}, nil
}

func (c *Client) WaitForTerminalExit(ctx context.Context, params *acp.WaitForTerminalExitRequest) (*acp.WaitForTerminalExitResponse, error) {
	v, ok := c.terminals.Load(params.TerminalID)
	if !ok {
		return nil, fmt.Errorf("terminal %s not found", params.TerminalID)
	}
	t := v.(*terminal)
	select {
	case <-t.done:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	exitCode := int64(t.exitCode)
	return &acp.WaitForTerminalExitResponse{ExitCode: &exitCode}, nil
}

func (c *Client) KillTerminalCommand(_ context.Context, params *acp.KillTerminalRequest) (*acp.KillTerminalResponse, error) {
	v, ok := c.terminals.Load(params.TerminalID)
	if !ok {
		return nil, fmt.Errorf("terminal %s not found", params.TerminalID)
	}
	t := v.(*terminal)
	if t.cmd.Process != nil {
		_ = t.cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-t.done:
		case <-time.After(1500 * time.Millisecond):
			_ = t.cmd.Process.Kill()
			<-t.done
		}
	}
	return &acp.KillTerminalResponse{}, nil
}

func (c *Client) ReleaseTerminal(_ context.Context, params *acp.ReleaseTerminalRequest) (*acp.ReleaseTerminalResponse, error) {
	if v, ok := c.terminals.LoadAndDelete(params.TerminalID); ok {
		t := v.(*terminal)
		// Kill the process if it's still running.
		select {
		case <-t.done:
		default:
			if t.cmd.Process != nil {
				_ = t.cmd.Process.Kill()
			}
		}
	}
	return &acp.ReleaseTerminalResponse{}, nil
}

// lockedWriter is an io.Writer that appends to a bytes.Buffer under a mutex.
type lockedWriter struct {
	buf *bytes.Buffer
	mu  *sync.Mutex
}

func (w *lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(p)
}
