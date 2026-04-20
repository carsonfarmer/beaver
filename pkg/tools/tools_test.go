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
	sessionUpdate  func(*acp.SessionNotification) error
	updates        []*acp.SessionNotification
}

func (m *mockClient) SessionUpdate(_ context.Context, n *acp.SessionNotification) error {
	m.updates = append(m.updates, n)
	if m.sessionUpdate != nil {
		return m.sessionUpdate(n)
	}
	return nil
}
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

// testCtx returns a context with a session ID set for tool tests.
func testCtx() context.Context {
	return WithSessionID(context.Background(), "test-sess")
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

func TestToolKinds(t *testing.T) {
	wantKinds := map[string]acp.ToolKind{
		ReadName:    acp.ToolKindRead,
		WriteName:   acp.ToolKindEdit,
		ExecuteName: acp.ToolKindExecute,
		PlanName:    acp.ToolKindThink,
	}
	for name, want := range wantKinds {
		if got := ToolKinds[name]; got != want {
			t.Errorf("ToolKinds[%q] = %q, want %q", name, got, want)
		}
	}
}

func TestReadFile_Success(t *testing.T) {
	tool := NewReadFileTool(&mockClient{readContent: "hello world"})
	resp, err := tool.Run(testCtx(), mockToolCall(ReadName, `{"path":"/tmp/test.txt"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
}

func TestReadFile_Error(t *testing.T) {
	tool := NewReadFileTool(&mockClient{readErr: fmt.Errorf("not found")})
	resp, err := tool.Run(testCtx(), mockToolCall(ReadName, `{"path":"/tmp/nope"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}

func TestWriteFile_Success(t *testing.T) {
	tool := NewWriteFileTool(&mockClient{})
	resp, err := tool.Run(testCtx(), mockToolCall(WriteName, `{"path":"/tmp/test.txt","content":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
}

func TestWriteFile_Error(t *testing.T) {
	tool := NewWriteFileTool(&mockClient{writeErr: fmt.Errorf("permission denied")})
	resp, err := tool.Run(testCtx(), mockToolCall(WriteName, `{"path":"/tmp/test.txt","content":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}

func TestExecute_Success(t *testing.T) {
	tool := NewExecuteTool(&mockClient{terminalOutput: "hello\n"})
	resp, err := tool.Run(testCtx(), mockToolCall(ExecuteName, `{"command":"echo","args":["hello"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
}

func TestExecute_Error(t *testing.T) {
	tool := NewExecuteTool(&mockClient{terminalErr: fmt.Errorf("denied")})
	resp, err := tool.Run(testCtx(), mockToolCall(ExecuteName, `{"command":"echo","args":["hi"]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}

func TestPlan_EmitsInput(t *testing.T) {
	client := &mockClient{}
	tool := NewPlanTool(client)
	input := `{"entries":[{"content":"step 1","priority":"high","status":"pending"},{"content":"step 2","priority":"medium","status":"in_progress"}]}`
	resp, err := tool.Run(testCtx(), mockToolCall(PlanName, input))
	if err != nil {
		t.Fatal(err)
	}
	if resp.IsError {
		t.Fatal("expected success")
	}
	if len(client.updates) != 1 {
		t.Fatalf("got %d updates, want 1", len(client.updates))
	}
	plan, ok := client.updates[0].Update.AsPlan()
	if !ok {
		t.Fatalf("update = %+v, want Plan", client.updates[0].Update)
	}
	if len(plan.Entries) != 2 {
		t.Fatalf("plan has %d entries, want 2", len(plan.Entries))
	}
	if plan.Entries[0].Content != "step 1" || plan.Entries[0].Priority != acp.PlanEntryPriorityHigh {
		t.Fatalf("entry 0 = %+v", plan.Entries[0])
	}
}

func TestPlan_RendersChecklist(t *testing.T) {
	tool := NewPlanTool(&mockClient{})
	input := `{"entries":[{"content":"done task","priority":"high","status":"completed"},{"content":"todo task","priority":"low","status":"pending"}]}`
	resp, err := tool.Run(testCtx(), mockToolCall(PlanName, input))
	if err != nil {
		t.Fatal(err)
	}
	want := "- [x] (high) done task\n- [ ] (low) todo task\n"
	if resp.Content != want {
		t.Fatalf("render =\n%q\nwant\n%q", resp.Content, want)
	}
}

func TestPlan_EmitError(t *testing.T) {
	client := &mockClient{sessionUpdate: func(*acp.SessionNotification) error {
		return fmt.Errorf("send failed")
	}}
	tool := NewPlanTool(client)
	resp, err := tool.Run(testCtx(), mockToolCall(PlanName, `{"entries":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	if !resp.IsError {
		t.Fatal("expected error response")
	}
}
