package jobs

import (
	"fmt"

	"github.com/aporicho/lovart/internal/consent"
)

// BatchGate is the aggregate safety decision before batch submission.
type BatchGate struct {
	Allowed                  bool             `json:"allowed"`
	AllowPaid                bool             `json:"allow_paid"`
	MaxTotalCredits          float64          `json:"max_total_credits,omitempty"`
	TotalCredits             float64          `json:"total_credits"`
	SelectedRemoteRequests   int              `json:"selected_remote_requests"`
	PlannedRemoteRequests    int              `json:"planned_remote_requests"`
	StructuredConsentOutcome *consent.Outcome `json:"structured_consent,omitempty"`
	PaidRequestIDs           []string         `json:"paid_request_ids,omitempty"`
	UnknownPricingRequestIDs []string         `json:"unknown_pricing_request_ids,omitempty"`
	FailedRequestIDs         []string         `json:"failed_request_ids,omitempty"`
	BlockingRequestIDs       []string         `json:"blocking_request_ids,omitempty"`
	RecommendedActions       []string         `json:"recommended_actions,omitempty"`
}

// GateError reports a batch gate failure.
type GateError struct {
	Code      string
	Gate      *BatchGate
	RunDir    string
	StateFile string
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
			if hasBlockingRequestErrors(request) {
				gate.BlockingRequestIDs = append(gate.BlockingRequestIDs, request.RequestID)
			}
		}
	}
	gate.PlannedRemoteRequests = plannedRemoteRequests(gate.SelectedRemoteRequests, opts)
	if len(gate.UnknownPricingRequestIDs) > 0 {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, "rerun `lovart jobs run <jobs.jsonl>` or `lovart jobs resume <run_dir>` after network/pricing is available")
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

func hasBlockingRequestErrors(request RemoteRequest) bool {
	for _, err := range request.Errors {
		if err.Code != "submission_deferred" {
			return true
		}
	}
	return false
}

func gateErrorCode(gate *BatchGate) string {
	if gate.StructuredConsentOutcome != nil && gate.StructuredConsentOutcome.Code != "" {
		return gate.StructuredConsentOutcome.Code
	}
	if len(gate.UnknownPricingRequestIDs) > 0 {
		return "unknown_pricing"
	}
	if len(gate.FailedRequestIDs) > 0 || len(gate.BlockingRequestIDs) > 0 {
		return "task_failed"
	}
	return "credit_risk"
}

func structuredConsentAction(code string, gate *BatchGate, opts JobsOptions) string {
	switch code {
	case "structured_consent_missing_confirm_paid", "structured_consent_confirm_paid_mismatch":
		return "pass structured paid consent `confirm_paid=true` for this paid batch"
	case "structured_consent_missing_max_total_credits", "structured_consent_max_total_credits_mismatch":
		return fmt.Sprintf("pass `confirmed_max_total_credits` of at least %.0f for this paid batch", gate.TotalCredits)
	case "structured_consent_missing_remote_requests", "structured_consent_remote_requests_mismatch":
		return fmt.Sprintf("pass `confirmed_remote_requests=%d` for this paid batch invocation", gate.PlannedRemoteRequests)
	case "structured_consent_missing_max_active_tasks", "structured_consent_max_active_tasks_mismatch":
		return fmt.Sprintf("pass `confirmed_max_active_tasks=%d` for this paid batch", normalizedMaxActiveTasks(opts))
	default:
		return "pass matching structured paid consent fields before submitting this paid batch"
	}
}

func applyStructuredPaidConsent(gate *BatchGate, opts JobsOptions) {
	if len(gate.PaidRequestIDs) == 0 || !opts.AllowPaid || !gate.Allowed {
		return
	}
	outcome := consent.VerifyPaid(consent.PaidPlan{
		Required:       true,
		TotalCredits:   gate.TotalCredits,
		RemoteRequests: gate.PlannedRemoteRequests,
		MaxActiveTasks: normalizedMaxActiveTasks(opts),
	}, consent.PaidConfirmation{
		ConfirmPaid:              boolPtr(opts.ConfirmPaid),
		ConfirmedMaxTotalCredits: positiveFloatPtr(opts.ConfirmedMaxTotalCredits),
		ConfirmedRemoteRequests:  positiveIntPtr(opts.ConfirmedRemoteRequests),
		ConfirmedMaxActiveTasks:  positiveIntPtr(opts.ConfirmedMaxActiveTasks),
	})
	gate.StructuredConsentOutcome = &outcome
	if !outcome.Allowed {
		gate.Allowed = false
		gate.RecommendedActions = append(gate.RecommendedActions, structuredConsentAction(outcome.Code, gate, opts))
	}
}

func ensureGateAllowed(state *RunState, opts JobsOptions, statuses map[string]bool) error {
	gate := evaluateGate(state, opts, statuses)
	applyStructuredPaidConsent(gate, opts)
	state.BatchGate = gate
	if gate.Allowed {
		return nil
	}
	return &GateError{Code: gateErrorCode(gate), Gate: gate, RunDir: state.RunDir, StateFile: state.StateFile}
}

func plannedRemoteRequests(selected int, opts JobsOptions) int {
	limit := normalizedSubmitLimit(opts)
	if limit <= 0 || limit > selected {
		return selected
	}
	return limit
}

func boolPtr(value bool) *bool {
	if !value {
		return nil
	}
	return &value
}

func positiveFloatPtr(value float64) *float64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func positiveIntPtr(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}
