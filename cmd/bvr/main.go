package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/carsonfarmer/beaver/cmd/bvr/tui"
	"github.com/carsonfarmer/beaver/pkg/client"
	tea "github.com/charmbracelet/bubbletea"
	acp "github.com/ironpark/go-acp"
)

// Global flags shared across subcommands.
var (
	httpAddr   string
	cwd        string
	sessionID  string
	format     string
	promptFile string
	approveAll bool
	denyAll    bool
	useTUI     bool
	timeout    float64
)

func main() {
	flag.StringVar(&httpAddr, "http", "", "HTTP address of the agent (e.g. http://localhost:8080)")
	flag.StringVar(&cwd, "cwd", "", "working directory (defaults to current)")
	flag.StringVar(&sessionID, "s", "", "session ID")
	flag.StringVar(&sessionID, "session", "", "session ID")
	flag.StringVar(&format, "format", "text", "output format: text, json, quiet")
	flag.StringVar(&promptFile, "f", "", "read prompt from file (use - for stdin)")
	flag.StringVar(&promptFile, "file", "", "read prompt from file (use - for stdin)")
	flag.BoolVar(&approveAll, "approve-all", false, "auto-approve all permission requests")
	flag.BoolVar(&denyAll, "deny-all", false, "deny all permission requests")
	flag.BoolVar(&useTUI, "tui", false, "use interactive TUI mode")
	flag.Float64Var(&timeout, "timeout", 0, "max wait time in seconds (0 = no timeout)")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, `Usage: bvr [flags] <agent> [command] [args...]

Standalone ACP client. Connects to an agent via stdio or HTTP.

In stdio mode, <agent> is the path to the agent binary.
In HTTP mode, use -http <url> and omit <agent>.

Commands:
  [prompt text]              Send a prompt (default command)
  sessions list              List all sessions
  sessions new               Create a new session
  set <key> <value>          Set a config option (e.g. model, thought_level, mode)
  resume [prompt text]       Resume a session and optionally send a prompt
  fork [prompt text]         Fork a session and optionally send a prompt

Prompt can be provided as args, via -f <file>, or piped via stdin.

Flags:
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	if cwd == "" {
		d, err := os.Getwd()
		if err != nil {
			fatal("failed to get working directory: %v", err)
		}
		cwd = d
	}

	args := flag.Args()

	// In stdio mode, first arg is the agent binary.
	var agentBinary string
	if httpAddr == "" {
		if len(args) < 1 {
			flag.Usage()
			os.Exit(2)
		}
		agentBinary = args[0]
		args = args[1:]
	}

	// Parse subcommand from remaining args.
	cmd, args := parseCommand(args)

	// Read prompt text from args/file/stdin before spawning the agent.
	// Once the agent subprocess is running, stdin detection is unreliable.
	var prompt string
	if cmd == "prompt" || cmd == "resume" || cmd == "fork" {
		prompt = readPrompt(args)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeout*float64(time.Second)))
		defer cancel()
	}

	outputMode := parseOutputMode()
	if useTUI {
		outputMode = client.OutputTUI
	}

	cl := client.New(cwd, outputMode, parsePermissionMode())
	conn := connect(ctx, cl, agentBinary)
	defer func() {
		// conn.Close can panic if the connection was never fully established.
		defer func() { recover() }()
		conn.Close()
	}()

	go func() {
		if err := conn.Start(ctx); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "connection error: %v\n", err)
		}
	}()

	initialize(ctx, conn)

	switch cmd {
	case "sessions":
		runSessions(ctx, conn, args)
	case "set":
		runSet(ctx, conn, args)
	case "resume":
		sid := requireSession("resume")
		if _, err := conn.ResumeSession(ctx, &acp.ResumeSessionRequest{
			SessionID: acp.SessionID(sid), Cwd: cwd,
		}); err != nil {
			fatal("resume session: %v", err)
		}
		if prompt == "" {
			fmt.Fprintln(os.Stderr, "resumed session "+sid)
		} else if useTUI {
			runPromptTUI(ctx, conn, cl, prompt)
		} else {
			sendPrompt(ctx, conn, acp.SessionID(sid), prompt)
		}
	case "fork":
		sid := requireSession("fork")
		resp, err := conn.ForkSession(ctx, &acp.ForkSessionRequest{
			SessionID: acp.SessionID(sid), Cwd: cwd,
		})
		if err != nil {
			fatal("fork session: %v", err)
		}
		sessionID = string(resp.SessionID) // update for sendPrompt/runPromptTUI
		fmt.Fprintf(os.Stderr, "forked to %s\n", resp.SessionID)
		if prompt == "" {
			fmt.Println(resp.SessionID)
		} else if useTUI {
			runPromptTUI(ctx, conn, cl, prompt)
		} else {
			sendPrompt(ctx, conn, resp.SessionID, prompt)
		}
	default:
		if useTUI {
			runPromptTUI(ctx, conn, cl, prompt)
		} else {
			runPrompt(ctx, conn, prompt)
		}
	}
}

func parseCommand(args []string) (string, []string) {
	if len(args) == 0 {
		return "prompt", nil
	}
	switch args[0] {
	case "sessions", "set", "resume", "fork":
		return args[0], args[1:]
	default:
		return "prompt", args
	}
}

// --- Subcommands ---

func runSessions(ctx context.Context, conn *acp.ClientSideConnection, args []string) {
	subcmd := "list"
	if len(args) > 0 {
		subcmd = args[0]
	}

	switch subcmd {
	case "list":
		resp, err := conn.ListSessions(ctx, &acp.ListSessionsRequest{Cwd: cwd})
		if err != nil {
			fatal("list sessions: %v", err)
		}
		switch parseOutputMode() {
		case client.OutputJSON:
			b, _ := json.MarshalIndent(resp.Sessions, "", "  ")
			fmt.Println(string(b))
		case client.OutputQuiet:
			for _, s := range resp.Sessions {
				fmt.Println(s.SessionID)
			}
		default:
			if len(resp.Sessions) == 0 {
				fmt.Println("no sessions")
				return
			}
			for _, s := range resp.Sessions {
				title := s.Title
				if title == "" {
					title = "(untitled)"
				}
				fmt.Printf("%s\t%s\t%s\t%s\n", s.SessionID, title, s.Cwd, s.UpdatedAt)
			}
		}

	case "new":
		resp, err := conn.NewSession(ctx, &acp.NewSessionRequest{
			Cwd:        cwd,
			MCPServers: []acp.MCPServer{},
		})
		if err != nil {
			fatal("new session: %v", err)
		}
		fmt.Println(resp.SessionID)

	default:
		fatal("unknown sessions subcommand: %s", subcmd)
	}
}

func runSet(ctx context.Context, conn *acp.ClientSideConnection, args []string) {
	if len(args) < 2 {
		fatal("usage: bvr set <key> <value>")
	}
	sid := requireSession("set")
	key, value := args[0], args[1]
	resp, err := conn.SetSessionConfigOption(ctx, &acp.SetSessionConfigOptionRequest{
		SessionID: acp.SessionID(sid),
		ConfigID:  acp.SessionConfigID(key),
		Value:     acp.SessionConfigValueID(value),
	})
	if err != nil {
		fatal("set config: %v", err)
	}
	if parseOutputMode() == client.OutputJSON {
		b, _ := json.MarshalIndent(resp.ConfigOptions, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Fprintf(os.Stderr, "set %s = %s\n", key, value)
	}
}

func runPrompt(ctx context.Context, conn *acp.ClientSideConnection, prompt string) {
	if prompt == "" {
		fatal("no prompt provided")
	}

	var sid acp.SessionID
	if sessionID != "" {
		sid = acp.SessionID(sessionID)
		_, err := conn.LoadSession(ctx, &acp.LoadSessionRequest{
			SessionID:  sid,
			Cwd:        cwd,
			MCPServers: []acp.MCPServer{},
		})
		if err != nil {
			fatal("load session: %v", err)
		}
	} else {
		resp, err := conn.NewSession(ctx, &acp.NewSessionRequest{
			Cwd:        cwd,
			MCPServers: []acp.MCPServer{},
		})
		if err != nil {
			fatal("new session: %v", err)
		}
		sid = resp.SessionID
	}

	sendPrompt(ctx, conn, sid, prompt)
}

func runPromptTUI(ctx context.Context, conn *acp.ClientSideConnection, cl *client.Client, initialPrompt string) {
	// Create or load session.
	var sid acp.SessionID
	if sessionID != "" {
		sid = acp.SessionID(sessionID)
		_, err := conn.LoadSession(ctx, &acp.LoadSessionRequest{
			SessionID:  sid,
			Cwd:        cwd,
			MCPServers: []acp.MCPServer{},
		})
		if err != nil {
			fatal("load session: %v", err)
		}
	} else {
		resp, err := conn.NewSession(ctx, &acp.NewSessionRequest{
			Cwd:        cwd,
			MCPServers: []acp.MCPServer{},
		})
		if err != nil {
			fatal("new session: %v", err)
		}
		sid = resp.SessionID
	}

	// Create the bubbletea program. The prompt callback sends prompts to the agent.
	var p *tea.Program

	promptFn := func(text string) {
		result, err := conn.Prompt(ctx, &acp.PromptRequest{
			SessionID: sid,
			Prompt:    []acp.ContentBlock{acp.NewContentBlockText(text)},
		})
		if err != nil {
			tui.SendDone(p, "", err)
			return
		}
		tui.SendDone(p, result.StopReason, nil)
	}

	var m tui.Model
	if initialPrompt != "" {
		m = tui.NewWithInitialPrompt(initialPrompt, promptFn)
	} else {
		m = tui.New(promptFn)
	}
	p = tea.NewProgram(m)

	// Wire client updates to the TUI program.
	cl.SetOnUpdate(func(notif *acp.SessionNotification) {
		tui.SendUpdate(p, *notif)
	})

	// If there's an initial prompt, fire it off immediately.
	if initialPrompt != "" {
		go promptFn(initialPrompt)
	}

	if _, err := p.Run(); err != nil {
		fatal("tui: %v", err)
	}
}

// --- Helpers ---

func sendPrompt(ctx context.Context, conn *acp.ClientSideConnection, sid acp.SessionID, prompt string) {
	result, err := conn.Prompt(ctx, &acp.PromptRequest{
		SessionID: sid,
		Prompt: []acp.ContentBlock{
			acp.NewContentBlockText(prompt),
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "\ninterrupted")
			os.Exit(130)
		}
		fatal("prompt failed: %v", err)
	}

	if parseOutputMode() != client.OutputJSON {
		fmt.Println()
	}

	if result.StopReason == acp.StopReasonMaxTokens {
		fmt.Fprintln(os.Stderr, "warning: response truncated (max tokens)")
	}
}

// readPrompt reads prompt text from -f flag, positional args, or stdin pipe.
func readPrompt(args []string) string {
	if promptFile != "" {
		var data []byte
		var err error
		if promptFile == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(promptFile)
		}
		if err != nil {
			fatal("read file %s: %v", promptFile, err)
		}
		return strings.TrimSpace(string(data))
	}

	if text := strings.Join(args, " "); text != "" {
		return text
	}

	// Try stdin if it's piped.
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fatal("read stdin: %v", err)
		}
		return strings.TrimSpace(string(data))
	}

	return ""
}

func requireSession(cmd string) string {
	if sessionID == "" {
		fatal("%s requires -s <session-id>", cmd)
	}
	return sessionID
}

func parseOutputMode() client.OutputMode {
	switch format {
	case "json":
		return client.OutputJSON
	case "quiet":
		return client.OutputQuiet
	default:
		return client.OutputText
	}
}

func parsePermissionMode() client.PermissionMode {
	if denyAll {
		return client.PermissionDenyAll
	}
	if approveAll {
		return client.PermissionApproveAll
	}
	return client.PermissionApproveReads
}


func connect(ctx context.Context, cl *client.Client, agentBinary string) *acp.ClientSideConnection {
	if httpAddr != "" {
		transport := acp.NewHTTPClientTransport(httpAddr)
		if err := transport.Connect(ctx); err != nil {
			fatal("connect to %s: %v", httpAddr, err)
		}
		return acp.NewClientSideConnection(cl, nil, nil, acp.WithTransport(transport))
	}
	conn, err := acp.SpawnAgent(ctx, cl, agentBinary)
	if err != nil {
		fatal("spawn agent %s: %v", agentBinary, err)
	}
	return conn
}

func initialize(ctx context.Context, conn *acp.ClientSideConnection) {
	_, err := conn.Initialize(ctx, &acp.InitializeRequest{
		ProtocolVersion: acp.ProtocolVersion(acp.CurrentProtocolVersion),
		ClientCapabilities: &acp.ClientCapabilities{
			FS: &acp.FileSystemCapabilities{
				ReadTextFile:  true,
				WriteTextFile: true,
			},
			Terminal: true,
		},
	})
	if err != nil {
		fatal("initialize: %v", err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "bvr: "+format+"\n", args...)
	os.Exit(1)
}
