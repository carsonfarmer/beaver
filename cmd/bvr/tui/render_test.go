package tui

import (
	"strings"
	"testing"

	acp "github.com/ironpark/go-acp"
)

func TestRenderDiff_NewFile(t *testing.T) {
	d := acp.Diff{
		Path:    "new.go",
		OldText: "",
		NewText: "package main\n\nfunc main() {}\n",
	}
	out := renderDiff(d, 80)
	if !strings.Contains(out, "+ package main") {
		t.Errorf("expected additions for new file, got:\n%s", out)
	}
	if !strings.Contains(out, "new.go") {
		t.Errorf("expected file path in header, got:\n%s", out)
	}
}

func TestRenderDiff_Modification(t *testing.T) {
	d := acp.Diff{
		Path:    "main.go",
		OldText: "line1\nline2\nline3\nline4\nline5\n",
		NewText: "line1\nline2\nLINE3\nline4\nline5\n",
	}
	out := renderDiff(d, 80)
	if !strings.Contains(out, "- line3") {
		t.Errorf("expected removal of old line, got:\n%s", out)
	}
	if !strings.Contains(out, "+ LINE3") {
		t.Errorf("expected addition of new line, got:\n%s", out)
	}
}

func TestRenderDiff_ContextCollapse(t *testing.T) {
	// Build a file with many unchanged lines and one change in the middle.
	var oldLines, newLines []string
	for i := range 20 {
		oldLines = append(oldLines, "unchanged line "+string(rune('A'+i)))
		newLines = append(newLines, "unchanged line "+string(rune('A'+i)))
	}
	oldLines[10] = "old middle"
	newLines[10] = "new middle"

	d := acp.Diff{
		Path:    "big.go",
		OldText: strings.Join(oldLines, "\n"),
		NewText: strings.Join(newLines, "\n"),
	}
	out := renderDiff(d, 80)
	if !strings.Contains(out, "···") {
		t.Errorf("expected collapsed context markers, got:\n%s", out)
	}
	if !strings.Contains(out, "- old middle") {
		t.Errorf("expected removal, got:\n%s", out)
	}
	if !strings.Contains(out, "+ new middle") {
		t.Errorf("expected addition, got:\n%s", out)
	}
}

func TestFormatTokens(t *testing.T) {
	tests := []struct {
		n    int64
		want string
	}{
		{500, "500"},
		{1500, "1.5K"},
		{1_500_000, "1.5M"},
	}
	for _, tt := range tests {
		got := FormatTokens(tt.n)
		if got != tt.want {
			t.Errorf("FormatTokens(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestToolStatusDisplay(t *testing.T) {
	tests := []struct {
		status acp.ToolCallStatus
		icon   string
	}{
		{acp.ToolCallStatusPending, "○"},
		{acp.ToolCallStatusInProgress, "◐"},
		{acp.ToolCallStatusCompleted, "●"},
		{acp.ToolCallStatusFailed, "✗"},
	}
	for _, tt := range tests {
		icon, _ := toolStatusDisplay(tt.status)
		if icon != tt.icon {
			t.Errorf("toolStatusDisplay(%q) icon = %q, want %q", tt.status, icon, tt.icon)
		}
	}
}

func TestPlanStatusDisplay(t *testing.T) {
	tests := []struct {
		status acp.PlanEntryStatus
		icon   string
	}{
		{acp.PlanEntryStatusPending, "○"},
		{acp.PlanEntryStatusInProgress, "◐"},
		{acp.PlanEntryStatusCompleted, "●"},
	}
	for _, tt := range tests {
		icon, _ := planStatusDisplay(tt.status)
		if icon != tt.icon {
			t.Errorf("planStatusDisplay(%q) icon = %q, want %q", tt.status, icon, tt.icon)
		}
	}
}
