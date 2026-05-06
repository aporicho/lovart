package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/selftest"
	"github.com/aporicho/lovart/internal/update"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	var online bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run Lovart readiness diagnostics",
		RunE: func(cmd *cobra.Command, args []string) error {
			result := selftest.Run()
			data := map[string]any{
				"status":  result.Status,
				"ready":   result.Status == selftest.StatusReady,
				"version": result.Version,
				"root":    result.Root,
				"checks":  result.Checks,
			}
			if len(result.RecommendedActions) > 0 {
				data["recommended_actions"] = result.RecommendedActions
			}
			if !online {
				printEnvelope(okLocal(data, true))
				return nil
			}
			updateStatus, err := update.Check(context.Background())
			if err != nil {
				data["online"] = map[string]any{
					"status": "network_unavailable",
					"error":  err.Error(),
					"recommended_actions": []string{
						"check network connectivity to www.lovart.ai",
						"rerun `lovart doctor --online`",
					},
				}
				printEnvelope(okPreflight(data, true))
				return nil
			}
			data["online"] = updateStatus
			printEnvelope(okPreflight(data, true))
			return nil
		},
	}
	cmd.Flags().BoolVar(&online, "online", false, "also check Lovart network/update status")
	return cmd
}
