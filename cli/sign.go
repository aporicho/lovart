package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/spf13/cobra"
)

func newSignCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sign",
		Short: "Test the WASM signer with a generated signature",
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			signer, err := signing.NewSigner()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeSignerStale, "failed to create signer", map[string]any{"error": err.Error()}))
				return nil
			}
			defer signer.(wazeroSignerAdapter).Close(context.Background())

			if err := signer.Health(); err != nil {
				printEnvelope(envelope.Err(errors.CodeSignerStale, "signer unhealthy", map[string]any{"error": err.Error()}))
				return nil
			}

			timestamp := signing.TimestampNowMS(0)
			reqUUID := signing.RandomHex(32)

			result, err := signer.Sign(context.Background(), signing.SigningPayload{
				Timestamp: timestamp,
				ReqUUID:   reqUUID,
			})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeSignerStale, "sign failed", map[string]any{"error": err.Error()}))
				return nil
			}

			printEnvelope(envelope.OK(map[string]any{
				"signer":    "wazero+wasm",
				"healthy":   true,
				"signature": result.Signature,
				"headers":   result.Headers(),
				"timestamp": timestamp,
				"req_uuid":  reqUUID,
			}))
			return nil
		},
	}
}

func printEnvelope(e envelope.Envelope) {
	b, _ := json.Marshal(e)
	fmt.Println(string(b))
}

// wazeroSignerAdapter exposes Close() on the signer for cleanup.
type wazeroSignerAdapter interface {
	signing.Signer
	Close(ctx context.Context) error
}
