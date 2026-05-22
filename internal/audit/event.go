// Package audit writes redacted security-relevant events as JSON Lines.
package audit

import "time"

// Event describes one auditable operation decision.
type Event struct {
	Timestamp time.Time      `json:"timestamp"`
	Operation string         `json:"operation,omitempty"`
	Actor     string         `json:"actor,omitempty"`
	Source    string         `json:"source,omitempty"`
	Tool      string         `json:"tool,omitempty"`
	Action    string         `json:"action,omitempty"`
	Decision  string         `json:"decision,omitempty"`
	RiskFlags []string       `json:"risk_flags,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	Error     string         `json:"error,omitempty"`
}
