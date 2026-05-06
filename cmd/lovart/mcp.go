package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/mcp"
)

// runMCP starts the MCP stdio JSON-RPC server.
func runMCP(args []string) error {
	if len(args) == 0 {
		return mcp.NewServer().Run(context.Background(), os.Stdin, os.Stdout)
	}
	switch args[0] {
	case "status":
		envelope.PrintJSON(mcp.Status())
		return nil
	default:
		return fmt.Errorf("unknown mcp command: %s", args[0])
	}
}
