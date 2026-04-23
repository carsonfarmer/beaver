package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/carsonfarmer/beaver/pkg/agent"
	"github.com/carsonfarmer/beaver/pkg/extensions"
	"github.com/carsonfarmer/beaver/pkg/llm"
	"github.com/carsonfarmer/beaver/pkg/storage"
	"github.com/carsonfarmer/beaver/pkg/tools"
	acp "github.com/ironpark/go-acp"
)

func main() {
	dataDir := flag.String("data", ".beaver", "path to beaver data directory")
	httpAddr := flag.String("http", "", "HTTP address to listen on (e.g. :8080); uses stdio if empty")
	flag.Parse()

	registry, err := llm.LoadRegistry(filepath.Join(*dataDir, "config.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	logStore, err := storage.NewFileArchive(filepath.Join(*dataDir, "sessions"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	a := agent.New(
		agent.WithRegistry(registry),
		agent.WithStorage(logStore),
	)

	if *httpAddr != "" {
		transport := acp.NewHTTPServerTransport()
		conn := acp.NewAgentSideConnection(a, nil, nil, acp.WithTransport(transport))
		a.SetClient(conn)
		a.SetTools(
			tools.NewReadFileTool(conn),
			tools.NewWriteFileTool(conn),
			tools.NewExecuteTool(conn),
			tools.NewPlanTool(conn),
		)
		a.SetProviders(
			extensions.BasePrompt(extensions.DefaultPrompt),
			extensions.AgentsMd(conn),
			extensions.Skills(conn),
		)

		go func() {
			if err := conn.Start(context.Background()); err != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				os.Exit(1)
			}
		}()

		fmt.Fprintf(os.Stderr, "beaver listening on %s\n", *httpAddr)
		if err := http.ListenAndServe(*httpAddr, transport.Handler()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	} else {
		conn := acp.NewAgentSideConnection(a, os.Stdin, os.Stdout)
		a.SetClient(conn)
		a.SetTools(
			tools.NewReadFileTool(conn),
			tools.NewWriteFileTool(conn),
			tools.NewExecuteTool(conn),
			tools.NewPlanTool(conn),
		)
		a.SetProviders(
			extensions.BasePrompt(extensions.DefaultPrompt),
			extensions.AgentsMd(conn),
			extensions.Skills(conn),
		)

		if err := conn.Start(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}
