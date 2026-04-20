package instructions

import (
	"context"
	"strings"

	"github.com/carsonfarmer/beaver/pkg/tools"
	acp "github.com/ironpark/go-acp"
)

// fs is the minimal file-access shape used by discovery helpers.
// Only internal — callers of Discover pass an acp.Client directly.
type fs interface {
	ReadFile(path string) (string, error)
	ListDir(dir string) []string
}

// clientFS adapts an ACP client into fs by proxying reads and directory
// listings through the client. It is the only way to satisfy fs in
// production; tests construct their own fake fs values.
type clientFS struct {
	ctx    context.Context
	client acp.Client
	sid    acp.SessionID
}

func (f *clientFS) ReadFile(path string) (string, error) {
	resp, err := f.client.ReadTextFile(f.ctx, &acp.ReadTextFileRequest{
		Path: path, SessionID: f.sid,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (f *clientFS) ListDir(dir string) []string {
	out, err := tools.RunCommand(f.ctx, f.client, f.sid, "ls", []string{dir})
	if err != nil || out == "" {
		return nil
	}
	var names []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			names = append(names, line)
		}
	}
	return names
}
