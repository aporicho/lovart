package pricing

import (
	"context"
	"fmt"

	"github.com/aporicho/lovart/internal/http"
)

// QuoteResult contains the credit cost for a request.
type QuoteResult struct {
	Quoted         bool    `json:"quoted"`
	Credits        float64 `json:"credits"`
	PayableCredits float64 `json:"payable_credits"`
}

// quoteResponse mirrors the Lovart pricing endpoint response envelope.
type quoteResponse struct {
	Code int `json:"code"`
	Data struct {
		Price float64 `json:"price"`
	} `json:"data"`
}

// Quote fetches a live credit quote for the given model and body.
func Quote(ctx context.Context, client *http.Client, model string, body map[string]any) (*QuoteResult, error) {
	path := "/v1/generator/pricing"

	// Build pricing request matching v1 format.
	reqBody := map[string]any{
		"generator_name": model,
	}
	// Merge body fields (prompt, size, quality, etc.) into input_args.
	// The pricing endpoint expects the same shape as generation.
	if len(body) > 0 {
		reqBody["input_args"] = body
	}

	var resp quoteResponse
	if err := client.PostJSON(ctx, http.LGWBase, path, reqBody, &resp); err != nil {
		return nil, fmt.Errorf("pricing: quote: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("pricing: quote returned code %d", resp.Code)
	}

	return &QuoteResult{
		Quoted:         true,
		Credits:        resp.Data.Price,
		PayableCredits: resp.Data.Price,
	}, nil
}

// QuoteExact returns whether the quote credits are exact.
func QuoteExact() bool {
	return true
}
