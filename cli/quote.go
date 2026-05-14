package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/aporicho/lovart/internal/registry"
	"github.com/spf13/cobra"
)

func newQuoteCmd() *cobra.Command {
	var bodyFile string
	var mode string

	cmd := &cobra.Command{
		Use:   "quote <model> --body-file <file> [--mode auto|fast|relax]",
		Short: "Fetch Lovart credit quote for a model request",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			model := args[0]
			ctx := context.Background()

			body, err := loadBodyFile(bodyFile)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "read body file", map[string]any{"error": err.Error()}))
				return nil
			}
			if validation := registry.ValidateRequest(model, body); !validation.OK {
				printEnvelope(envelope.Err(validationErrorCode(validation), "request body failed schema validation", map[string]any{
					"validation":          validation,
					"recommended_actions": validationRecommendedActions(validation),
				}))
				return nil
			}
			if _, err := pricing.NormalizeMode(mode); err != nil {
				printEnvelope(envelope.Err(errors.CodeInputError, "invalid mode", map[string]any{"error": err.Error()}))
				return nil
			}

			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			result, err := pricing.QuoteWithOptions(ctx, client, model, body, pricing.QuoteOptions{Mode: mode})
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "quote failed", map[string]any{"error": err.Error()}))
				return nil
			}

			printEnvelope(okPreflight(map[string]any{
				"price":           result.Price,
				"balance":         result.Balance,
				"price_detail":    result.PriceDetail,
				"normalized_body": result.NormalizedBody,
				"pricing_context": result.PricingContext,
			}))
			return nil
		},
	}

	cmd.Flags().StringVar(&bodyFile, "body-file", "", "path to request JSON file")
	cmd.Flags().StringVar(&mode, "mode", pricing.ModeAuto, "generation mode for pricing: auto, fast, relax")
	return cmd
}

func newBalanceCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "balance",
		Short: "Show current Lovart credit balance",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := newSignedClient()
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()}))
				return nil
			}

			bal, err := pricing.Balance(context.Background(), client)
			if err != nil {
				printEnvelope(envelope.Err(errors.CodeInternal, "fetch balance", map[string]any{"error": err.Error()}))
				return nil
			}

			printEnvelope(okPreflight(map[string]any{
				"balance": bal,
			}))
			return nil
		},
	}
}

func loadBodyFile(path string) (map[string]any, error) {
	if path == "" {
		return nil, fmt.Errorf("--body-file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}
	return body, nil
}

func loadBodyInput(bodyFile string, prompt string) (map[string]any, error) {
	if bodyFile != "" && prompt != "" {
		return nil, fmt.Errorf("--body-file and --prompt are mutually exclusive")
	}
	if prompt != "" {
		return map[string]any{"prompt": prompt}, nil
	}
	return loadBodyFile(bodyFile)
}
