// Package instructions discovers project context (AGENTS.md files and skills)
// and builds the system prompt. See https://agents.md and https://agentskills.io
package instructions

import (
	"context"
	"fmt"

	acp "github.com/ironpark/go-acp"
)

// DefaultPrompt is the default base system prompt.
const DefaultPrompt = "You are a helpful coding assistant. Use the provided tools to read files, write files, and execute commands."

// Section is a named block of content appended to the system prompt.
// Extensions contribute sections to provide the LLM with additional context.
type Section struct {
	Content string
}

// SectionFunc returns a section to include in the system prompt.
// Return an empty Content to skip.
type SectionFunc func() Section

// Instructions configures how to discover and build the system prompt.
type Instructions struct {
	basePrompt   string
	contextFiles []string
	skillsDirs   []string
	sections     []SectionFunc
}

// Option configures an Instructions value.
type Option func(*Instructions)

// New creates Instructions with the given options.
// Defaults: DefaultPrompt, DefaultContextFiles, DefaultSkillsDir.
func New(opts ...Option) *Instructions {
	i := &Instructions{
		basePrompt:   DefaultPrompt,
		contextFiles: DefaultContextFiles,
		skillsDirs:   []string{DefaultSkillsDir},
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

// WithBasePrompt sets the base system prompt.
func WithBasePrompt(p string) Option {
	return func(i *Instructions) { i.basePrompt = p }
}

// WithContextFiles sets the filenames to search for (e.g. "AGENTS.md").
func WithContextFiles(names ...string) Option {
	return func(i *Instructions) { i.contextFiles = names }
}

// WithSkillsDirs sets the skills directories relative to cwd.
func WithSkillsDirs(dirs ...string) Option {
	return func(i *Instructions) { i.skillsDirs = dirs }
}

// WithSections adds dynamic section providers to the system prompt.
func WithSections(fns ...SectionFunc) Option {
	return func(i *Instructions) { i.sections = append(i.sections, fns...) }
}

// Discover reads context files and skills via the ACP client, then
// assembles and returns the complete system prompt.
func (i *Instructions) Discover(ctx context.Context, cwd string, client acp.Client, sid acp.SessionID) string {
	return i.discover(cwd, &clientFS{ctx: ctx, client: client, sid: sid})
}

// discover is the internal entry point used by both Discover and tests.
func (i *Instructions) discover(cwd string, fs fs) string {
	context := discoverContext(cwd, i.contextFiles, fs)
	var skills []Skill
	for _, dir := range i.skillsDirs {
		skills = append(skills, discoverSkills(cwd, dir, fs)...)
	}

	// Collect extension sections.
	var sections []Section
	for _, fn := range i.sections {
		if s := fn(); s.Content != "" {
			sections = append(sections, s)
		}
	}

	return systemPrompt(i.basePrompt, cwd, context, skills, sections)
}

// systemPrompt builds a complete system prompt from the base prompt,
// working directory, context files, available skills, and extension sections.
func systemPrompt(basePrompt, cwd string, context []contextFile, skills []Skill, sections []Section) string {
	prompt := basePrompt
	if cwd != "" {
		prompt += fmt.Sprintf(" The working directory is %s. Always use absolute paths when reading or writing files.", cwd)
	}
	if xml := contextToXML(context); xml != "" {
		prompt += "\n\n" + xml
	}
	if xml := skillsToXML(skills); xml != "" {
		prompt += "\n\n" + xml + "\n\n" + skillsUsage
	}
	for _, s := range sections {
		prompt += "\n\n" + s.Content
	}
	return prompt
}
