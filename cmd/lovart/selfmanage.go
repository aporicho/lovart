package main

import (
	"context"
	stderrors "errors"
	"runtime"

	"github.com/aporicho/lovart/internal/envelope"
	lovarterrors "github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/selfmanage"
	"github.com/aporicho/lovart/mcp"
	"github.com/spf13/cobra"
)

func newUpgradeCommand() *cobra.Command {
	var opts selfmanage.UpgradeOptions
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the Lovart CLI and connector extension",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := selfmanage.Upgrade(context.Background(), opts)
			if err != nil {
				envelope.PrintJSON(selfManageErrorEnvelope("upgrade failed", err))
				return nil
			}
			envelope.PrintJSON(envelope.OKWithMetadata(result, envelope.ExecutionMetadata{
				ExecutionClass:  "preflight",
				NetworkRequired: true,
				RemoteWrite:     false,
			}))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Repo, "repo", selfmanage.DefaultRepo, "GitHub release repo OWNER/REPO")
	cmd.Flags().StringVar(&opts.Version, "version", "latest", "release version: latest or vX.Y.Z")
	cmd.Flags().StringVar(&opts.InstallPath, "install-path", "", "path to the installed lovart binary")
	cmd.Flags().StringVar(&opts.ExtensionDir, "extension-dir", "", "Lovart Connector extension directory")
	cmd.Flags().BoolVar(&opts.NoExtension, "no-extension", false, "upgrade only the CLI binary")
	cmd.Flags().BoolVar(&opts.CheckOnly, "check", false, "check the latest available release without downloading")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview upgrade targets without downloading")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "confirm upgrading local files")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "reinstall even when the requested version is already installed")
	return cmd
}

func newUninstallCommand() *cobra.Command {
	var (
		opts      selfmanage.UninstallOptions
		clients   string
		keepMCP   bool
		forceMCP  bool
		home      string
		lovartMCP string
	)
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the Lovart CLI, connector extension, and MCP config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.DryRun && !opts.Yes {
				envelope.PrintJSON(envelope.Err(lovarterrors.CodeInputError, "uninstall requires --yes", map[string]any{
					"recommended_actions": []string{"rerun with --dry-run to preview", "rerun with --yes to uninstall"},
				}))
				return nil
			}
			previewOpts := opts
			previewOpts.DryRun = true
			localPreview, err := selfmanage.Uninstall(previewOpts)
			if err != nil {
				envelope.PrintJSON(selfManageErrorEnvelope("uninstall failed", err))
				return nil
			}
			if runtime.GOOS == "windows" && !opts.DryRun {
				result, err := selfmanage.Uninstall(opts)
				if err != nil {
					envelope.PrintJSON(selfManageErrorEnvelope("uninstall failed", err))
					return nil
				}
				envelope.PrintJSON(okLocalMain(map[string]any{"uninstall": result}, true))
				return nil
			}

			var mcpData any
			if !keepMCP {
				mcpEnv := mcp.Uninstall(mcp.ConfigOptions{
					Clients:    clients,
					LovartPath: lovartMCP,
					Home:       home,
					DryRun:     opts.DryRun,
					Yes:        opts.Yes,
					Force:      forceMCP,
				})
				if !mcpEnv.OK {
					envelope.PrintJSON(mcpEnv)
					return nil
				}
				mcpData = mcpEnv.Data
			}

			localResult := localPreview
			if !opts.DryRun {
				localResult, err = selfmanage.Uninstall(opts)
				if err != nil {
					envelope.PrintJSON(selfManageErrorEnvelope("uninstall failed", err))
					return nil
				}
			}
			envelope.PrintJSON(okLocalMain(map[string]any{
				"uninstall":         localResult,
				"mcp_configuration": mcpData,
				"mcp_skipped":       keepMCP,
			}, true))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.InstallPath, "install-path", "", "path to the installed lovart binary")
	cmd.Flags().StringVar(&opts.ExtensionDir, "extension-dir", "", "Lovart Connector extension directory")
	cmd.Flags().StringVar(&clients, "clients", "auto", "MCP clients: auto, all, none, or comma-separated list")
	cmd.Flags().StringVar(&lovartMCP, "lovart-path", "", "path to lovart binary in MCP configs")
	cmd.Flags().StringVar(&home, "home", "", "home directory override for MCP config files")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview removal without deleting files")
	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "confirm uninstalling local files")
	cmd.Flags().BoolVar(&opts.Data, "data", false, "also delete the Lovart runtime data directory")
	cmd.Flags().BoolVar(&keepMCP, "keep-mcp", false, "keep MCP client configuration")
	cmd.Flags().BoolVar(&opts.KeepExtension, "keep-extension", false, "keep Lovart Connector extension files")
	cmd.Flags().BoolVar(&forceMCP, "force", false, "remove unmanaged Lovart MCP config entries")
	return cmd
}

func selfManageErrorEnvelope(fallback string, err error) envelope.Envelope {
	var lovartErr *lovarterrors.LovartError
	if stderrors.As(err, &lovartErr) {
		return envelope.Err(lovartErr.Code, lovartErr.Message, lovartErr.Details)
	}
	return envelope.Err(lovarterrors.CodeInternal, fallback, map[string]any{"error": err.Error()})
}

func okLocalMain(data any, cacheUsed bool) envelope.Envelope {
	return envelope.OKWithMetadata(data, envelope.ExecutionMetadata{
		ExecutionClass:  "local",
		NetworkRequired: false,
		RemoteWrite:     false,
		CacheUsed:       &cacheUsed,
	})
}
