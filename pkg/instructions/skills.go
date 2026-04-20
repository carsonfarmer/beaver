package instructions

import (
	"errors"
	"fmt"
	"html/template"
	"path/filepath"
	"strings"

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

// discoverSkills finds skills in the given directory under cwd.
func discoverSkills(cwd, skillsDir string, fs fs) []Skill {
	if cwd == "" {
		return nil
	}
	dir := filepath.Join(cwd, skillsDir)
	var found []Skill
	for _, name := range fs.ListDir(dir) {
		loc := filepath.Join(dir, name, "SKILL.md")
		content, err := fs.ReadFile(loc)
		if err != nil {
			continue
		}
		skill, err := parseFrontmatter(content)
		if err != nil {
			continue
		}
		skill.Location = loc
		found = append(found, *skill)
	}
	return found
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
