package cli

import (
	"github.com/aporicho/lovart/internal/connector"
	"github.com/aporicho/lovart/internal/envelope"
	lovarterrors "github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newExtensionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "extension",
		Short: "Manage the Lovart Connector browser extension files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newExtensionStatusCmd())
	cmd.AddCommand(newExtensionInstallCmd())
	cmd.AddCommand(newExtensionOpenCmd())
	return cmd
}

func newExtensionStatusCmd() *cobra.Command {
	var extensionDir string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Lovart Connector extension file status",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := connector.Status(connector.Options{ExtensionDir: extensionDir})
			printEnvelope(connectorEnvelope(result, err))
			return nil
		},
	}
	cmd.Flags().StringVar(&extensionDir, "extension-dir", "", "extension install directory")
	return cmd
}

func newExtensionInstallCmd() *cobra.Command {
	var opts connector.Options
	var yes bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Prepare Lovart Connector extension files and optionally open Chrome extensions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !opts.DryRun && !yes {
				printEnvelope(envelope.Err(lovarterrors.CodeInputError, "extension install requires --yes", map[string]any{
					"recommended_actions": []string{"rerun with --dry-run to preview", "rerun with --yes to install"},
				}))
				return nil
			}
			opts.OpenURL = openBrowser
			result, err := connector.Install(opts)
			printEnvelope(connectorEnvelope(result, err))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.SourceDir, "source-dir", "", "source unpacked extension directory")
	cmd.Flags().StringVar(&opts.ExtensionDir, "extension-dir", "", "extension install directory")
	cmd.Flags().BoolVar(&opts.DryRun, "dry-run", false, "preview changes without writing files")
	cmd.Flags().BoolVar(&opts.Open, "open", false, "open chrome://extensions after preparing files")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm extension file installation")
	return cmd
}

func newExtensionOpenCmd() *cobra.Command {
	var extensionDir string
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Open Chrome extension management for Lovart Connector",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := connector.Open(connector.Options{ExtensionDir: extensionDir, OpenURL: openBrowser})
			printEnvelope(connectorEnvelope(result, err))
			return nil
		},
	}
	cmd.Flags().StringVar(&extensionDir, "extension-dir", "", "extension install directory")
	return cmd
}

func connectorEnvelope(result connector.Result, err error) envelope.Envelope {
	if err == nil {
		return okLocal(result, true)
	}
	if lovartErr, ok := err.(*lovarterrors.LovartError); ok {
		return envelope.Err(lovartErr.Code, lovartErr.Message, lovartErr.Details)
	}
	return envelope.Err(lovarterrors.CodeInternal, "extension operation failed", map[string]any{"error": err.Error()})
}
