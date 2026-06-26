// Package main is the entry point for the terraspin binary.
package main

import (
	"flag"
	"fmt"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/terraspin/terraspin/internal/integrations/mcp"
)

// serveCmd runs the MCP server.
func serveCmd(args []string) {
	var transport string
	var port int
	var host string

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	fs.StringVar(&transport, "transport", "stdio", "MCP transport: stdio|sse")
	fs.IntVar(&port, "port", 8080, "SSE port (only used with --transport sse)")
	fs.StringVar(&host, "host", "localhost", "SSE host (only used with --transport sse)")
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, `usage: terraspin serve [flags]

  Run Terraspin as a Model Context Protocol (MCP) server.

Flags:
  --transport string   MCP transport: stdio|sse (default "stdio")
  --port int           SSE port (default 8080, only used with --transport sse)
  --host string        SSE host (default "localhost")
`)
	}
	fs.Parse(args)

	s := mcp.NewServer()

	switch transport {
	case "stdio":
		if err := mcpserver.ServeStdio(s); err != nil {
			fmt.Fprintf(os.Stderr, "error: mcp stdio server: %v\n", err)
			os.Exit(3)
		}
	case "sse":
		sseServer := mcpserver.NewSSEServer(s)
		addr := fmt.Sprintf("%s:%d", host, port)
		fmt.Fprintf(os.Stderr, "Terraspin MCP server listening on %s (SSE)\n", addr)
		if err := sseServer.Start(addr); err != nil {
			fmt.Fprintf(os.Stderr, "error: mcp sse server: %v\n", err)
			os.Exit(3)
		}
	default:
		fmt.Fprintf(os.Stderr, "error: unknown transport %q (use stdio or sse)\n", transport)
		os.Exit(2)
	}
}
