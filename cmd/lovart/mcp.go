package main

import (
	"context"
	"os"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/mcp"
	"github.com/spf13/cobra"
)

func newMCPCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run or configure the Lovart MCP server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return mcp.NewServer().Run(context.Background(), os.Stdin, os.Stdout)
		},
	}
	cmd.AddCommand(newMCPStatusCommand())
	cmd.AddCommand(newMCPInstallCommand())
	return cmd
}

func newMCPStatusCommand() *cobra.Command {
	var opts mcp.ConfigOptions
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Inspect Lovart MCP server and client configuration",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envelope.PrintJSON(mcp.Status(opts))
			return nil
		},
	}
	addMCPConfigFlags(cmd, &opts)
	return cmd
}

func newMCPInstallCommand() *cobra.Command {
	var opts mcp.ConfigOptions
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Configure local MCP clients to run lovart mcp",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			envelope.PrintJSON(mcp.Install(opts))
			return nil
		},
	}
	addMCPConfigFlags(cmd, &opts)
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview changes without writing config")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "confirm client config changes")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "replace unmanaged existing config")
	return cmd
}

func addMCPConfigFlags(cmd *cobra.Command, opts *mcp.ConfigOptions) {
	cmd.Flags().StringVar(&opts.Clients, "clients", "auto", "MCP clients: auto, all, none, or comma-separated list")
	cmd.Flags().StringVar(&opts.LovartPath, "lovart-path", "", "path to lovart binary for client configs")
	cmd.Flags().StringVar(&opts.Home, "home", "", "home directory override")
}
