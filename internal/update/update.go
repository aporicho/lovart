// Package update handles metadata drift detection and sync.
package update

// DriftResult shows what has changed between local refs and live Lovart.
type DriftResult struct {
	Detected bool              `json:"detected"`
	Changes  []string          `json:"changes,omitempty"`
}

// Check compares local metadata snapshots against live Lovart.
func Check() (*DriftResult, error) {
	// TODO: fetch live manifest, compare with local
	return &DriftResult{Detected: false}, nil
}

// SyncMetadata refreshes local metadata snapshots from live Lovart.
func SyncMetadata() error {
	// TODO: download and save manifest, generator list, schema, pricing
	return nil
}
