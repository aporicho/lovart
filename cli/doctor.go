package cli

import (
	"context"

	"github.com/aporicho/lovart/internal/metadata"
	"github.com/aporicho/lovart/internal/paths"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run architecture integrity checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := map[string]any{
				"signer":   checkSigner(),
				"metadata": checkMetadata(),
				"paths": map[string]string{
					"signer_wasm":       paths.SignerWASMFile,
					"signer_manifest":   paths.SignerManifestFile,
					"metadata_manifest": paths.MetadataManifestFile,
					"generator_list":    paths.GeneratorListFile,
					"generator_schema":  paths.GeneratorSchemaFile,
				},
			}
			status := "ok"
			recommended := recommendedDoctorActions()
			if len(recommended) > 0 {
				status = "needs_setup"
				checks["recommended_actions"] = recommended
			}
			checks["status"] = status
			printEnvelope(okLocal(checks, true))
			return nil
		},
	}
}

func recommendedDoctorActions() []string {
	var actions []string
	s, err := signing.NewSigner()
	if err != nil {
		actions = append(actions, "run `lovart update sync --all`")
	} else {
		if closer, ok := s.(interface{ Close(context.Context) error }); ok {
			_ = closer.Close(context.Background())
		}
	}
	if _, err := metadata.ReadManifest(); err != nil && len(actions) == 0 {
		actions = append(actions, "run `lovart update sync --all`")
	}
	return actions
}
