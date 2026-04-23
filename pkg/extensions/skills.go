package extensions

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

	acp "github.com/ironpark/go-acp"
	"gopkg.in/yaml.v3"
)

// DefaultSkillsDir is the standard skill directory relative to the project root.
const DefaultSkillsDir = ".agents/skills"

const skillsUsage = "The skills listed above provide specialized instructions for specific tasks.\n" +
	"When a task matches a skill's description, use the read_file tool to load\n" +
	"the SKILL.md at the listed location before proceeding."

// Skill represents a parsed SKILL.md file's metadata.
type Skill struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description" yaml:"description"`
	Location    string `json:"location" yaml:"-"` // absolute path to SKILL.md
}

// Skills returns a provider that discovers SKILL.md files and contributes
// their metadata as an <available_skills> section.
func Skills(client acp.Client, dirs ...string) Provider {
	if len(dirs) == 0 {
		dirs = []string{DefaultSkillsDir}
	}
	return func(ctx context.Context, cwd string, sid acp.SessionID) (string, error) {
		var all []Skill
		for _, d := range dirs {
			all = append(all, discoverSkills(ctx, cwd, d, client, sid)...)
		}
		if len(all) == 0 {
			return "", nil
		}
		return skillsToXML(all) + "\n\n" + skillsUsage, nil
	}
}

func discoverSkills(ctx context.Context, cwd, skillsDir string, client acp.Client, sid acp.SessionID) []Skill {
	if cwd == "" {
		return nil
	}
	dir := filepath.Join(cwd, skillsDir)
	var found []Skill
	for _, name := range listDir(ctx, client, sid, dir) {
		loc := filepath.Join(dir, name, "SKILL.md")
		resp, err := client.ReadTextFile(ctx, &acp.ReadTextFileRequest{
			Path:      loc,
			SessionID: sid,
		})
		if err != nil {
			continue
		}
		skill, err := parseFrontmatter(resp.Content)
		if err != nil {
			continue
		}
		skill.Location = loc
		found = append(found, *skill)
	}
	return found
}

func listDir(ctx context.Context, client acp.Client, sid acp.SessionID, dir string) []string {
	resp, err := client.CreateTerminal(ctx, &acp.CreateTerminalRequest{
		Command:   "ls",
		Args:      []string{dir},
		SessionID: sid,
	})
	if err != nil {
		return nil
	}
	handle := acp.NewTerminalHandle(resp.TerminalID, sid, client)
	defer handle.Release(ctx)
	handle.WaitForExit(ctx)
	out, err := handle.CurrentOutput(ctx)
	if err != nil || out.Output == "" {
		return nil
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out.Output), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}

// parseFrontmatter extracts skill metadata from SKILL.md content.
func parseFrontmatter(content string) (*Skill, error) {
	fm, _, err := splitFrontmatter(content)
	if err != nil {
		return nil, err
	}
	var s Skill
	if err := yaml.Unmarshal([]byte(fm), &s); err != nil {
		return nil, fmt.Errorf("parsing frontmatter: %w", err)
	}
	if s.Name == "" {
		return nil, errors.New("skill name is required")
	}
	if s.Description == "" {
		return nil, errors.New("skill description is required")
	}
	return &s, nil
}

func splitFrontmatter(content string) (frontmatter, body string, err error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", errors.New("no YAML frontmatter found")
	}
	rest := strings.TrimPrefix(content, "---\n")
	before, after, ok := strings.Cut(rest, "\n---")
	if !ok {
		return "", "", errors.New("unclosed frontmatter")
	}
	return before, after, nil
}

func skillsToXML(skills []Skill) string {
	if len(skills) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, s := range skills {
		sb.WriteString("  <skill>\n")
		fmt.Fprintf(&sb, "    <name>%s</name>\n", template.HTMLEscapeString(s.Name))
		fmt.Fprintf(&sb, "    <description>%s</description>\n", template.HTMLEscapeString(s.Description))
		fmt.Fprintf(&sb, "    <location>%s</location>\n", template.HTMLEscapeString(s.Location))
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>")
	return sb.String()
}
