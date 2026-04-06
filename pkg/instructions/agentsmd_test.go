package instructions

import (
	"strings"
	"testing"
)

func TestDiscoverContext(t *testing.T) {
	fs := &mockFS{files: map[string]string{
		"/project/AGENTS.md": "# Root",
	}}
	found := discoverContext("/project", DefaultContextFiles, fs)
	if len(found) != 1 {
		t.Fatalf("expected 1 file, got %d", len(found))
	}
	if found[0].Path != "/project/AGENTS.md" {
		t.Fatalf("expected cwd AGENTS.md, got %q", found[0].Path)
	}
}

func TestDiscoverContext_DotAgentsDir(t *testing.T) {
	fs := &mockFS{files: map[string]string{
		"/project/.agents/AGENTS.md": "# Dot agents",
	}}
	found := discoverContext("/project", DefaultContextFiles, fs)
	if len(found) != 1 {
		t.Fatalf("expected 1 file, got %d", len(found))
	}
	if found[0].Path != "/project/.agents/AGENTS.md" {
		t.Fatalf("expected .agents/AGENTS.md, got %q", found[0].Path)
	}
}

func TestDiscoverContext_Both(t *testing.T) {
	fs := &mockFS{files: map[string]string{
		"/project/AGENTS.md":         "# Root",
		"/project/.agents/AGENTS.md": "# Dot agents",
	}}
	found := discoverContext("/project", DefaultContextFiles, fs)
	if len(found) != 2 {
		t.Fatalf("expected 2 files, got %d", len(found))
	}
}

func TestDiscoverContext_DoesNotWalkUp(t *testing.T) {
	fs := &mockFS{files: map[string]string{
		"/project/AGENTS.md": "# Parent",
	}}
	found := discoverContext("/project/sub", DefaultContextFiles, fs)
	if len(found) != 0 {
		t.Fatal("should not walk up to parent directories")
	}
}

func TestDiscoverContext_Empty(t *testing.T) {
	fs := &mockFS{files: map[string]string{}}
	if len(discoverContext("/project", DefaultContextFiles, fs)) != 0 {
		t.Fatal("expected no files")
	}
}

func TestDiscoverContext_EmptyCwd(t *testing.T) {
	if len(discoverContext("", DefaultContextFiles, nil)) != 0 {
		t.Fatal("expected no files for empty cwd")
	}
}

func TestContextToXML_Empty(t *testing.T) {
	if contextToXML(nil) != "" {
		t.Fatal("expected empty string")
	}
}

func TestContextToXML_TrailingNewline(t *testing.T) {
	xml := contextToXML([]contextFile{{Path: "/p/AGENTS.md", Content: "no newline"}})
	if !strings.Contains(xml, "no newline\n</file>") {
		t.Fatal("expected trailing newline added")
	}
}
