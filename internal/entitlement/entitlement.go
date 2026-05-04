// Package entitlement checks free tier eligibility.
package entitlement

// FreeCheckResult reports whether a request qualifies for zero-credit generation.
type FreeCheckResult struct {
	Eligible   bool              `json:"eligible"`
	Mode       string            `json:"mode"`
	Credits    float64           `json:"credits"`
	Warnings   []string          `json:"warnings,omitempty"`
}

// CheckFree verifies zero-credit eligibility for a request.
func CheckFree(model string, body map[string]any, mode string) (*FreeCheckResult, error) {
	// TODO: call entitlement API
	return &FreeCheckResult{Eligible: false}, nil
}
