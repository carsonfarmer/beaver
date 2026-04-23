package extensions

import (
	"strings"
	"testing"
)

func TestDiscoverContext(t *testing.T) {
	// covered by integration-style tests; pure XML formatting tested below
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

func TestContextToXML_Multiple(t *testing.T) {
	files := []contextFile{
		{Path: "/a.md", Content: "A"},
		{Path: "/b.md", Content: "B"},
	}
	xml := contextToXML(files)
	if !strings.Contains(xml, `<file path="/a.md">`) {
		t.Fatal("expected first file")
	}
	if !strings.Contains(xml, `<file path="/b.md">`) {
		t.Fatal("expected second file")
	}
}
