package instructions

import (
	"fmt"
	"strings"
	"testing"
)

type mockFS struct {
	files map[string]string
	dirs  map[string][]string
}

func (m *mockFS) ReadFile(path string) (string, error) {
	if c, ok := m.files[path]; ok {
		return c, nil
	}
	return "", fmt.Errorf("not found")
}

func (m *mockFS) WriteFile(path, content string) error {
	if m.files == nil {
		m.files = make(map[string]string)
	}
	m.files[path] = content
	return nil
}

func (m *mockFS) ListDir(dir string) []string {
	return m.dirs[dir]
}

func TestNew_Defaults(t *testing.T) {
	i := New()
	if i.basePrompt != DefaultPrompt {
		t.Fatalf("expected default prompt")
	}
	if len(i.contextFiles) != len(DefaultContextFiles) {
		t.Fatalf("expected default context files")
	}
	if len(i.skillsDirs) != 1 || i.skillsDirs[0] != DefaultSkillsDir {
		t.Fatalf("expected default skills dirs")
	}
}

func TestNew_WithOptions(t *testing.T) {
	i := New(
		WithBasePrompt("custom"),
		WithContextFiles("CUSTOM.md"),
		WithSkillsDirs(".custom-skills", ".more-skills"),
	)
	if i.basePrompt != "custom" {
		t.Fatalf("expected custom prompt, got %q", i.basePrompt)
	}
	if len(i.contextFiles) != 1 || i.contextFiles[0] != "CUSTOM.md" {
		t.Fatalf("expected custom context files, got %v", i.contextFiles)
	}
	if len(i.skillsDirs) != 2 || i.skillsDirs[0] != ".custom-skills" {
		t.Fatalf("expected custom skills dirs, got %v", i.skillsDirs)
	}
}

func TestDiscover(t *testing.T) {
	fs := &mockFS{
		files: map[string]string{
			"/project/AGENTS.md": "# Rules",
		},
	}
	i := New()
	prompt := i.Discover("/project", fs)
	if !strings.Contains(prompt, "# Rules") {
		t.Fatal("expected context in prompt")
	}
	if !strings.Contains(prompt, "/project") {
		t.Fatal("expected cwd in prompt")
	}
}

func TestDiscover_WithSkills(t *testing.T) {
	fs := &mockFS{
		files: map[string]string{
			"/project/.agents/skills/my-skill/SKILL.md": "---\nname: my-skill\ndescription: Does stuff.\n---\nBody.",
		},
		dirs: map[string][]string{"/project/.agents/skills": {"my-skill"}},
	}
	i := New()
	prompt := i.Discover("/project", fs)
	if !strings.Contains(prompt, "my-skill") {
		t.Fatal("expected skill in prompt")
	}
}

func TestDiscover_EmptyCwd(t *testing.T) {
	i := New()
	prompt := i.Discover("", &mockFS{})
	if prompt != DefaultPrompt {
		t.Fatalf("expected base prompt only, got %q", prompt)
	}
}

func TestSystemPrompt_BaseOnly(t *testing.T) {
	prompt := systemPrompt("base prompt", "", nil, nil)
	if prompt != "base prompt" {
		t.Fatalf("expected base prompt, got %q", prompt)
	}
}

func TestSystemPrompt_WithCwd(t *testing.T) {
	prompt := systemPrompt("base", "/project", nil, nil)
	if !strings.Contains(prompt, "/project") {
		t.Fatal("expected cwd in prompt")
	}
}

func TestSystemPrompt_WithContext(t *testing.T) {
	ctx := []contextFile{{Path: "/project/AGENTS.md", Content: "# Rules\nUse Go."}}
	prompt := systemPrompt("base", "/project", ctx, nil)
	if !strings.Contains(prompt, "<context>") {
		t.Fatal("expected context XML")
	}
	if !strings.Contains(prompt, "Use Go.") {
		t.Fatal("expected context content")
	}
}

func TestSystemPrompt_WithSkills(t *testing.T) {
	sk := []Skill{{Name: "test-skill", Description: "A test.", Location: "/project/.agents/skills/test-skill/SKILL.md"}}
	prompt := systemPrompt("base", "/project", nil, sk)
	if !strings.Contains(prompt, "<available_skills>") {
		t.Fatal("expected skills XML")
	}
	if !strings.Contains(prompt, "test-skill") {
		t.Fatal("expected skill name")
	}
}
