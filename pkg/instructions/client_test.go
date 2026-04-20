package instructions

import (
	"context"
	"fmt"
	"strings"
	"testing"

	acp "github.com/ironpark/go-acp"
)

// mockClient is a minimal acp.Client for instruction discovery tests.
type mockClient struct {
	files   map[string]string
	termOut string
}

func (m *mockClient) SessionUpdate(context.Context, *acp.SessionNotification) error {
	return nil
}

func (m *mockClient) RequestPermission(context.Context, *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	return nil, nil
}

func (m *mockClient) ReadTextFile(_ context.Context, req *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	if c, ok := m.files[req.Path]; ok {
		return &acp.ReadTextFileResponse{Content: c}, nil
	}
	return nil, fmt.Errorf("not found: %s", req.Path)
}

func (m *mockClient) WriteTextFile(context.Context, *acp.WriteTextFileRequest) (*acp.WriteTextFileResponse, error) {
	return &acp.WriteTextFileResponse{}, nil
}

func (m *mockClient) CreateTerminal(context.Context, *acp.CreateTerminalRequest) (*acp.CreateTerminalResponse, error) {
	return &acp.CreateTerminalResponse{TerminalID: "t"}, nil
}

func (m *mockClient) TerminalOutput(context.Context, *acp.TerminalOutputRequest) (*acp.TerminalOutputResponse, error) {
	return &acp.TerminalOutputResponse{Output: m.termOut}, nil
}

func (m *mockClient) ReleaseTerminal(context.Context, *acp.ReleaseTerminalRequest) (*acp.ReleaseTerminalResponse, error) {
	return &acp.ReleaseTerminalResponse{}, nil
}

func (m *mockClient) WaitForTerminalExit(context.Context, *acp.WaitForTerminalExitRequest) (*acp.WaitForTerminalExitResponse, error) {
	return &acp.WaitForTerminalExitResponse{}, nil
}

func (m *mockClient) KillTerminalCommand(context.Context, *acp.KillTerminalRequest) (*acp.KillTerminalResponse, error) {
	return &acp.KillTerminalResponse{}, nil
}

func newClientFS(t *testing.T, c acp.Client) *clientFS {
	t.Helper()
	return &clientFS{ctx: context.Background(), client: c, sid: "test"}
}

func TestClientFS_ReadFile(t *testing.T) {
	fs := newClientFS(t, &mockClient{files: map[string]string{"/a.txt": "hello"}})
	got, err := fs.ReadFile("/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestClientFS_ReadFile_Error(t *testing.T) {
	fs := newClientFS(t, &mockClient{files: map[string]string{}})
	if _, err := fs.ReadFile("/nope"); err == nil {
		t.Fatal("expected error")
	}
}

func TestClientFS_ListDir(t *testing.T) {
	fs := newClientFS(t, &mockClient{termOut: "dir-a\ndir-b\n"})
	got := fs.ListDir("/project")
	if len(got) != 2 || got[0] != "dir-a" || got[1] != "dir-b" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestClientFS_ListDir_Empty(t *testing.T) {
	fs := newClientFS(t, &mockClient{})
	if len(fs.ListDir("/nope")) != 0 {
		t.Fatal("expected empty")
	}
}

func TestDiscover_ViaClient(t *testing.T) {
	// End-to-end exercise of the public Discover signature.
	c := &mockClient{
		files: map[string]string{"/project/AGENTS.md": "# Rules"},
	}
	prompt := New().Discover(context.Background(), "/project", c, "s")
	if !strings.Contains(prompt, "# Rules") {
		t.Fatal("expected context content in prompt")
	}
}
