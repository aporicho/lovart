// Package errors defines error codes and the base LovartError type.
package errors

import "fmt"

// Standard error codes.
const (
	CodeInputError            = "input_error"
	CodeAuthMissing           = "auth_missing"
	CodeMetadataStale         = "metadata_stale"
	CodeSignerStale           = "signer_stale"
	CodeSchemaInvalid         = "schema_invalid"
	CodeUnknownPricing        = "unknown_pricing"
	CodeCreditRisk            = "credit_risk"
	CodeTaskFailed            = "task_failed"
	CodeContentPolicyRejected = "content_policy_rejected"
	CodeTimeout               = "timeout"
	CodeNetworkUnavailable    = "network_unavailable"
	CodeInternal              = "internal_error"
)

// LovartError is the base error type with a machine-readable code and exit code.
type LovartError struct {
	Code     string
	Message  string
	Details  map[string]any
	ExitCode int
}

func (e *LovartError) Error() string {
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// New creates a LovartError with defaults.
func New(code, msg string, details map[string]any) *LovartError {
	return &LovartError{Code: code, Message: msg, Details: details, ExitCode: exitCodeFor(code)}
}

// helpers for common error types

func InputError(msg string, details map[string]any) *LovartError {
	return New(CodeInputError, msg, details)
}

func AuthMissing(msg string, details map[string]any) *LovartError {
	return New(CodeAuthMissing, msg, details)
}

func Internal(msg string, details map[string]any) *LovartError {
	return New(CodeInternal, msg, details)
}

func exitCodeFor(code string) int {
	switch code {
	case CodeInputError, CodeSchemaInvalid:
		return 2
	case CodeAuthMissing:
		return 3
	case CodeSignerStale:
		return 4
	case CodeMetadataStale:
		return 5
	case CodeCreditRisk:
		return 6
	case CodeUnknownPricing:
		return 7
	case CodeTaskFailed, CodeContentPolicyRejected:
		return 8
	case CodeTimeout:
		return 9
	default:
		return 1
	}
}
