package consent

import "fmt"

// DenialCode is a stable, machine-readable reason for rejecting paid consent.
type DenialCode string

const (
	CodeOK DenialCode = ""

	CodeStructuredConsentMissing             DenialCode = "structured_consent_missing"
	CodeStructuredConsentModelMismatch       DenialCode = "structured_consent_model_mismatch"
	CodeStructuredConsentModeMismatch        DenialCode = "structured_consent_mode_mismatch"
	CodeStructuredConsentCreditLimitExceeded DenialCode = "structured_consent_credit_limit_exceeded"
	CodeStructuredConsentOutputLimitExceeded DenialCode = "structured_consent_output_limit_exceeded"
	CodeStructuredConsentRemoteLimitExceeded DenialCode = "structured_consent_remote_limit_exceeded"
	CodeStructuredConsentActiveTaskExceeded  DenialCode = "structured_consent_active_task_limit_exceeded"
)

// PaidConsent is the caller-provided structured confirmation for a paid action.
// Empty string and zero numeric confirmation fields are intentionally optional
// and therefore do not constrain validation.
type PaidConsent struct {
	ConfirmPaid              bool    `json:"confirm_paid"`
	ConfirmedModel           string  `json:"confirmed_model,omitempty"`
	ConfirmedMode            string  `json:"confirmed_mode,omitempty"`
	ConfirmedMaxCredits      float64 `json:"confirmed_max_credits,omitempty"`
	ConfirmedMaxTotalCredits float64 `json:"confirmed_max_total_credits,omitempty"`
	ConfirmedOutputs         int     `json:"confirmed_outputs,omitempty"`
	ConfirmedRemoteRequests  int     `json:"confirmed_remote_requests,omitempty"`
	ConfirmedMaxActiveTasks  int     `json:"confirmed_max_active_tasks,omitempty"`
}

// PaidScope describes the actual paid operation being attempted.
type PaidScope struct {
	Model          string  `json:"model,omitempty"`
	Mode           string  `json:"mode,omitempty"`
	QuoteCredits   float64 `json:"quote_credits,omitempty"`
	TotalCredits   float64 `json:"total_credits,omitempty"`
	Outputs        int     `json:"outputs,omitempty"`
	RemoteRequests int     `json:"remote_requests,omitempty"`
	MaxActiveTasks int     `json:"max_active_tasks,omitempty"`
}

// ValidationResult is the machine-readable result of validating paid consent.
type ValidationResult struct {
	Allowed bool       `json:"allowed"`
	Code    DenialCode `json:"code,omitempty"`
	Message string     `json:"message,omitempty"`
}

// ValidatePaidConsent validates caller-provided structured paid consent against
// the actual paid scope. It returns Allowed=true only when the confirmation is
// explicit and all non-zero/non-empty constraints are satisfied.
func ValidatePaidConsent(consent PaidConsent, scope PaidScope) ValidationResult {
	if !consent.ConfirmPaid {
		return deny(CodeStructuredConsentMissing, "confirm_paid must be true for structured paid consent")
	}
	if consent.ConfirmedModel != "" && consent.ConfirmedModel != scope.Model {
		return deny(CodeStructuredConsentModelMismatch, fmt.Sprintf("confirmed_model %q does not match actual model %q", consent.ConfirmedModel, scope.Model))
	}
	if consent.ConfirmedMode != "" && consent.ConfirmedMode != scope.Mode {
		return deny(CodeStructuredConsentModeMismatch, fmt.Sprintf("confirmed_mode %q does not match actual mode %q", consent.ConfirmedMode, scope.Mode))
	}
	if consent.ConfirmedMaxCredits > 0 && scope.QuoteCredits > consent.ConfirmedMaxCredits {
		return deny(CodeStructuredConsentCreditLimitExceeded, fmt.Sprintf("quote credits %.2f exceed confirmed_max_credits %.2f", scope.QuoteCredits, consent.ConfirmedMaxCredits))
	}
	if consent.ConfirmedMaxTotalCredits > 0 && scope.TotalCredits > consent.ConfirmedMaxTotalCredits {
		return deny(CodeStructuredConsentCreditLimitExceeded, fmt.Sprintf("total credits %.2f exceed confirmed_max_total_credits %.2f", scope.TotalCredits, consent.ConfirmedMaxTotalCredits))
	}
	if consent.ConfirmedOutputs > 0 && scope.Outputs > consent.ConfirmedOutputs {
		return deny(CodeStructuredConsentOutputLimitExceeded, fmt.Sprintf("outputs %d exceed confirmed_outputs %d", scope.Outputs, consent.ConfirmedOutputs))
	}
	if consent.ConfirmedRemoteRequests > 0 && scope.RemoteRequests > consent.ConfirmedRemoteRequests {
		return deny(CodeStructuredConsentRemoteLimitExceeded, fmt.Sprintf("remote requests %d exceed confirmed_remote_requests %d", scope.RemoteRequests, consent.ConfirmedRemoteRequests))
	}
	if consent.ConfirmedMaxActiveTasks > 0 && scope.MaxActiveTasks > consent.ConfirmedMaxActiveTasks {
		return deny(CodeStructuredConsentActiveTaskExceeded, fmt.Sprintf("max active tasks %d exceed confirmed_max_active_tasks %d", scope.MaxActiveTasks, consent.ConfirmedMaxActiveTasks))
	}
	return ValidationResult{Allowed: true}
}

func deny(code DenialCode, message string) ValidationResult {
	return ValidationResult{Allowed: false, Code: code, Message: message}
}
