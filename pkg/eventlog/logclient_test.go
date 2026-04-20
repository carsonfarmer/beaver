package eventlog

import (
	"context"
	"testing"

	acp "github.com/ironpark/go-acp"
)

type nullClient struct{ updates []*acp.SessionNotification }

func (c *nullClient) SessionUpdate(_ context.Context, n *acp.SessionNotification) error {
	c.updates = append(c.updates, n)
	return nil
}
func (*nullClient) RequestPermission(context.Context, *acp.RequestPermissionRequest) (*acp.RequestPermissionResponse, error) {
	return nil, nil
}
func (*nullClient) ReadTextFile(context.Context, *acp.ReadTextFileRequest) (*acp.ReadTextFileResponse, error) {
	return nil, nil
}
func (*nullClient) WriteTextFile(context.Context, *acp.WriteTextFileRequest) (*acp.WriteTextFileResponse, error) {
	return nil, nil
}
func (*nullClient) CreateTerminal(context.Context, *acp.CreateTerminalRequest) (*acp.CreateTerminalResponse, error) {
	return nil, nil
}
func (*nullClient) TerminalOutput(context.Context, *acp.TerminalOutputRequest) (*acp.TerminalOutputResponse, error) {
	return nil, nil
}
func (*nullClient) ReleaseTerminal(context.Context, *acp.ReleaseTerminalRequest) (*acp.ReleaseTerminalResponse, error) {
	return nil, nil
}
func (*nullClient) WaitForTerminalExit(context.Context, *acp.WaitForTerminalExitRequest) (*acp.WaitForTerminalExitResponse, error) {
	return nil, nil
}
func (*nullClient) KillTerminalCommand(context.Context, *acp.KillTerminalRequest) (*acp.KillTerminalResponse, error) {
	return nil, nil
}

func TestLoggingClient_ForwardsAndLogs(t *testing.T) {
	store := NewMemStore()
	sid := acp.SessionID("s1")
	store.Create(sid, acp.SessionInfo{SessionID: sid})
	raw := &nullClient{}
	lc := &LoggingClient{Client: raw, Store: store}

	update := acp.NewSessionUpdatePlan([]acp.PlanEntry{{Content: "do it"}})
	if err := lc.SessionUpdate(context.Background(), &acp.SessionNotification{SessionID: sid, Update: update}); err != nil {
		t.Fatal(err)
	}
	if len(raw.updates) != 1 {
		t.Fatalf("client got %d updates, want 1", len(raw.updates))
	}
	log, _ := store.Open(sid)
	events, _ := log.Read(context.Background(), [16]byte{})
	// Primer + plan event.
	if len(events) != 2 {
		t.Fatalf("log has %d events, want 2", len(events))
	}
}

func TestLoggingClient_SkipsChunks(t *testing.T) {
	store := NewMemStore()
	sid := acp.SessionID("s1")
	store.Create(sid, acp.SessionInfo{SessionID: sid})
	lc := &LoggingClient{Client: &nullClient{}, Store: store}

	chunks := []acp.SessionUpdate{
		acp.NewSessionUpdateAgentMessageChunk(acp.NewContentBlockText("hi"), ""),
		acp.NewSessionUpdateAgentThoughtChunk(acp.NewContentBlockText("think"), ""),
	}
	for _, u := range chunks {
		if err := lc.SessionUpdate(context.Background(), &acp.SessionNotification{SessionID: sid, Update: u}); err != nil {
			t.Fatal(err)
		}
	}
	log, _ := store.Open(sid)
	events, _ := log.Read(context.Background(), [16]byte{})
	// Only the primer; chunks are not logged.
	if len(events) != 1 {
		t.Fatalf("log has %d events, want 1 (chunks should be skipped)", len(events))
	}
}
