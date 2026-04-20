package instructions

import (
	"fmt"
	"path/filepath"
	"strings"
)

// DefaultContextFiles are the file names searched for in cwd and cwd/.agents.
var DefaultContextFiles = []string{
	"AGENTS.md",
	"agents.md",
	"Agents.md",
}

// contextFile holds the content of a discovered AGENTS.md file.
type contextFile struct {
	Path    string
	Content string
}

// discoverContext checks cwd and cwd/.agents for context files.
func discoverContext(cwd string, names []string, fs fs) []contextFile {
	if cwd == "" {
		return nil
	}
	var files []contextFile
	for _, dir := range []string{cwd, filepath.Join(cwd, ".agents")} {
		for _, name := range names {
			path := filepath.Join(dir, name)
			content, err := fs.ReadFile(path)
			if err != nil || content == "" {
				continue
			}
			files = append(files, contextFile{Path: path, Content: content})
			break // only one variant per directory
		}
	}
	return files
}

func contextToXML(files []contextFile) string {
	if len(files) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("<context>\n")
	for _, f := range files {
		fmt.Fprintf(&sb, "<file path=%q>\n", f.Path)
		sb.WriteString(f.Content)
		if !strings.HasSuffix(f.Content, "\n") {
			sb.WriteByte('\n')
		}
		sb.WriteString("</file>\n")
	}
	sb.WriteString("</context>")
	return sb.String()
}
