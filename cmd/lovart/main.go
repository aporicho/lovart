// Command lovart is the single-binary CLI and MCP server for Lovart generation.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aporicho/lovart/cli"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
)

func main() {
	os.Exit(run())
}

func run() int {
	defer func() {
		if r := recover(); r != nil {
			e := envelope.Err(errors.CodeInternal, fmt.Sprintf("panic: %v", r), nil)
			printAndExit(e, 1)
		}
	}()

	// If first arg is "mcp", start the MCP stdio server.
	if len(os.Args) > 1 && os.Args[1] == "mcp" {
		if err := runMCP(os.Args[2:]); err != nil {
			e := envelope.Err(errors.CodeInternal, err.Error(), nil)
			printAndExit(e, 1)
		}
		return 0
	}

	// Dispatch to cobra CLI command tree.
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		e := envelope.Err(errors.CodeInternal, err.Error(), nil)
		printAndExit(e, 1)
	}
	return 0
}

func printAndExit(e envelope.Envelope, code int) {
	b, _ := json.Marshal(e)
	fmt.Println(string(b))
	os.Exit(code)
}
