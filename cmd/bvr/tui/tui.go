// Package tui implements an inline terminal UI for the bvr ACP client
// using bubbletea. Output streams into the terminal's native scrollback
// as updates arrive. The only managed UI element is the input/spinner line.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	acp "github.com/ironpark/go-acp"
)

// SessionUpdateMsg wraps an ACP session notification for the bubbletea message loop.
type SessionUpdateMsg struct {
	Update acp.SessionNotification
}

// PromptDoneMsg signals that a prompt response has completed.
type PromptDoneMsg struct {
	StopReason acp.StopReason
	Err        error
}

// PromptFunc is called when the user submits a prompt. It runs in a goroutine.
type PromptFunc func(text string)

// Model is the top-level bubbletea model for the TUI.
type Model struct {
	width int

	// streamBuf accumulates raw text for the live View() during streaming.
	streamBuf *strings.Builder
	// agentText accumulates only agent message chunks for markdown rendering on flush.
	agentText *strings.Builder

	// Track tool calls for rendering updates.
	toolCalls map[acp.ToolCallID]*toolCallState

	// Session-level info.
	sessionTitle string
	usage        *usageInfo

	// Sub-components.
	spinner   spinner.Model
	textInput textinput.Model

	// State.
	streaming bool

	// Markdown renderer.
	renderer *glamour.TermRenderer

	// Prompt callback.
	onPrompt PromptFunc
}

type toolCallState struct {
	title     string
	status    acp.ToolCallStatus
	kind      *acp.ToolKind
	locations []acp.ToolCallLocation
	diffs     []acp.Diff
	content   []string
}

type usageInfo struct {
	size int64
	used int64
	cost *acp.Cost
}

// New creates a new TUI model.
func New(onPrompt PromptFunc) Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	ti := textinput.New()
	ti.Placeholder = "ask something..."
	ti.Prompt = "❯ "
	ti.CharLimit = 4096
	ti.Focus()

	r, _ := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(100),
	)

	return Model{
		streamBuf: &strings.Builder{},
		agentText: &strings.Builder{},
		toolCalls: make(map[acp.ToolCallID]*toolCallState),
		renderer:  r,
		spinner:   s,
		textInput: ti,
		onPrompt:  onPrompt,
		width:     100,
	}
}

// NewWithInitialPrompt creates a TUI model that starts streaming immediately.
func NewWithInitialPrompt(prompt string, onPrompt PromptFunc) Model {
	m := New(onPrompt)
	m.streaming = true
	m.textInput.Blur()
	return m
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.streaming {
			return m, nil
		}
		if msg.String() == "enter" {
			text := strings.TrimSpace(m.textInput.Value())
			if text == "" {
				return m, nil
			}
			return m, m.submitPrompt(text)
		}
		var cmd tea.Cmd
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.textInput.Width = msg.Width - 4
		return m, nil

	case spinner.TickMsg:
		if m.streaming {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			cmds = append(cmds, cmd)
		}

	case SessionUpdateMsg:
		cmd := m.handleSessionUpdate(&msg.Update)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		if m.streaming {
			cmds = append(cmds, m.spinner.Tick)
		}

	case PromptDoneMsg:
		cmds = append(cmds, m.finishTurn(msg.StopReason, msg.Err))
		m.streaming = false
		m.textInput.Focus()
		cmds = append(cmds, textinput.Blink)
	}

	return m, tea.Batch(cmds...)
}

func (m *Model) submitPrompt(text string) tea.Cmd {
	m.streamBuf.Reset()
	m.agentText.Reset()
	m.toolCalls = make(map[acp.ToolCallID]*toolCallState)
	m.streaming = true
	m.textInput.SetValue("")
	m.textInput.Blur()

	printCmd := tea.Println(userPromptStyle.Render("❯ " + text))

	if m.onPrompt != nil {
		go m.onPrompt(text)
	}

	return tea.Batch(printCmd, m.spinner.Tick)
}

// handleSessionUpdate processes an update and returns a tea.Cmd that prints
// content to scrollback as it arrives.
func (m *Model) handleSessionUpdate(params *acp.SessionNotification) tea.Cmd {
	return acp.MatchSessionUpdate(&params.Update, acp.SessionUpdateMatcher[tea.Cmd]{
		AgentMessageChunk: func(v acp.SessionUpdateAgentMessageChunk) tea.Cmd {
			if text, ok := v.Content.AsText(); ok {
				m.streamBuf.WriteString(text.Text)
				m.agentText.WriteString(text.Text)
				// Flush complete markdown blocks (paragraph breaks) to scrollback.
				return m.flushCompleteBlocks()
			}
			return nil
		},
		AgentThoughtChunk: func(v acp.SessionUpdateAgentThoughtChunk) tea.Cmd {
			if text, ok := v.Content.AsText(); ok {
				m.streamBuf.WriteString(thoughtStyle.Render(text.Text))
			}
			return nil
		},
		ToolCall: func(v acp.SessionUpdateToolCall) tea.Cmd {
			tc := &toolCallState{
				title:     v.Title,
				kind:      v.Kind,
				locations: v.Locations,
			}
			if v.Status != nil {
				tc.status = *v.Status
			}
			extractToolContent(&v.ToolCall, tc)
			m.toolCalls[v.ToolCallID] = tc

			// Flush streamed agent text as markdown before printing tool call.
			var cmds []tea.Cmd
			if m.agentText.Len() > 0 {
				md := m.agentText.String()
				rendered, renderErr := m.renderer.Render(md)
				if renderErr != nil {
					cmds = append(cmds, tea.Println(md))
				} else {
					cmds = append(cmds, tea.Println(strings.TrimRight(rendered, "\n")))
				}
				m.agentText.Reset()
			}
			m.flushStreamBuf()
			cmds = append(cmds, tea.Println(renderToolCall(tc, m.width)))
			return tea.Batch(cmds...)
		},
		ToolCallUpdate: func(v acp.SessionUpdateToolCallUpdate) tea.Cmd {
			tc, ok := m.toolCalls[v.ToolCallID]
			if !ok {
				return nil
			}
			if v.Title != "" {
				tc.title = v.Title
			}
			prevStatus := tc.status
			if v.Status != nil {
				tc.status = *v.Status
			}
			if len(v.Locations) > 0 {
				tc.locations = v.Locations
			}
			extractToolUpdateContent(&v.ToolCallUpdate, tc)

			// Print update when status changes or new content arrives.
			if (v.Status != nil && tc.status != prevStatus) || len(v.Content) > 0 {
				return tea.Println(renderToolCall(tc, m.width))
			}
			return nil
		},
		Plan: func(v acp.SessionUpdatePlan) tea.Cmd {
			return tea.Println(renderPlanBlock(v.Entries))
		},
		SessionInfoUpdate: func(v acp.SessionUpdateSessionInfoUpdate) tea.Cmd {
			if v.Title != "" {
				m.sessionTitle = v.Title
			}
			return nil
		},
		UsageUpdate: func(v acp.SessionUpdateUsageUpdate) tea.Cmd {
			m.usage = &usageInfo{
				size: v.Size,
				used: v.Used,
				cost: v.Cost,
			}
			return nil
		},
		Default: func() tea.Cmd { return nil },
	})
}

// flushCompleteBlocks renders and flushes complete markdown blocks (split on
// double newlines) to scrollback, keeping the in-progress tail in the buffers.
func (m *Model) flushCompleteBlocks() tea.Cmd {
	s := m.agentText.String()
	// Find the last paragraph break.
	idx := strings.LastIndex(s, "\n\n")
	if idx < 0 {
		return nil
	}

	complete := s[:idx]
	remainder := s[idx+2:] // skip the \n\n

	// Render the complete blocks as markdown.
	rendered, err := m.renderer.Render(complete)
	if err != nil {
		rendered = complete
	} else {
		rendered = strings.TrimRight(rendered, "\n")
	}

	// Update buffers to only contain the remainder.
	m.agentText.Reset()
	m.agentText.WriteString(remainder)
	m.streamBuf.Reset()
	m.streamBuf.WriteString(remainder)

	return tea.Println(rendered)
}

// flushStreamBuf clears the live stream buffer.
func (m *Model) flushStreamBuf() {
	m.streamBuf.Reset()
}

// finishTurn renders accumulated agent text as markdown, flushes to scrollback.
func (m *Model) finishTurn(stopReason acp.StopReason, err error) tea.Cmd {
	var cmds []tea.Cmd

	// Clear the live area.
	m.flushStreamBuf()

	// Render agent text as markdown and print to scrollback.
	if m.agentText.Len() > 0 {
		md := m.agentText.String()
		rendered, renderErr := m.renderer.Render(md)
		if renderErr != nil {
			cmds = append(cmds, tea.Println(md))
		} else {
			cmds = append(cmds, tea.Println(strings.TrimRight(rendered, "\n")))
		}
		m.agentText.Reset()
	}

	if err != nil {
		cmds = append(cmds, tea.Println(errorStyle.Render(fmt.Sprintf("error: %v", err))))
	}
	if stopReason == acp.StopReasonMaxTokens {
		cmds = append(cmds, tea.Println(errorStyle.Render("warning: response truncated (max tokens)")))
	}

	if m.usage != nil {
		cmds = append(cmds, tea.Println(renderUsageLine(m.usage)))
	}

	return tea.Batch(cmds...)
}

// View renders the live area at the bottom of the terminal.
// During streaming, renders accumulated text as markdown (re-rendered on each update).
func (m Model) View() string {
	if m.streaming {
		if m.agentText.Len() > 0 {
			md := m.agentText.String()
			rendered, err := m.renderer.Render(md)
			if err != nil {
				return md + "\n" + m.spinner.View()
			}
			return strings.TrimRight(rendered, "\n") + "\n" + m.spinner.View()
		}
		if m.streamBuf.Len() > 0 {
			// Thought text only (no agent text yet).
			return m.streamBuf.String() + "\n" + m.spinner.View()
		}
		return m.spinner.View() + " working…"
	}
	return m.textInput.View()
}

func extractToolContent(tc *acp.ToolCall, state *toolCallState) {
	for _, c := range tc.Content {
		acp.MatchToolCallContent(&c, acp.ToolCallContentMatcher[struct{}]{
			Content: func(v acp.ToolCallContentContent) struct{} {
				if text, ok := v.Content.Content.AsText(); ok {
					state.content = append(state.content, text.Text)
				}
				return struct{}{}
			},
			Diff: func(v acp.ToolCallContentDiff) struct{} {
				state.diffs = append(state.diffs, v.Diff)
				return struct{}{}
			},
			Terminal: func(v acp.ToolCallContentTerminal) struct{} {
				return struct{}{}
			},
			Default: func() struct{} { return struct{}{} },
		})
	}
}

func extractToolUpdateContent(tc *acp.ToolCallUpdate, state *toolCallState) {
	for _, c := range tc.Content {
		acp.MatchToolCallContent(&c, acp.ToolCallContentMatcher[struct{}]{
			Content: func(v acp.ToolCallContentContent) struct{} {
				if text, ok := v.Content.Content.AsText(); ok {
					state.content = append(state.content, text.Text)
				}
				return struct{}{}
			},
			Diff: func(v acp.ToolCallContentDiff) struct{} {
				state.diffs = append(state.diffs, v.Diff)
				return struct{}{}
			},
			Terminal: func(v acp.ToolCallContentTerminal) struct{} {
				return struct{}{}
			},
			Default: func() struct{} { return struct{}{} },
		})
	}
}

// SendUpdate sends a session update to the running bubbletea program.
func SendUpdate(p *tea.Program, notif acp.SessionNotification) {
	p.Send(SessionUpdateMsg{Update: notif})
}

// SendDone signals prompt completion to the running bubbletea program.
func SendDone(p *tea.Program, stopReason acp.StopReason, err error) {
	p.Send(PromptDoneMsg{StopReason: stopReason, Err: err})
}

// FormatTokens formats a token count with K/M suffixes.
func FormatTokens(n int64) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fK", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
