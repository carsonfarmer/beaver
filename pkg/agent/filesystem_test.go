package agent

import (
	"context"
	"testing"
)

func TestClientFilesystem_ReadFile(t *testing.T) {
	fs := &ClientFilesystem{
		Ctx: context.Background(), SessionID: "s",
		Client: &mockClient{files: map[string]string{"/a.txt": "hello"}},
	}
	got, err := fs.ReadFile("/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestClientFilesystem_ReadFile_Error(t *testing.T) {
	fs := &ClientFilesystem{
		Ctx: context.Background(), SessionID: "s",
		Client: &mockClient{files: map[string]string{}},
	}
	_, err := fs.ReadFile("/nope")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientFilesystem_WriteFile(t *testing.T) {
	fs := &ClientFilesystem{
		Ctx: context.Background(), SessionID: "s",
		Client: &mockClient{},
	}
	if err := fs.WriteFile("/a.txt", "content"); err != nil {
		t.Fatal(err)
	}
}

func TestClientFilesystem_ListDir(t *testing.T) {
	fs := &ClientFilesystem{
		Ctx: context.Background(), SessionID: "s",
		Client: &mockClient{termOut: "dir-a\ndir-b\n"},
	}
	got := fs.ListDir("/project")
	if len(got) != 2 || got[0] != "dir-a" || got[1] != "dir-b" {
		t.Fatalf("unexpected: %v", got)
	}
}

func TestClientFilesystem_ListDir_Empty(t *testing.T) {
	fs := &ClientFilesystem{
		Ctx: context.Background(), SessionID: "s",
		Client: &mockClient{},
	}
	if len(fs.ListDir("/nope")) != 0 {
		t.Fatal("expected empty")
	}
}
