package jobs

import (
	"context"
	"fmt"
	"time"

	"github.com/aporicho/lovart/internal/pricing"
)

func quoteState(ctx context.Context, remote RemoteClient, state *RunState) error {
	if remote == nil {
		return fmt.Errorf("jobs: quote requires remote client")
	}
	cache := map[string]*pricingCacheEntry{}
	for i := range state.Jobs {
		for j := range state.Jobs[i].RemoteRequests {
			request := &state.Jobs[i].RemoteRequests[j]
			if request.Quote != nil || request.TaskID != "" {
				continue
			}
			normalizedBody, err := normalizeRemoteRequestBody(*request)
			if err != nil {
				addRequestError(request, "unknown_pricing", "request normalization failed", map[string]any{"error": err.Error()})
				request.Status = StatusFailed
				RefreshStatuses(state)
				if err := SaveState(state); err != nil {
					return err
				}
				continue
			}
			signature := CostSignature(JobLine{
				JobID:   request.RequestID,
				Model:   request.Model,
				Mode:    request.Mode,
				Outputs: request.OutputCount,
				Body:    normalizedBody,
			})
			if cached, ok := cache[signature]; ok {
				request.Quote = cached.quote
				request.NormalizedBody = normalizedBody
				request.Status = StatusQuoted
				continue
			}
			quote, err := remote.Quote(ctx, request.Model, normalizedBody, request.Mode)
			if err != nil {
				addRequestError(request, "unknown_pricing", "live quote failed", map[string]any{"error": err.Error()})
				request.Status = StatusFailed
			} else {
				request.Quote = quote
				request.NormalizedBody = quote.NormalizedBody
				if len(request.NormalizedBody) == 0 {
					request.NormalizedBody = normalizedBody
				}
				request.Status = StatusQuoted
				request.UpdatedAt = time.Now().UTC()
				cache[signature] = &pricingCacheEntry{quote: quote}
			}
			RefreshStatuses(state)
			if err := SaveState(state); err != nil {
				return err
			}
		}
	}
	RefreshStatuses(state)
	return nil
}

type pricingCacheEntry struct {
	quote *pricing.QuoteResult
}
