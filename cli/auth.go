package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage Lovart browser login",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(newAuthStatusCmd())
	cmd.AddCommand(newAuthLoginCmd())
	cmd.AddCommand(newAuthLogoutCmd())
	return cmd
}

func newAuthStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show Lovart auth status without exposing secrets",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			printEnvelope(okLocal(auth.GetStatus(), true))
			return nil
		},
	}
}

func newAuthLoginCmd() *cobra.Command {
	var timeoutSeconds float64
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Connect Lovart browser login through the Lovart Connector extension",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeoutSeconds <= 0 {
				printEnvelope(envelope.Err(errors.CodeInputError, "timeout must be positive", nil))
				return nil
			}
			timeout := time.Duration(timeoutSeconds * float64(time.Second))
			result, err := auth.RunBrowserExtensionLogin(context.Background(), auth.BrowserLoginOptions{
				Timeout:     timeout,
				OpenBrowser: openBrowser,
				BeforeOpen: func(result auth.BrowserLoginResult) {
					fmt.Fprintf(os.Stderr, "Lovart auth login waiting on http://127.0.0.1:%d\n", result.CallbackPort)
					fmt.Fprintln(os.Stderr, "Opening Lovart in Google Chrome. Stay signed in, then click Connect in the Lovart Connector page prompt.")
				},
				OnOpenError: func(result auth.BrowserLoginResult) {
					fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\nOpen manually: %s\n", result.OpenError, result.LoginURL)
				},
			})
			if err != nil {
				printEnvelope(authLoginErrorEnvelope(err))
				return nil
			}
			printEnvelope(okLocal(map[string]any{
				"authenticated": true,
				"status":        result.Status,
				"next_steps":    result.NextSteps,
			}))
			return nil
		},
	}
	cmd.Flags().Float64Var(&timeoutSeconds, "timeout-seconds", 300, "seconds to wait for browser connection")
	return cmd
}

func newAuthLogoutCmd() *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Remove stored Lovart credentials",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				printEnvelope(envelope.Err(errors.CodeInputError, "logout requires --yes", map[string]any{
					"recommended_actions": []string{"rerun `lovart auth logout --yes`"},
				}))
				return nil
			}
			if err := auth.Delete(); err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "delete auth credentials", map[string]any{"error": err.Error()}))
				return nil
			}
			printEnvelope(okLocal(map[string]any{"logged_out": true, "status": auth.GetStatus()}))
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm deleting stored Lovart credentials")
	return cmd
}

func authLoginErrorEnvelope(err error) envelope.Envelope {
	if lovartErr, ok := err.(*errors.LovartError); ok {
		return envelope.Err(lovartErr.Code, lovartErr.Message, lovartErr.Details)
	}
	return envelope.Err(errors.CodeInternal, "auth login failed", map[string]any{"error": err.Error()})
}
