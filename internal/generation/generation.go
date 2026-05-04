// Package generation handles single image generation: preflight, dry-run, submit, and poll.
package generation

import (
	"context"

	"github.com/aporicho/lovart/internal/http"
)

// Options configures a generation request.
type Options struct {
	Mode        string  // auto, fast, relax
	AllowPaid   bool
	MaxCredits  float64
}

// PreflightResult is the gate check outcome before submission.
type PreflightResult struct {
	CanSubmit         bool     `json:"can_submit"`
	BlockingError     any      `json:"blocking_error,omitempty"`
	CreditRisk        bool     `json:"credit_risk"`
	PaidRequired      bool     `json:"paid_required"`
	QuotedCredits     float64  `json:"quoted_credits"`
	RecommendedActions []string `json:"recommended_actions,omitempty"`
}

// SubmitResult is the response after a successful generation submission.
type SubmitResult struct {
	TaskID    string        `json:"task_id"`
	Status    string        `json:"status"`
	Task      any           `json:"task,omitempty"`
	Artifacts []any         `json:"artifacts"`
	Downloads []any         `json:"downloads,omitempty"`
}

// Preflight checks all gates: auth, schema, credits, mode.
func Preflight(client *http.Client, model string, body map[string]any, opts Options) (*PreflightResult, error) {
	// TODO: implement auth check, schema validation, quote, mode slot
	return &PreflightResult{CanSubmit: false, RecommendedActions: []string{"not implemented"}}, nil
}

// DryRun returns the request payload without submitting.
func DryRun(model string, body map[string]any) (map[string]any, error) {
	// TODO: return dry request preview
	return map[string]any{"model": model, "body": body, "submitted": false}, nil
}

// Submit sends a generation request and returns the task ID.
func Submit(ctx context.Context, client *http.Client, model string, body map[string]any, opts Options) (*SubmitResult, error) {
	// TODO: call take_generation_slot, apply mode, submit to LGW
	return nil, nil
}

// Wait polls task status until completion or timeout.
func Wait(ctx context.Context, client *http.Client, taskID string) (map[string]any, error) {
	// TODO: poll GET /v1/generator/tasks?task_id=...
	return nil, nil
}
