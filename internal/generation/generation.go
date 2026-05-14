package generation

import (
	"context"
	"fmt"
	"time"

	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/pricing"
)

// PreflightResult is the gate check outcome before submission.
type PreflightResult struct {
	CanSubmit          bool                    `json:"can_submit"`
	Credits            float64                 `json:"credits"`
	CreditRisk         bool                    `json:"credit_risk"`
	PaidRequired       bool                    `json:"paid_required"`
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

// Options configures a generation request.
type Options struct {
	Mode       string
	AllowPaid  bool
	MaxCredits float64
	ProjectID  string
	CID        string
	Wait       bool
	Download   bool
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

	if paidRequired && !opts.AllowPaid {
		canSubmit = false
		actions = append(actions, fmt.Sprintf("this request costs %.2f credits; use --allow-paid --max-credits %.0f", credits, credits))
	}
	if paidRequired && opts.AllowPaid && credits > opts.MaxCredits {
		canSubmit = false
		actions = append(actions, fmt.Sprintf("cost %.2f exceeds max %.0f credits", credits, opts.MaxCredits))
	}
	if credits == 0 {
		actions = append(actions, "zero-credit generation (free tier)")
	}

	return &PreflightResult{
		CanSubmit:          canSubmit,
		Credits:            credits,
		CreditRisk:         paidRequired && !opts.AllowPaid,
		PaidRequired:       paidRequired,
		NormalizedBody:     quote.NormalizedBody,
		PricingContext:     quote.PricingContext,
		RecommendedActions: actions,
	}, nil
}

// Submit sends a generation request to LGW and returns the task ID.
func Submit(ctx context.Context, client *http.Client, model string, body map[string]any, opts Options) (*SubmitResult, error) {
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

// FetchTask retrieves the current task status once.
func FetchTask(ctx context.Context, client *http.Client, taskID string) (map[string]any, error) {
	path := fmt.Sprintf("/v1/generator/tasks?task_id=%s", taskID)

	var resp struct {
		Code int `json:"code"`
		Data struct {
			Status          string `json:"status"`
			GeneratorTaskID string `json:"generator_task_id"`
			Artifacts       []struct {
				Content  string `json:"content"`
				Metadata struct {
					Width  int `json:"width"`
					Height int `json:"height"`
				} `json:"metadata"`
			} `json:"artifacts"`
		} `json:"data"`
	}

	if err := client.GetJSON(ctx, http.LGWBase, path, &resp); err != nil {
		return nil, fmt.Errorf("generation: poll: %w", err)
	}
	if resp.Code != 0 {
		return nil, fmt.Errorf("generation: poll returned code %d", resp.Code)
	}

	result := map[string]any{
		"task_id": resp.Data.GeneratorTaskID,
		"status":  resp.Data.Status,
	}
	if resp.Data.Status == "completed" {
		var urls []string
		var details []map[string]any
		for _, a := range resp.Data.Artifacts {
			urls = append(urls, a.Content)
			details = append(details, map[string]any{
				"url":    a.Content,
				"width":  a.Metadata.Width,
				"height": a.Metadata.Height,
			})
		}
		result["artifacts"] = urls
		result["artifact_details"] = details
	}

	return result, nil
}

// Wait polls the task status until it's completed.
func Wait(ctx context.Context, client *http.Client, taskID string) (map[string]any, error) {
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
		status, _ := result["status"].(string)
		if status == "completed" || status == "failed" {
			return result, nil
		}
		time.Sleep(2 * time.Second)
	}
}
