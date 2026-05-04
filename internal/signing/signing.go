// Package signing defines the Signer interface for Lovart request signing.
// The default implementation uses wazero to load a WASM module embedded at build time.
package signing

import (
	"context"
	"fmt"
)

// SigningPayload is the input to the signing function.
// Timestamp and ReqUUID must be provided by the caller (after time sync).
// Third and Fourth are typically empty strings.
type SigningPayload struct {
	Timestamp string // adjusted server timestamp (ms)
	ReqUUID   string // random hex UUID
	Third     string
	Fourth    string
}

// SigningResult carries the computed signature and headers that must be
// attached to the outbound HTTP request.
type SigningResult struct {
	Signature string
	Timestamp string
	ReqUUID   string
}

// Headers returns the three HTTP headers for the signature.
func (r *SigningResult) Headers() map[string]string {
	return map[string]string{
		"X-Send-Timestamp":   r.Timestamp,
		"X-Req-Uuid":         r.ReqUUID,
		"X-Client-Signature": r.Signature,
	}
}

// Signer signs Lovart API requests.
type Signer interface {
	Sign(ctx context.Context, payload SigningPayload) (*SigningResult, error)
	Health() error
}

// ErrNoSigner is returned when no WASM module or signer is available.
var ErrNoSigner = fmt.Errorf("no signer available: run capture to extract signing wasm")
