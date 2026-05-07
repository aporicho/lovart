package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
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
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			server, err := auth.StartLoginServer(ctx, auth.LoginServerOptions{Timeout: timeout})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "start auth login server", map[string]any{"error": err.Error()}))
				return nil
			}
			defer func() {
				closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
				defer closeCancel()
				_ = server.Close(closeCtx)
			}()

			loginURL := "https://www.lovart.ai/?lovart_cli_auth=1&port=" + strconv.Itoa(server.Port())
			fmt.Fprintf(os.Stderr, "Lovart auth login waiting on http://127.0.0.1:%d\n", server.Port())
			fmt.Fprintln(os.Stderr, "Open Lovart, stay signed in, then click Connect in the Lovart Connector page prompt.")
			if err := openBrowser(loginURL); err != nil {
				fmt.Fprintf(os.Stderr, "Could not open browser automatically: %v\nOpen manually: %s\n", err, loginURL)
			}

			select {
			case session := <-server.Result():
				session.Source = auth.LoginSourceBrowserExtension
				if err := auth.SaveSession(session); err != nil {
					printEnvelope(envelope.Err(errors.CodeInternal, "save auth session", map[string]any{"error": err.Error()}))
					return nil
				}
				printEnvelope(okLocal(map[string]any{
					"authenticated": true,
					"status":        auth.GetStatus(),
					"next_steps": []string{
						"lovart doctor",
						"lovart project list",
						"lovart project select <project_id>",
					},
				}))
				return nil
			case <-server.Cancelled():
				printEnvelope(envelope.Err(errors.CodeInputError, "auth login cancelled", nil))
				return nil
			case <-ctx.Done():
				printEnvelope(envelope.Err(errors.CodeTimeout, "auth login timed out", map[string]any{
					"recommended_actions": []string{"rerun `lovart auth login`", "run `lovart dev auth-login` for developer browser capture"},
				}))
				return nil
			}
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

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}
