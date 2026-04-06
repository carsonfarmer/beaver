package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/carsonfarmer/beaver/pkg/agent"
	"github.com/carsonfarmer/beaver/pkg/instructions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/session"
	acp "github.com/ironpark/go-acp"
)

func main() {
	dataDir := flag.String("data", ".beaver", "path to beaver data directory")
	flag.Parse()

	registry, err := llm.LoadRegistry(filepath.Join(*dataDir, "config.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	store := session.NewFileStore(filepath.Join(*dataDir, "sessions"))
	insts := instructions.New()

	a := agent.New(registry, store, insts)
	conn := acp.NewAgentSideConnection(a, os.Stdin, os.Stdout)
	a.SetClient(conn)

	if err := conn.Start(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
