// Package instructions discovers project context (AGENTS.md files and skills)
// and builds the system prompt. See https://agents.md and https://agentskills.io
package instructions

import "fmt"

// DefaultPrompt is the default base system prompt.
const DefaultPrompt = "You are a helpful coding assistant. Use the provided tools to read files, write files, and execute commands."

// Filesystem provides file access for discovering context and skills.
type Filesystem interface {
	ReadFile(path string) (string, error)
	WriteFile(path, content string) error
	ListDir(dir string) []string
}

// Instructions configures how to discover and build the system prompt.
type Instructions struct {
	basePrompt   string
	contextFiles []string
	skillsDirs   []string
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

// Discover reads context files and skills from the filesystem,
// then assembles and returns the complete system prompt.
func (i *Instructions) Discover(cwd string, fs Filesystem) string {
	context := discoverContext(cwd, i.contextFiles, fs)
	var skills []Skill
	for _, dir := range i.skillsDirs {
		skills = append(skills, discoverSkills(cwd, dir, fs)...)
	}
	return systemPrompt(i.basePrompt, cwd, context, skills)
}

// systemPrompt builds a complete system prompt from the base prompt,
// working directory, context files, and available skills.
func systemPrompt(basePrompt, cwd string, context []contextFile, skills []Skill) string {
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
	return prompt
}
