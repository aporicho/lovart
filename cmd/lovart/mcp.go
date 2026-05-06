package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/mcp"
)

// runMCP starts the MCP stdio JSON-RPC server.
func runMCP(args []string) error {
	if len(args) == 0 {
		return mcp.NewServer().Run(context.Background(), os.Stdin, os.Stdout)
	}
	switch args[0] {
	case "status":
		opts, err := parseMCPConfigFlags("lovart mcp status", args[1:], false)
		if err != nil {
			envelope.PrintJSON(envelope.Err(errors.CodeInputError, err.Error(), nil))
			return nil
		}
		envelope.PrintJSON(mcp.Status(opts))
		return nil
	case "install":
		opts, err := parseMCPConfigFlags("lovart mcp install", args[1:], true)
		if err != nil {
			envelope.PrintJSON(envelope.Err(errors.CodeInputError, err.Error(), nil))
			return nil
		}
		envelope.PrintJSON(mcp.Install(opts))
		return nil
	default:
		envelope.PrintJSON(envelope.Err(errors.CodeInputError, fmt.Sprintf("unknown mcp command: %s", args[0]), map[string]any{
			"valid_commands": []string{"status", "install"},
		}))
		return nil
	}
}

func parseMCPConfigFlags(name string, args []string, install bool) (mcp.ConfigOptions, error) {
	var opts mcp.ConfigOptions
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&opts.Clients, "clients", "auto", "MCP clients: auto, all, none, or comma-separated list")
	flags.StringVar(&opts.LovartPath, "lovart-path", "", "path to lovart binary for client configs")
	flags.StringVar(&opts.Home, "home", "", "home directory override")
	if install {
		flags.BoolVar(&opts.DryRun, "dry-run", false, "preview changes without writing config")
		flags.BoolVar(&opts.Yes, "yes", false, "confirm client config changes")
		flags.BoolVar(&opts.Force, "force", false, "replace unmanaged existing config")
	}
	if err := flags.Parse(args); err != nil {
		return opts, err
	}
	if flags.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	return opts, nil
}
