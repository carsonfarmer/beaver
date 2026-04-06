package agent

import (
	"context"
	"strings"

	"github.com/carsonfarmer/beaver/pkg/tools"
	acp "github.com/ironpark/go-acp"
)

// ClientFilesystem adapts an ACP client to the instructions.Filesystem interface.
type ClientFilesystem struct {
	Ctx       context.Context
	SessionID acp.SessionID
	Client    acp.Client
}

func (f *ClientFilesystem) ReadFile(path string) (string, error) {
	resp, err := f.Client.ReadTextFile(f.Ctx, &acp.ReadTextFileRequest{
		Path: path, SessionID: f.SessionID,
	})
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (f *ClientFilesystem) WriteFile(path, content string) error {
	_, err := f.Client.WriteTextFile(f.Ctx, &acp.WriteTextFileRequest{
		Path: path, Content: content, SessionID: f.SessionID,
	})
	return err
}

func (f *ClientFilesystem) ListDir(dir string) []string {
	out, err := tools.RunCommand(f.Ctx, f.Client, f.SessionID, "ls", []string{dir})
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
