package extensions

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	acp "github.com/ironpark/go-acp"
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

// AgentsMd returns a provider that discovers AGENTS.md files and contributes
// their contents as a <context> section.
func AgentsMd(client acp.Client, names ...string) Provider {
	if len(names) == 0 {
		names = DefaultContextFiles
	}
	return func(ctx context.Context, cwd string, sid acp.SessionID) (string, error) {
		files := discoverContext(ctx, cwd, names, client, sid)
		return contextToXML(files), nil
	}
}

func discoverContext(ctx context.Context, cwd string, names []string, client acp.Client, sid acp.SessionID) []contextFile {
	if cwd == "" {
		return nil
	}
	var files []contextFile
	for _, dir := range []string{cwd, filepath.Join(cwd, ".agents")} {
		for _, name := range names {
			path := filepath.Join(dir, name)
			resp, err := client.ReadTextFile(ctx, &acp.ReadTextFileRequest{
				Path:      path,
				SessionID: sid,
			})
			if err != nil || resp.Content == "" {
				continue
			}
			files = append(files, contextFile{Path: path, Content: resp.Content})
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
