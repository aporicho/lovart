package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/aporicho/lovart/internal/consent"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/pricing"
)

// PreflightResult is the gate check outcome before submission.
type PreflightResult struct {
	CanSubmit          bool                    `json:"can_submit"`
	Credits            float64                 `json:"credits"`
	CreditRisk         bool                    `json:"credit_risk"`
	PaidRequired       bool                    `json:"paid_required"`
	StructuredConsent  *consent.Outcome        `json:"structured_consent,omitempty"`
	NormalizedBody     map[string]any          `json:"normalized_body,omitempty"`
	PricingContext     *pricing.PricingContext `json:"pricing_context,omitempty"`
	RecommendedActions []string                `json:"recommended_actions,omitempty"`
}

// SubmitResult is the response after a successful generation submission.
type SubmitResult struct {
	TaskID         string         `json:"task_id"`
	Status         string         `json:"status"`
	NormalizedBody map[string]any `json:"normalized_body,omitempty"`
}

// StructuredConsentError is returned when a paid generation lacks matching
// structured consent.
type StructuredConsentError struct {
	Code    string
	Message string
	Outcome consent.Outcome
}

func (e *StructuredConsentError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return fmt.Sprintf("generation: structured paid consent denied: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("generation: structured paid consent denied: %s", e.Code)
}

// Options configures a generation request.
type Options struct {
	Mode                      string
	AllowPaid                 bool
	MaxCredits                float64
	ConfirmPaid               bool
	ConfirmedMaxCredits       float64
	ConfirmedModel            string
	ConfirmedMode             string
	ConfirmedOutputs          int
	ConfirmedRemoteRequests   int
	ProjectID                 string
	ConfirmedProjectID        string
	CID                       string
	Wait                      bool
	Download                  bool
	DownloadDir               string
	Overwrite                 bool
	ConfirmedCanonicalPath    string
	ConfirmOverwrite          bool
	ConfirmOutsideDefaultRoot bool
	Canvas                    bool
	CanvasLayout              string
	ConfirmCanvasWrite        bool
	ConfirmedCanvasLayout     string
}

// WaitOptions configures task polling.
type WaitOptions struct {
	PollInterval time.Duration
}

// Preflight checks all gates: auth, quote, slot, mode.
func Preflight(ctx context.Context, client *http.Client, model string, body map[string]any, opts Options) (*PreflightResult, error) {
	// 1. Quote the request.
	quote, err := pricing.QuoteWithOptions(ctx, client, model, body, pricing.QuoteOptions{Mode: opts.Mode})
	if err != nil {
		return &PreflightResult{
			CanSubmit:          false,
			CreditRisk:         true,
			RecommendedActions: []string{fmt.Sprintf("quote failed: %v", err)},
		}, nil
	}

	credits := quote.Price
	paidRequired := credits > 0
	canSubmit := true
	var actions []string
	var structuredConsent *consent.Outcome

	if paidRequired && !opts.AllowPaid {
		canSubmit = false
		actions = append(actions, fmt.Sprintf("this request costs %.2f credits; use --allow-paid --max-credits %.0f", credits, credits))
	}
	if paidRequired && opts.AllowPaid && credits > opts.MaxCredits {
		canSubmit = false
		actions = append(actions, fmt.Sprintf("cost %.2f exceeds max %.0f credits", credits, opts.MaxCredits))
	}
	if paidRequired && opts.AllowPaid {
		if outcome := verifyStructuredPaidConsent(model, quote.NormalizedBody, opts, credits); !outcome.Allowed {
			structuredConsent = &outcome
			canSubmit = false
			actions = append(actions, structuredConsentMessage(outcome))
		} else {
			structuredConsent = &outcome
		}
	}
	if credits == 0 {
		actions = append(actions, "zero-credit generation (free tier)")
	}

	return &PreflightResult{
		CanSubmit:          canSubmit,
		Credits:            credits,
		CreditRisk:         paidRequired && !canSubmit,
		PaidRequired:       paidRequired,
		StructuredConsent:  structuredConsent,
		NormalizedBody:     quote.NormalizedBody,
		PricingContext:     quote.PricingContext,
		RecommendedActions: actions,
	}, nil
}

// Submit sends a generation request to LGW and returns the task ID.
func Submit(ctx context.Context, client *http.Client, model string, body map[string]any, opts Options) (*SubmitResult, error) {
	if err := EnforceSubmitConsent(opts); err != nil {
		return nil, err
	}
	if opts.AllowPaid {
		quote, err := pricing.QuoteWithOptions(ctx, client, model, body, pricing.QuoteOptions{Mode: opts.Mode})
		if err != nil {
			return nil, fmt.Errorf("generation: submit: quote paid gate: %w", err)
		}
		if quote.Price > opts.MaxCredits {
			return nil, fmt.Errorf("generation: submit: cost %.2f exceeds max %.0f credits", quote.Price, opts.MaxCredits)
		}
		if quote.Price > 0 {
			if outcome := verifyStructuredPaidConsent(model, quote.NormalizedBody, opts, quote.Price); !outcome.Allowed {
				return nil, structuredConsentError(outcome)
			}
		}
	}

	payload, normalizedBody, err := buildNormalizedTaskPayload(model, body, opts)
	if err != nil {
		return nil, fmt.Errorf("generation: submit: normalize request defaults: %w", err)
	}

	// Set mode.
	if err := SetMode(ctx, client, opts.CID, opts.Mode); err != nil {
		return nil, fmt.Errorf("generation: submit: set mode: %w", err)
	}

	// Take slot.
	if err := TakeSlot(ctx, client, opts.ProjectID, opts.CID); err != nil {
		return nil, fmt.Errorf("generation: submit: take slot: %w", err)
	}

	path := "/v1/generator/tasks"
	var resp struct {
		Code int `json:"code"`
		Data struct {
			TaskID string `json:"generator_task_id"`
		} `json:"data"`
	}

	if err := client.PostJSON(ctx, http.LGWBase, path, payload, &resp); err != nil {
		return nil, fmt.Errorf("generation: submit: %w", err)
	}

	if resp.Code != 0 {
		return nil, fmt.Errorf("generation: submit returned code %d", resp.Code)
	}

	return &SubmitResult{
		TaskID:         resp.Data.TaskID,
		Status:         "submitted",
		NormalizedBody: normalizedBody,
	}, nil
}

func verifyStructuredPaidConsent(model string, normalizedBody map[string]any, opts Options, credits float64) consent.Outcome {
	mode, err := pricing.NormalizeMode(opts.Mode)
	if err != nil {
		mode = opts.Mode
	}
	return consent.VerifyPaid(consent.PaidPlan{
		Required:          credits > 0,
		TotalCredits:      credits,
		RemoteRequests:    1,
		Model:             model,
		Mode:              mode,
		Outputs:           generationOutputs(normalizedBody),
		RequireModel:      true,
		RequireMode:       true,
		RequireOutputs:    true,
		RequireMaxCredits: true,
	}, consent.PaidConfirmation{
		ConfirmPaid:             boolPtr(opts.ConfirmPaid),
		ConfirmedMaxCredits:     positiveFloatPtr(opts.ConfirmedMaxCredits),
		ConfirmedRemoteRequests: positiveIntPtr(opts.ConfirmedRemoteRequests),
		ConfirmedModel:          stringPtr(opts.ConfirmedModel),
		ConfirmedMode:           stringPtr(opts.ConfirmedMode),
		ConfirmedOutputs:        positiveIntPtr(opts.ConfirmedOutputs),
	})
}

func generationOutputs(body map[string]any) int {
	if n, ok := intFromAny(body["n"]); ok && n > 0 {
		return n
	}
	if n, ok := intFromAny(body["num_images"]); ok && n > 0 {
		return n
	}
	if n, ok := intFromAny(body["count"]); ok && n > 0 {
		return n
	}
	return 1
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int64:
		return int(v), true
	case float64:
		return int(v), true
	case float32:
		return int(v), true
	default:
		return 0, false
	}
}

func structuredConsentError(outcome consent.Outcome) error {
	return &StructuredConsentError{Code: outcome.Code, Message: "pass matching structured paid consent before paid generation submit", Outcome: outcome}
}

func structuredConsentMessage(outcome consent.Outcome) string {
	if outcome.Code == "" {
		return "structured paid consent denied"
	}
	return fmt.Sprintf("structured paid consent denied: %s", outcome.Code)
}

func boolPtr(v bool) *bool { return &v }

func positiveFloatPtr(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	return &v
}

func positiveIntPtr(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// FetchTask retrieves the current task status once.
func FetchTask(ctx context.Context, client *http.Client, taskID string) (map[string]any, error) {
	path := fmt.Sprintf("/v1/generator/tasks?task_id=%s", taskID)

	var resp struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}

	if err := client.GetJSON(ctx, http.LGWBase, path, &resp); err != nil {
		return nil, fmt.Errorf("generation: poll: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("generation: poll returned code %d", resp.Code)
	}

	return BuildTaskResult(taskID, resp.Data), nil
}

// Wait polls the task status until it's completed.
func Wait(ctx context.Context, client *http.Client, taskID string) (map[string]any, error) {
	return WaitWithOptions(ctx, client, taskID, WaitOptions{PollInterval: 2 * time.Second})
}

// WaitWithOptions polls the task status until it reaches a terminal state.
func WaitWithOptions(ctx context.Context, client *http.Client, taskID string, opts WaitOptions) (map[string]any, error) {
	pollInterval := opts.PollInterval
	if pollInterval <= 0 {
		pollInterval = 2 * time.Second
	}
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := FetchTask(ctx, client, taskID)
		if err != nil {
			return nil, err
		}
		status := NormalizeTaskStatus(result)
		if status == TaskStatusCompleted || status == TaskStatusFailed {
			return result, nil
		}
		timer := time.NewTimer(pollInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}
