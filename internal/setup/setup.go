// Package setup reports Lovart CLI readiness.
package setup

// Status reports auth, refs, signer, and runtime readiness.
type Status struct {
	Ready      bool              `json:"ready"`
	Auth       map[string]any    `json:"auth"`
	Signer     map[string]any    `json:"signer"`
	Refs       map[string]any    `json:"refs"`
	Warnings   []string          `json:"warnings,omitempty"`
}

// Readiness checks all components needed for generation.
func Readiness(offline bool) *Status {
	// TODO: check auth file, signing wasm, ref data
	return &Status{
		Ready: false,
		Auth:   map[string]any{"available": false},
		Signer: map[string]any{"available": false},
		Refs:   map[string]any{"available": false},
	}
}
