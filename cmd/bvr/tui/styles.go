package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Thought text: dim, italic.
	thoughtStyle = lipgloss.NewStyle().
			Faint(true).
			Italic(true)

	// Tool call status styles.
	toolHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")).
			Bold(true)
	toolPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("8"))
	toolRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("3")).
				Bold(true)
	toolDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
	toolErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))

	// Diff styles.
	diffHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Bold(true)
	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2"))
	diffRemoveStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1"))
	diffContextStyle = lipgloss.NewStyle().
				Faint(true)

	// Plan entry styles.
	planPendingStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	planInProgressStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	planCompletedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))

	// Status bar.
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Faint(true)

	// Section divider.
	dividerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			Faint(true)

	// Error style.
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Bold(true)

	// Location path style.
	locationStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")).
			Faint(true)

	// User prompt in conversation history.
	userPromptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("4")).
			Bold(true)
)
