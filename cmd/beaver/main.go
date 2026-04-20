package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/carsonfarmer/beaver/pkg/agent"
	"github.com/carsonfarmer/beaver/pkg/eventlog"
	"github.com/carsonfarmer/beaver/pkg/instructions"
	"github.com/carsonfarmer/beaver/pkg/llm"
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

	logStore := eventlog.NewJSONLStore(filepath.Join(*dataDir, "sessions"))
	insts := instructions.New()
	a := agent.New(registry, logStore, insts)

	if *httpAddr != "" {
		transport := acp.NewHTTPServerTransport()
		conn := acp.NewAgentSideConnection(a, nil, nil, acp.WithTransport(transport))
		a.SetClient(conn)

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
		if err := conn.Start(context.Background()); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}
}
