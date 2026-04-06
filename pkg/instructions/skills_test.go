package instructions

import (
	"strings"
	"testing"
)

func TestParseFrontmatter(t *testing.T) {
	content := "---\nname: test-skill\ndescription: A test skill for testing.\n---\n\n# Instructions\nDo the thing."
	s, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test-skill" {
		t.Fatalf("expected name %q, got %q", "test-skill", s.Name)
	}
	if s.Description != "A test skill for testing." {
		t.Fatalf("expected description %q, got %q", "A test skill for testing.", s.Description)
	}
}

func TestParseFrontmatter_MissingName(t *testing.T) {
	_, err := parseFrontmatter("---\ndescription: A skill.\n---\nBody.")
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestParseFrontmatter_MissingDescription(t *testing.T) {
	_, err := parseFrontmatter("---\nname: my-skill\n---\nBody.")
	if err == nil {
		t.Fatal("expected error for missing description")
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	_, err := parseFrontmatter("# Just markdown")
	if err == nil {
		t.Fatal("expected error for no frontmatter")
	}
}

func TestParseFrontmatter_UnclosedFrontmatter(t *testing.T) {
	_, err := parseFrontmatter("---\nname: test\n")
	if err == nil {
		t.Fatal("expected error for unclosed frontmatter")
	}
}

func TestParseFrontmatter_WindowsLineEndings(t *testing.T) {
	content := "---\r\nname: test-skill\r\ndescription: Works on Windows.\r\n---\r\nBody."
	s, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if s.Name != "test-skill" {
		t.Fatalf("expected name %q, got %q", "test-skill", s.Name)
	}
}

func TestDiscoverSkills(t *testing.T) {
	fs := &mockFS{
		dirs: map[string][]string{"/project/.agents/skills": {"my-skill"}},
		files: map[string]string{
			"/project/.agents/skills/my-skill/SKILL.md": "---\nname: my-skill\ndescription: Does stuff.\n---\nBody.",
		},
	}
	found := discoverSkills("/project", DefaultSkillsDir, fs)
	if len(found) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(found))
	}
	if found[0].Name != "my-skill" {
		t.Fatalf("expected name %q, got %q", "my-skill", found[0].Name)
	}
}

func TestDiscoverSkills_EmptyCwd(t *testing.T) {
	if len(discoverSkills("", DefaultSkillsDir, nil)) != 0 {
		t.Fatal("expected no skills for empty cwd")
	}
}

func TestSkillsToXML(t *testing.T) {
	sk := []Skill{
		{Name: "pdf-tool", Description: "Handles PDFs.", Location: "/project/.agents/skills/pdf-tool/SKILL.md"},
		{Name: "data-viz", Description: "Charts & graphs.", Location: "/project/.agents/skills/data-viz/SKILL.md"},
	}
	xml := skillsToXML(sk)
	if !strings.Contains(xml, "<available_skills>") {
		t.Fatal("expected <available_skills> tag")
	}
	if !strings.Contains(xml, "<name>pdf-tool</name>") {
		t.Fatal("expected pdf-tool name")
	}
	if !strings.Contains(xml, "Charts &amp; graphs.") {
		t.Fatal("expected escaped ampersand")
	}
}

func TestSkillsToXML_Empty(t *testing.T) {
	if skillsToXML(nil) != "" {
		t.Fatal("expected empty string")
	}
}
