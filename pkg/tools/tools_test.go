package tools

import (
	"context"
	"fmt"
	"testing"

	"charm.land/fantasy"
	acp "github.com/ironpark/go-acp"
)

type mockClient struct {
	readContent    string
	readErr        error
	writeErr       error
	terminalOutput string
	terminalErr    error
}

func (m *mockClient) SessionUpdate(_ context.Context, _ *acp.SessionNotification) error { return nil }
func (m *mockClient) RequestPermission(_ context.Context, _ *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	return nil, nil
}
func (m *mockClient) ReadTextFile(_ context.Context, _ *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	if m.readErr != nil {
		return nil, m.readErr
	}
	return &acp.ReadTextFileResponse{Content: m.readContent}, nil
}
func (m *mockClient) WriteTextFile(_ context.Context, _ *acp.WriteTextFileRequest) (*acp.WriteTextFileResponse, error) {
	if m.writeErr != nil {
		return nil, m.writeErr
	}
	return &acp.WriteTextFileResponse{}, nil
}
func (m *mockClient) CreateTerminal(_ context.Context, _ *acp.CreateTerminalRequest) (*acp.CreateTerminalResponse, error) {
	if m.terminalErr != nil {
		return nil, m.terminalErr
	}
	return &acp.CreateTerminalResponse{TerminalID: "term-1"}, nil
}
func (m *mockClient) TerminalOutput(_ context.Context, _ *acp.TerminalOutputRequest) (*acp.TerminalOutputResponse, error) {
	return &acp.TerminalOutputResponse{Output: m.terminalOutput}, nil
}
func (m *mockClient) ReleaseTerminal(_ context.Context, _ *acp.ReleaseTerminalRequest) (*acp.ReleaseTerminalResponse, error) {
	return &acp.ReleaseTerminalResponse{}, nil
}
func (m *mockClient) WaitForTerminalExit(_ context.Context, _ *acp.WaitForTerminalExitRequest) (*acp.WaitForTerminalExitResponse, error) {
	return &acp.WaitForTerminalExitResponse{}, nil
}
func (m *mockClient) KillTerminalCommand(_ context.Context, _ *acp.KillTerminalRequest) (*acp.KillTerminalResponse, error) {
	return &acp.KillTerminalResponse{}, nil
}

func mockToolCall(name, input string) fantasy.ToolCall {
	return fantasy.ToolCall{ID: "test-tc", Name: name, Input: input}
}

func TestRunCommand_Success(t *testing.T) {
	out, err := RunCommand(context.Background(), &mockClient{terminalOutput: "hello\n"}, "s", "echo", []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if out != "hello\n" {
		t.Fatalf("expected %q, got %q", "hello\n", out)
	}
}

func TestRunCommand_Error(t *testing.T) {
	_, err := RunCommand(context.Background(), &mockClient{terminalErr: fmt.Errorf("denied")}, "s", "echo", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestForSession_Names(t *testing.T) {
	tools := ForSession(&mockClient{}, "test-sess")
	want := []string{"read_file", "write_file", "execute"}
	if len(tools) != len(want) {
		t.Fatalf("expected %d tools, got %d", len(want), len(tools))
	}
	for i, name := range want {
		if got := tools[i].Info().Name; got != name {
			t.Errorf("tool[%d]: expected %q, got %q", i, name, got)
		}
	}
}

func TestForSession_ReadFile_Success(t *testing.T) {
	tools := ForSession(&mockClient{readContent: "hello world"}, "test-sess")
	resp, err := tools[0].Run(context.Background(), mockToolCall("read_file", `{"path":"/tmp/test.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
}

func TestForSession_ReadFile_Error(t *testing.T) {
	tools := ForSession(&mockClient{readErr: fmt.Errorf("not found")}, "test-sess")
	resp, err := tools[0].Run(context.Background(), mockToolCall("read_file", `{"path":"/tmp/nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}

func TestForSession_WriteFile_Success(t *testing.T) {
	tools := ForSession(&mockClient{}, "test-sess")
	resp, err := tools[1].Run(context.Background(), mockToolCall("write_file", `{"path":"/tmp/test.txt","content":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
}

func TestForSession_WriteFile_Error(t *testing.T) {
	tools := ForSession(&mockClient{writeErr: fmt.Errorf("permission denied")}, "test-sess")
	resp, err := tools[1].Run(context.Background(), mockToolCall("write_file", `{"path":"/tmp/test.txt","content":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}

func TestForSession_Execute_Success(t *testing.T) {
	tools := ForSession(&mockClient{terminalOutput: "hello\n"}, "test-sess")
	resp, err := tools[2].Run(context.Background(), mockToolCall("execute", `{"command":"echo","args":["hello"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
}

func TestForSession_Execute_Error(t *testing.T) {
	tools := ForSession(&mockClient{terminalErr: fmt.Errorf("denied")}, "test-sess")
	resp, err := tools[2].Run(context.Background(), mockToolCall("execute", `{"command":"echo","args":["hi"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}

func TestKind(t *testing.T) {
	tests := []struct {
		name string
		want acp.ToolKind
	}{
		{"read_file", acp.ToolKindRead},
		{"write_file", acp.ToolKindEdit},
		{"execute", acp.ToolKindExecute},
		{"unknown", acp.ToolKindOther},
	}
	for _, tt := range tests {
		if got := Kind(tt.name); got != tt.want {
			t.Errorf("Kind(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}
