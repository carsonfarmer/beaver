package agent

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"sync"

	"charm.land/fantasy"
	"github.com/carsonfarmer/beaver/pkg/instructions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	"github.com/carsonfarmer/beaver/pkg/tools"
	acp "github.com/ironpark/go-acp"
)

// Agent implements acp.Agent and the optional session lifecycle interfaces.
type Agent struct {
	registry     llm.ModelRegistry
	store        acp.SessionStore[*session.Session]
	client       acp.Client
	instructions *instructions.Instructions
}

// New creates a new Agent with the given dependencies.
func New(registry llm.ModelRegistry, store acp.SessionStore[*session.Session], insts *instructions.Instructions) *Agent {
	return &Agent{registry: registry, store: store, instructions: insts}
}

// SetClient sets the ACP client for outbound calls and notifications.
func (a *Agent) SetClient(c acp.Client) { a.client = c }

// --- ACP Agent interface ---

func (a *Agent) Initialize(_ context.Context, _ *acp.InitializeRequest) (*acp.InitializeResponse, error) {
	return &acp.InitializeResponse{
		ProtocolVersion: acp.ProtocolVersion(acp.CurrentProtocolVersion),
		AgentCapabilities: &acp.AgentCapabilities{
			LoadSession:     true,
			MCPCapabilities: &acp.MCPCapabilities{},
			PromptCapabilities: &acp.PromptCapabilities{
				Image:           true,
				EmbeddedContext: true,
			},
			SessionCapabilities: &acp.SessionCapabilities{
				List:   &acp.SessionListCapabilities{},
				Fork:   &acp.SessionForkCapabilities{},
				Resume: &acp.SessionResumeCapabilities{},
			},
		},
		AgentInfo:   &acp.Implementation{Name: "beaver", Title: "Beaver", Version: "0.1.0"},
		AuthMethods: []acp.AuthMethod{},
	}, nil
}

func (a *Agent) Authenticate(_ context.Context, _ *acp.AuthenticateRequest) (*acp.AuthenticateResponse, error) {
	return &acp.AuthenticateResponse{}, nil
}

func (a *Agent) SetSessionMode(_ context.Context, _ *acp.SetSessionModeRequest) (*acp.SetSessionModeResponse, error) {
	return &acp.SetSessionModeResponse{}, nil
}

func (a *Agent) SetSessionConfigOption(_ context.Context, req *acp.SetSessionConfigOptionRequest) (*acp.SetSessionConfigOptionResponse, error) {
	sess, ok := a.store.Get(req.SessionID)
	if !ok {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	switch req.ConfigID {
	case session.ConfigModel:
		sess.Model = string(req.Value)
	case session.ConfigThoughtLevel:
		sess.ThoughtLevel = string(req.Value)
	}
	a.store.Set(req.SessionID, sess)
	return &acp.SetSessionConfigOptionResponse{
		ConfigOptions: session.ConfigOptions(a.registry, sess),
	}, nil
}

func (a *Agent) Cancel(_ context.Context, req *acp.CancelNotification) error {
	if sess, ok := a.store.Get(req.SessionID); ok && sess.Cancel != nil {
		sess.Cancel()
	}
	return nil
}

func (a *Agent) Prompt(ctx context.Context, req *acp.PromptRequest) (*acp.PromptResponse, error) {
	sess, ok := a.store.Get(req.SessionID)
	if !ok {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	if sess.Model == "" {
		return nil, acp.ErrInvalidParams(nil, "no model configured on session")
	}

	// Cancel any previous prompt for this session.
	if sess.Cancel != nil {
		sess.Cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	sess.Cancel = cancel
	a.store.Set(req.SessionID, sess)
	defer func() {
		cancel()
		sess.Cancel = nil
	}()

	model, err := a.registry.ResolveModel(ctx, sess.Model)
	if err != nil {
		return nil, acp.ErrInternalError(nil, err.Error())
	}
	opts := a.registry.ModelOptions(sess.Model, sess.ThoughtLevel)

	if sess.SystemPrompt == "" && a.client != nil {
		fs := &ClientFilesystem{Ctx: ctx, SessionID: req.SessionID, Client: a.client}
		sess.SystemPrompt = a.instructions.Discover(sess.Cwd, fs)
	}

	if msg, ok := llm.ContentBlocksToMessage(req.Prompt); ok {
		sess.History = append(sess.History, msg)
		// Set title from first user message.
		if sess.Title == "" {
			sess.Title = titleFromMessage(msg)
		}
	}

	stream := acp.NewSessionStream(a.client, req.SessionID)
	fa := fantasy.NewAgent(model,
		fantasy.WithTools(tools.ForSession(a.client, req.SessionID)...),
		fantasy.WithSystemPrompt(sess.SystemPrompt),
	)

	// Track per-tool-call input for constructing rich content on completion.
	var toolInputsMu sync.Mutex
	toolInputs := make(map[string]json.RawMessage)

	result, err := fa.Stream(ctx, fantasy.AgentStreamCall{
		Messages:        sess.History,
		MaxOutputTokens: opts.MaxOutputTokens,
		ProviderOptions: opts.ProviderOptions,
		OnTextDelta: func(_, text string) error {
			return stream.SendText(ctx, text)
		},
		OnReasoningDelta: func(_, text string) error {
			return stream.SendThought(ctx, text)
		},
		OnToolCall: func(tc fantasy.ToolCallContent) error {
			toolInputsMu.Lock()
			toolInputs[tc.ToolCallID] = json.RawMessage(tc.Input)
			toolInputsMu.Unlock()
			locs := tools.Locations(tc.ToolName, tc.Input)
			kind := tools.Kind(tc.ToolName)
			status := acp.ToolCallStatusInProgress
			tc2 := acp.ToolCall{
				ToolCallID: acp.ToolCallID(tc.ToolCallID),
				Title:      tools.Title(tc.ToolName, tc.Input, sess.Cwd),
				Kind:       &kind,
				Status:     &status,
				RawInput:   json.RawMessage(tc.Input),
			}
			if len(locs) > 0 {
				tc2.Locations = locs
			}
			return stream.SendUpdate(ctx, acp.NewSessionUpdateToolCall(tc2))
		},
		OnToolResult: func(res fantasy.ToolResultContent) error {
			toolInputsMu.Lock()
			input := toolInputs[res.ToolCallID]
			delete(toolInputs, res.ToolCallID)
			toolInputsMu.Unlock()
			if tools.IsError(res.Result) {
				return stream.FailToolCall(ctx, acp.ToolCallID(res.ToolCallID))
			}
			content := tools.ResultContent(res.ToolName, input, res.Result, res.ClientMetadata)
			status := acp.ToolCallStatusCompleted
			update := acp.ToolCallUpdate{
				ToolCallID: acp.ToolCallID(res.ToolCallID),
				Status:     &status,
				Content:    content,
				RawOutput:  mustMarshal(tools.TextResult(res.Result)),
			}
			return stream.SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(update))
		},
	})
	if err != nil {
		if ctx.Err() != nil {
			return &acp.PromptResponse{StopReason: acp.StopReasonCancelled}, nil
		}
		return nil, acp.ErrInternalError(nil, providerErrorDetail(err))
	}

	for _, step := range result.Steps {
		sess.History = append(sess.History, step.Messages...)
	}
	a.store.Set(req.SessionID, sess)
	stream.SendSessionInfo(ctx, sess.Title, sess.UpdatedAt)
	if opts.ContextWindow > 0 {
		stream.SendUpdate(ctx, acp.NewSessionUpdateUsageUpdate(acp.UsageUpdate{
			Used: result.TotalUsage.InputTokens + result.TotalUsage.OutputTokens,
			Size: opts.ContextWindow,
		}))
	}

	stopReason := acp.StopReasonEndTurn
	if result.Response.FinishReason == fantasy.FinishReasonLength {
		stopReason = acp.StopReasonMaxTokens
	}
	usage := llm.UsageToUsage(result.TotalUsage)
	return &acp.PromptResponse{
		StopReason: stopReason,
		Usage:      &usage,
	}, nil
}

// --- Session lifecycle interfaces ---

func (a *Agent) NewSession(ctx context.Context, req *acp.NewSessionRequest) (*acp.NewSessionResponse, error) {
	id := acp.GenerateSessionID()
	sess := &session.Session{
		Cwd:          req.Cwd,
		Model:        a.registry.Defaults().Model,
		ThoughtLevel: a.registry.Defaults().ThoughtLevel,
	}
	a.store.Set(id, sess)
	return &acp.NewSessionResponse{
		SessionID:     id,
		ConfigOptions: session.ConfigOptions(a.registry, sess),
	}, nil
}

func (a *Agent) LoadSession(ctx context.Context, req *acp.LoadSessionRequest) (*acp.LoadSessionResponse, error) {
	sess, ok := a.store.Get(req.SessionID)
	if !ok {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	// Replay conversation history as session updates so the UI can reconstruct it.
	if a.client != nil {
		go a.replayHistory(ctx, req.SessionID, sess)
	}
	return &acp.LoadSessionResponse{
		ConfigOptions: session.ConfigOptions(a.registry, sess),
	}, nil
}

func (a *Agent) ListSessions(_ context.Context, _ *acp.ListSessionsRequest) (*acp.ListSessionsResponse, error) {
	ids := a.store.List()
	sessions := make([]acp.SessionInfo, 0, len(ids))
	for _, id := range ids {
		sess, ok := a.store.Get(id)
		if !ok {
			continue
		}
		sessions = append(sessions, sess.Info(id))
	}
	return &acp.ListSessionsResponse{Sessions: sessions}, nil
}

func (a *Agent) ForkSession(ctx context.Context, req *acp.ForkSessionRequest) (*acp.ForkSessionResponse, error) {
	src, ok := a.store.Get(req.SessionID)
	if !ok {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	newID := acp.GenerateSessionID()
	forked := src.Fork(req.Cwd)
	a.store.Set(newID, forked)
	return &acp.ForkSessionResponse{
		SessionID: newID,
	}, nil
}

func (a *Agent) ResumeSession(_ context.Context, req *acp.ResumeSessionRequest) (*acp.ResumeSessionResponse, error) {
	if _, ok := a.store.Get(req.SessionID); !ok {
		return nil, acp.ErrInvalidParams(nil, "session not found")
	}
	return &acp.ResumeSessionResponse{}, nil
}

// replayHistory sends session updates to reconstruct the conversation in the UI.
func (a *Agent) replayHistory(ctx context.Context, sid acp.SessionID, sess *session.Session) {
	stream := acp.NewSessionStream(a.client, sid)
	toolNames := make(map[string]string)
	for _, msg := range sess.History {
		for _, part := range sortParts(msg.Content) {
			switch p := part.(type) {
			case fantasy.TextPart:
				if msg.Role == fantasy.MessageRoleUser {
					stream.SendUserMessage(ctx, p.Text)
				} else {
					stream.SendText(ctx, p.Text)
				}
			case fantasy.ReasoningPart:
				stream.SendThought(ctx, p.Text)
			case fantasy.ToolCallPart:
				toolNames[p.ToolCallID] = p.ToolName
				locs := tools.Locations(p.ToolName, p.Input)
				kind := tools.Kind(p.ToolName)
				status := acp.ToolCallStatusInProgress
				tc := acp.ToolCall{
					ToolCallID: acp.ToolCallID(p.ToolCallID),
					Title:      tools.Title(p.ToolName, p.Input, sess.Cwd),
					Kind:       &kind,
					Status:     &status,
					RawInput:   json.RawMessage(p.Input),
				}
				if len(locs) > 0 {
					tc.Locations = locs
				}
				if p.ToolName == "execute" {
					tc.Meta = map[string]any{
						"terminal_info": map[string]any{
							"terminal_id": p.ToolCallID,
						},
					}
				}
				stream.SendUpdate(ctx, acp.NewSessionUpdateToolCall(tc))
			case fantasy.ToolResultPart:
				toolName := toolNames[p.ToolCallID]
				text := tools.TextResult(p.Output)
				status := acp.ToolCallStatusCompleted
				update := acp.ToolCallUpdate{
					ToolCallID: acp.ToolCallID(p.ToolCallID),
					Status:     &status,
					RawOutput:  mustMarshal(text),
				}
				if toolName == "execute" {
					update.Content = []acp.ToolCallContent{
						acp.NewToolCallContentTerminal(p.ToolCallID),
					}
					update.Meta = map[string]any{
						"terminal_output": map[string]any{
							"terminal_id": p.ToolCallID,
							"data":        text,
						},
						"terminal_exit": map[string]any{
							"terminal_id": p.ToolCallID,
							"exit_code":   0,
							"signal":      nil,
						},
					}
				} else {
					update.Content = []acp.ToolCallContent{
						acp.NewToolCallContentContent(acp.NewContentBlockText(text)),
					}
				}
				stream.SendUpdate(ctx, acp.NewSessionUpdateToolCallUpdate(update))
			}
		}
	}
}

// sortParts reorders message parts so reasoning comes before text and tool calls.
// LLM providers may return parts in arbitrary order within a message.
func sortParts(parts []fantasy.MessagePart) []fantasy.MessagePart {
	sorted := make([]fantasy.MessagePart, len(parts))
	copy(sorted, parts)
	sort.SliceStable(sorted, func(i, j int) bool {
		return partOrder(sorted[i]) < partOrder(sorted[j])
	})
	return sorted
}

func partOrder(p fantasy.MessagePart) int {
	switch p.(type) {
	case fantasy.ReasoningPart:
		return 0
	case fantasy.TextPart:
		return 1
	case fantasy.ToolCallPart:
		return 2
	case fantasy.ToolResultPart:
		return 3
	default:
		return 4
	}
}

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

// titleFromMessage extracts a short title from a user message.
func titleFromMessage(msg fantasy.Message) string {
	for _, part := range msg.Content {
		if tp, ok := part.(fantasy.TextPart); ok && tp.Text != "" {
			t := tp.Text
			// Use only the first line.
			if i := strings.IndexByte(t, '\n'); i >= 0 {
				t = t[:i]
			}
			if len(t) > 80 {
				t = t[:80] + "…"
			}
			return t
		}
	}
	return ""
}

// providerErrorDetail extracts a useful error message from provider errors,
// including the response body when available.
func providerErrorDetail(err error) string {
	var pe *fantasy.ProviderError
	if errors.As(err, &pe) {
		msg := pe.Error()
		if len(pe.ResponseBody) > 0 {
			msg += ": " + string(pe.ResponseBody)
		}
		return msg
	}
	return err.Error()
}
