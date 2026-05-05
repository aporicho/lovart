package jobs

import "fmt"

// BatchGate is the aggregate safety decision before batch submission.
type BatchGate struct {
	Allowed                  bool     `json:"allowed"`
	AllowPaid                bool     `json:"allow_paid"`
	MaxTotalCredits          float64  `json:"max_total_credits,omitempty"`
	TotalCredits             float64  `json:"total_credits"`
	SelectedRemoteRequests   int      `json:"selected_remote_requests"`
	PaidRequestIDs           []string `json:"paid_request_ids,omitempty"`
	UnknownPricingRequestIDs []string `json:"unknown_pricing_request_ids,omitempty"`
	FailedRequestIDs         []string `json:"failed_request_ids,omitempty"`
	BlockingRequestIDs       []string `json:"blocking_request_ids,omitempty"`
	RecommendedActions       []string `json:"recommended_actions,omitempty"`
}

// GateError reports a batch gate failure.
type GateError struct {
	Code string
	Gate *BatchGate
}

func (e *GateError) Error() string {
	return fmt.Sprintf("jobs: batch gate blocked with %s", e.Code)
}

func evaluateGate(state *RunState, opts JobsOptions, statuses map[string]bool) *BatchGate {
	gate := &BatchGate{
		Allowed:         true,
		AllowPaid:       opts.AllowPaid,
		MaxTotalCredits: opts.MaxTotalCredits,
	}
	for _, job := range state.Jobs {
		for _, request := range job.RemoteRequests {
			if statuses != nil && !statuses[request.Status] {
				continue
			}
			gate.SelectedRemoteRequests++
			if request.Quote == nil {
				gate.UnknownPricingRequestIDs = append(gate.UnknownPricingRequestIDs, request.RequestID)
			} else {
				gate.TotalCredits += request.Quote.Price
				if request.Quote.Price > 0 {
					gate.PaidRequestIDs = append(gate.PaidRequestIDs, request.RequestID)
				}
			}
			if request.Status == StatusFailed {
				gate.FailedRequestIDs = append(gate.FailedRequestIDs, request.RequestID)
			}
			if len(request.Errors) > 0 {
				gate.BlockingRequestIDs = append(gate.BlockingRequestIDs, request.RequestID)
			}
		}
	}
	if len(gate.UnknownPricingRequestIDs) > 0 {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, "run `lovart jobs dry-run <jobs.jsonl>` again after network/pricing is available")
	}
	if len(gate.FailedRequestIDs) > 0 || len(gate.BlockingRequestIDs) > 0 {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, "inspect failed requests with `lovart jobs status <run_dir> --detail requests`")
	}
	if len(gate.PaidRequestIDs) > 0 && !opts.AllowPaid {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, fmt.Sprintf("pass `--allow-paid --max-total-credits %.0f` to allow this batch", gate.TotalCredits))
	}
	if len(gate.PaidRequestIDs) > 0 && opts.AllowPaid && opts.MaxTotalCredits <= 0 {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, "pass `--max-total-credits N` together with `--allow-paid`")
	}
	if len(gate.PaidRequestIDs) > 0 && opts.AllowPaid && opts.MaxTotalCredits > 0 && gate.TotalCredits > opts.MaxTotalCredits {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, fmt.Sprintf("increase `--max-total-credits` to at least %.0f or reduce the batch", gate.TotalCredits))
	}
	return gate
}

func gateErrorCode(gate *BatchGate) string {
	if len(gate.UnknownPricingRequestIDs) > 0 {
		return "unknown_pricing"
	}
	if len(gate.FailedRequestIDs) > 0 || len(gate.BlockingRequestIDs) > 0 {
		return "task_failed"
	}
	return "credit_risk"
}

func ensureGateAllowed(state *RunState, opts JobsOptions, statuses map[string]bool) error {
	gate := evaluateGate(state, opts, statuses)
	state.BatchGate = gate
	if gate.Allowed {
		return nil
	}
	return &GateError{Code: gateErrorCode(gate), Gate: gate}
}
