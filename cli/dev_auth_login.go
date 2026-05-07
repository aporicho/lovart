package cli

import (
	"context"
	"time"

	"github.com/aporicho/lovart/internal/devauth"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/spf13/cobra"
)

func newDevAuthLoginCmd() *cobra.Command {
	var timeoutSeconds float64
	var debugPort int
	cmd := &cobra.Command{
		Use:   "auth-login",
		Short: "Developer login by restarting Chrome and capturing Lovart browser auth",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if timeoutSeconds <= 0 {
				printEnvelope(envelope.Err(errors.CodeInputError, "timeout must be positive", nil))
				return nil
			}
			result, err := devauth.Run(context.Background(), devauth.Options{
				Timeout:       time.Duration(timeoutSeconds * float64(time.Second)),
				DebugPort:     debugPort,
				RestartChrome: true,
			})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "dev auth login failed", map[string]any{
					"error": err.Error(),
					"recommended_actions": []string{
						"make sure Google Chrome is installed",
						"sign in to Lovart in the Chrome window opened by this command",
						"rerun `lovart dev auth-login`",
					},
				}))
				return nil
			}
			printEnvelope(okPreflight(result, false))
			return nil
		},
	}
	cmd.Flags().Float64Var(&timeoutSeconds, "timeout-seconds", devauth.DefaultTimeout.Seconds(), "seconds to wait for browser capture and validation")
	cmd.Flags().IntVar(&debugPort, "debug-port", devauth.DefaultDebugPort, "local Chrome DevTools debugging port")
	return cmd
}
