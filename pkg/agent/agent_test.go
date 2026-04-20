package agent

import (
	"context"
	"testing"

	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	acp "github.com/ironpark/go-acp"
)

func TestSetClient(t *testing.T) {
	a := newTestAgent(t)
	mc := &mockClient{}
	a.SetClient(mc)
	if a.client != mc {
		t.Fatal("expected client to be set")
	}
}

func TestConfigOptions(t *testing.T) {
	reg := newTestRegistry()
	opts := llm.SessionOptions(reg, "test/model", string(llm.ThoughtMedium))
	if len(opts) != 2 {
		t.Fatalf("expected 2, got %d", len(opts))
	}
}

func TestInitialize(t *testing.T) {
	resp, err := newTestAgent(t).Initialize(context.Background(), &acp.InitializeRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if resp.AgentInfo.Name != "beaver" {
		t.Fatalf("expected %q, got %q", "beaver", resp.AgentInfo.Name)
	}
}

func TestAuthenticate(t *testing.T) {
	_, err := newTestAgent(t).Authenticate(context.Background(), &acp.AuthenticateRequest{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetSessionConfigOption_Model(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := createTestSession(t, a, "/tmp")
	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: id, ConfigID: llm.SessionConfigModel, Value: "openai/gpt-4.1-mini",
	})
	if err != nil {
		t.Fatal(err)
	}
	s, ok := a.cachedSession(id)
	if !ok {
		t.Fatal("session not found")
	}
	if s.Model != "openai/gpt-4.1-mini" {
		t.Fatalf("expected model %q, got %q", "openai/gpt-4.1-mini", s.Model)
	}
}

func TestSetSessionConfigOption_ThoughtLevel(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := createTestSession(t, a, "/tmp")
	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: id, ConfigID: llm.SessionConfigThoughtLevel, Value: "high",
	})
	if err != nil {
		t.Fatal(err)
	}
	s, _ := a.cachedSession(id)
	if s.ThoughtLevel != string(llm.ThoughtHigh) {
		t.Fatalf("expected %q, got %q", llm.ThoughtHigh, s.ThoughtLevel)
	}
}

func TestSetSessionConfigOption_SessionNotFound(t *testing.T) {
	a := newTestAgent(t)
	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: "nonexistent", ConfigID: llm.SessionConfigModel, Value: "openai/gpt-4.1",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSetSessionConfigOption_UnknownConfigID(t *testing.T) {
	a := newTestAgent(t)
	a.SetClient(&mockClient{})
	id := acp.SessionID("test-sess")
	setupSession(t, a, id, &session.Session{Model: "openai/gpt-4.1"})

	_, err := a.SetSessionConfigOption(context.Background(), &acp.SetSessionConfigOptionRequest{
		SessionID: id, ConfigID: "unknown", Value: "whatever",
	})
	if err != nil {
		t.Fatal("unknown config IDs should not error")
	}
	s, _ := a.cachedSession(id)
	if s.Model != "openai/gpt-4.1" {
		t.Fatal("model should be unchanged")
	}
}
