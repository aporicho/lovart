// Package pricing handles Lovart credit quoting.
package pricing

import (
	"context"

	"github.com/aporicho/lovart/internal/http"
)

// QuoteResult contains the credit cost for a request.
type QuoteResult struct {
	Quoted         bool    `json:"quoted"`
	Credits        float64 `json:"credits"`
	PayableCredits float64 `json:"payable_credits"`
	ListedCredits  float64 `json:"listed_credits"`
	Exact          bool    `json:"exact"`
}

// Quote fetches a live credit quote for the given model and body.
func Quote(ctx context.Context, client *http.Client, model string, body map[string]any) (*QuoteResult, error) {
	// TODO: call Lovart signed pricing endpoint
	return &QuoteResult{Quoted: false, Exact: false}, nil
}
