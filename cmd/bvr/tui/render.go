package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	acp "github.com/ironpark/go-acp"
	"github.com/sergi/go-diff/diffmatchpatch"
)

// renderToolCall renders a tool call with its full content for scrollback.
func renderToolCall(tc *toolCallState, width int) string {
	var b strings.Builder

	// Header: status icon + title + locations.
	icon, style := toolStatusDisplay(tc.status)
	b.WriteString(style.Render(fmt.Sprintf("%s %s", icon, tc.title)))

	for _, loc := range tc.locations {
		path := loc.Path
		if loc.Line != nil {
			path = fmt.Sprintf("%s:%d", path, *loc.Line)
		}
		b.WriteString(" " + locationStyle.Render(path))
	}

	// Diffs.
	for _, d := range tc.diffs {
		b.WriteByte('\n')
		b.WriteString(renderDiff(d, width))
	}

	// Text content (file reads, command output, etc.).
	for _, text := range tc.content {
		trimmed := strings.TrimSpace(text)
		if trimmed != "" {
			w := min(width, 80)
			b.WriteByte('\n')
			b.WriteString(dividerStyle.Render(strings.Repeat("─", w)))
			b.WriteByte('\n')
			// Truncate very long output to keep scrollback readable.
			lines := strings.Split(trimmed, "\n")
			if len(lines) > 30 {
				for _, line := range lines[:15] {
					b.WriteString(line)
					b.WriteByte('\n')
				}
				b.WriteString(dividerStyle.Render(fmt.Sprintf("  … %d lines omitted …", len(lines)-30)))
				b.WriteByte('\n')
				for _, line := range lines[len(lines)-15:] {
					b.WriteString(line)
					b.WriteByte('\n')
				}
			} else {
				b.WriteString(trimmed)
				b.WriteByte('\n')
			}
			b.WriteString(dividerStyle.Render(strings.Repeat("─", w)))
		}
	}

	return b.String()
}

func toolStatusDisplay(status acp.ToolCallStatus) (string, lipgloss.Style) {
	switch status {
	case acp.ToolCallStatusPending:
		return "○", toolPendingStyle
	case acp.ToolCallStatusInProgress:
		return "◐", toolRunningStyle
	case acp.ToolCallStatusCompleted:
		return "●", toolDoneStyle
	case acp.ToolCallStatusFailed:
		return "✗", toolErrorStyle
	default:
		return "▸", toolHeaderStyle
	}
}

// renderDiff renders a diff using a line-level diff algorithm.
func renderDiff(d acp.Diff, width int) string {
	var b strings.Builder
	w := min(width, 80)

	pathLabel := fmt.Sprintf("── %s ", d.Path)
	padding := max(w-len(pathLabel), 0)
	b.WriteString(diffHeaderStyle.Render(pathLabel + strings.Repeat("─", padding)))
	b.WriteByte('\n')

	if d.OldText == "" {
		for _, line := range strings.Split(d.NewText, "\n") {
			b.WriteString(diffAddStyle.Render("+ " + line))
			b.WriteByte('\n')
		}
		return b.String()
	}

	if d.NewText == "" {
		for _, line := range strings.Split(d.OldText, "\n") {
			b.WriteString(diffRemoveStyle.Render("- " + line))
			b.WriteByte('\n')
		}
		return b.String()
	}

	dmp := diffmatchpatch.New()
	charA, charB, lines := dmp.DiffLinesToChars(d.OldText, d.NewText)
	diffs := dmp.DiffMain(charA, charB, false)
	diffs = dmp.DiffCharsToLines(diffs, lines)
	diffs = dmp.DiffCleanupSemantic(diffs)

	const contextLines = 3
	type diffLine struct {
		op   diffmatchpatch.Operation
		text string
	}

	var allLines []diffLine
	for _, d := range diffs {
		for _, line := range strings.Split(strings.TrimRight(d.Text, "\n"), "\n") {
			allLines = append(allLines, diffLine{op: d.Type, text: line})
		}
	}

	show := make([]bool, len(allLines))
	for i, dl := range allLines {
		if dl.op != diffmatchpatch.DiffEqual {
			lo := max(i-contextLines, 0)
			hi := min(i+contextLines+1, len(allLines))
			for j := lo; j < hi; j++ {
				show[j] = true
			}
		}
	}

	skipping := false
	for i, dl := range allLines {
		if !show[i] {
			if !skipping {
				b.WriteString(diffContextStyle.Render("  ···"))
				b.WriteByte('\n')
				skipping = true
			}
			continue
		}
		skipping = false

		switch dl.op {
		case diffmatchpatch.DiffInsert:
			b.WriteString(diffAddStyle.Render("+ " + dl.text))
		case diffmatchpatch.DiffDelete:
			b.WriteString(diffRemoveStyle.Render("- " + dl.text))
		default:
			b.WriteString(diffContextStyle.Render("  " + dl.text))
		}
		b.WriteByte('\n')
	}

	return b.String()
}

// renderPlanBlock renders the plan as a block of text for scrollback.
func renderPlanBlock(entries []acp.PlanEntry) string {
	var b strings.Builder
	b.WriteString(dividerStyle.Render("Plan"))
	b.WriteByte('\n')

	for _, entry := range entries {
		icon, style := planStatusDisplay(entry.Status)
		b.WriteString(style.Render(fmt.Sprintf("  %s %s", icon, entry.Content)))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

func planStatusDisplay(status acp.PlanEntryStatus) (string, lipgloss.Style) {
	switch status {
	case acp.PlanEntryStatusPending:
		return "○", planPendingStyle
	case acp.PlanEntryStatusInProgress:
		return "◐", planInProgressStyle
	case acp.PlanEntryStatusCompleted:
		return "●", planCompletedStyle
	default:
		return "○", planPendingStyle
	}
}

// renderUsageLine renders the token usage as a single line.
func renderUsageLine(u *usageInfo) string {
	ctx := fmt.Sprintf("%s/%s tokens", FormatTokens(u.used), FormatTokens(u.size))
	if u.cost != nil {
		ctx += fmt.Sprintf(" · $%.4f", u.cost.Amount)
	}
	return statusBarStyle.Render(ctx)
}
