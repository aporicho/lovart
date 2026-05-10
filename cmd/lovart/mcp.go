package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
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
	cmd.AddCommand(newMCPSmokeCommand())
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

func newMCPSmokeCommand() *cobra.Command {
	const defaultPrompt = "lovart mcp smoke test"
	var (
		opts     mcp.SmokeOptions
		bodyFile string
		prompt   string
	)
	opts.Model = "openai/gpt-image-2"
	opts.Mode = "relax"
	prompt = defaultPrompt

	cmd := &cobra.Command{
		Use:   "smoke",
		Short: "Run an agent-style MCP smoke check",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			body, err := smokeBody(cmd, bodyFile, prompt)
			if err != nil {
				envelope.PrintJSON(envelope.Err(errors.CodeInputError, "read smoke body", map[string]any{"error": err.Error()}))
				return nil
			}
			opts.Body = body
			envelope.PrintJSON(mcp.Smoke(context.Background(), opts))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Model, "model", opts.Model, "model used for config and quote checks")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to request JSON body for quote checks")
	cmd.Flags().StringVar(&prompt, "prompt", prompt, "prompt for the default quote body")
	cmd.Flags().StringVar(&opts.Mode, "mode", opts.Mode, "generation mode for quote checks: auto, fast, relax")
	cmd.Flags().BoolVar(&opts.Submit, "submit", false, "submit a generation request after preflight checks")
	cmd.Flags().BoolVar(&opts.AllowPaid, "allow-paid", false, "allow paid generation when --submit is set")
	cmd.Flags().Float64Var(&opts.MaxCredits, "max-credits", 0, "max credits to spend when --submit is set")
	return cmd
}

func smokeBody(cmd *cobra.Command, bodyFile string, prompt string) (map[string]any, error) {
	if bodyFile != "" {
		if cmd.Flags().Changed("prompt") {
			return nil, fmt.Errorf("--body-file and --prompt are mutually exclusive")
		}
		data, err := os.ReadFile(bodyFile)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", bodyFile, err)
		}
		var body map[string]any
		if err := json.Unmarshal(data, &body); err != nil {
			return nil, fmt.Errorf("parse JSON: %w", err)
		}
		return body, nil
	}
	return map[string]any{"prompt": prompt}, nil
}
