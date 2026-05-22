package consent

import "fmt"

const SchemaVersion = "structured-consent/v1"

const (
	StatusMatched     = "matched"
	StatusMismatched  = "mismatched"
	StatusMissing     = "missing"
	StatusNotRequired = "not_required"
)

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
		return denyValidation(CodeStructuredConsentMissing, "confirm_paid must be true for structured paid consent")
	}
	if consent.ConfirmedModel != "" && consent.ConfirmedModel != scope.Model {
		return denyValidation(CodeStructuredConsentModelMismatch, fmt.Sprintf("confirmed_model %q does not match actual model %q", consent.ConfirmedModel, scope.Model))
	}
	if consent.ConfirmedMode != "" && consent.ConfirmedMode != scope.Mode {
		return denyValidation(CodeStructuredConsentModeMismatch, fmt.Sprintf("confirmed_mode %q does not match actual mode %q", consent.ConfirmedMode, scope.Mode))
	}
	if consent.ConfirmedMaxCredits > 0 && scope.QuoteCredits > consent.ConfirmedMaxCredits {
		return denyValidation(CodeStructuredConsentCreditLimitExceeded, fmt.Sprintf("quote credits %.2f exceed confirmed_max_credits %.2f", scope.QuoteCredits, consent.ConfirmedMaxCredits))
	}
	if consent.ConfirmedMaxTotalCredits > 0 && scope.TotalCredits > consent.ConfirmedMaxTotalCredits {
		return denyValidation(CodeStructuredConsentCreditLimitExceeded, fmt.Sprintf("total credits %.2f exceed confirmed_max_total_credits %.2f", scope.TotalCredits, consent.ConfirmedMaxTotalCredits))
	}
	if consent.ConfirmedOutputs > 0 && scope.Outputs > consent.ConfirmedOutputs {
		return denyValidation(CodeStructuredConsentOutputLimitExceeded, fmt.Sprintf("outputs %d exceed confirmed_outputs %d", scope.Outputs, consent.ConfirmedOutputs))
	}
	if consent.ConfirmedRemoteRequests > 0 && scope.RemoteRequests > consent.ConfirmedRemoteRequests {
		return denyValidation(CodeStructuredConsentRemoteLimitExceeded, fmt.Sprintf("remote requests %d exceed confirmed_remote_requests %d", scope.RemoteRequests, consent.ConfirmedRemoteRequests))
	}
	if consent.ConfirmedMaxActiveTasks > 0 && scope.MaxActiveTasks > consent.ConfirmedMaxActiveTasks {
		return denyValidation(CodeStructuredConsentActiveTaskExceeded, fmt.Sprintf("max active tasks %d exceed confirmed_max_active_tasks %d", scope.MaxActiveTasks, consent.ConfirmedMaxActiveTasks))
	}
	return ValidationResult{Allowed: true}
}

func denyValidation(code DenialCode, message string) ValidationResult {
	return ValidationResult{Allowed: false, Code: code, Message: message}
}

// PaidConfirmation is the presence-aware form used at submit gates where a zero
// value can mean either "not provided" or "unbounded" depending on the caller.
type PaidConfirmation struct {
	ConfirmPaid              *bool
	ConfirmedMaxTotalCredits *float64
	ConfirmedRemoteRequests  *int
	ConfirmedMaxActiveTasks  *int
	ConfirmedModel           *string
	ConfirmedMode            *string
	ConfirmedOutputs         *int
	ConfirmedMaxCredits      *float64
}

type PaidPlan struct {
	Required          bool
	TotalCredits      float64
	RemoteRequests    int
	MaxActiveTasks    int
	Model             string
	Mode              string
	Outputs           int
	RequireModel      bool
	RequireMode       bool
	RequireOutputs    bool
	RequireMaxCredits bool
}

type Outcome struct {
	SchemaVersion string            `json:"schema_version"`
	Allowed       bool              `json:"allowed"`
	Code          string            `json:"code,omitempty"`
	Fields        map[string]string `json:"fields,omitempty"`
}

func VerifyPaid(plan PaidPlan, confirmation PaidConfirmation) Outcome {
	out := Outcome{
		SchemaVersion: SchemaVersion,
		Allowed:       true,
		Fields:        map[string]string{},
	}
	if !plan.Required {
		out.Fields["confirm_paid"] = StatusNotRequired
		out.Fields["confirmed_max_total_credits"] = StatusNotRequired
		out.Fields["confirmed_remote_requests"] = StatusNotRequired
		out.Fields["confirmed_max_active_tasks"] = StatusNotRequired
		return out
	}
	if confirmation.ConfirmPaid == nil || !*confirmation.ConfirmPaid {
		return denyOutcome(out, "structured_consent_missing_confirm_paid", "confirm_paid", StatusMissing)
	}
	out.Fields["confirm_paid"] = StatusMatched

	maxTotal := confirmation.ConfirmedMaxTotalCredits
	field := "confirmed_max_total_credits"
	if maxTotal == nil && confirmation.ConfirmedMaxCredits != nil {
		maxTotal = confirmation.ConfirmedMaxCredits
		field = "confirmed_max_credits"
	}
	if maxTotal == nil {
		return denyOutcome(out, "structured_consent_missing_max_total_credits", "confirmed_max_total_credits", StatusMissing)
	}
	if *maxTotal < plan.TotalCredits {
		return denyOutcome(out, "structured_consent_max_total_credits_mismatch", field, StatusMismatched)
	}
	out.Fields[field] = StatusMatched

	if confirmation.ConfirmedRemoteRequests == nil {
		return denyOutcome(out, "structured_consent_missing_remote_requests", "confirmed_remote_requests", StatusMissing)
	}
	if *confirmation.ConfirmedRemoteRequests < plan.RemoteRequests {
		return denyOutcome(out, "structured_consent_remote_requests_mismatch", "confirmed_remote_requests", StatusMismatched)
	}
	out.Fields["confirmed_remote_requests"] = StatusMatched

	if plan.MaxActiveTasks > 0 {
		if confirmation.ConfirmedMaxActiveTasks == nil {
			return denyOutcome(out, "structured_consent_missing_max_active_tasks", "confirmed_max_active_tasks", StatusMissing)
		}
		if *confirmation.ConfirmedMaxActiveTasks < plan.MaxActiveTasks {
			return denyOutcome(out, "structured_consent_max_active_tasks_mismatch", "confirmed_max_active_tasks", StatusMismatched)
		}
		out.Fields["confirmed_max_active_tasks"] = StatusMatched
	} else {
		out.Fields["confirmed_max_active_tasks"] = StatusNotRequired
	}

	if plan.RequireModel {
		if confirmation.ConfirmedModel == nil {
			return denyOutcome(out, "structured_consent_missing_model", "confirmed_model", StatusMissing)
		}
		if *confirmation.ConfirmedModel != plan.Model {
			return denyOutcome(out, "structured_consent_model_mismatch", "confirmed_model", StatusMismatched)
		}
		out.Fields["confirmed_model"] = StatusMatched
	}
	if plan.RequireMode {
		if confirmation.ConfirmedMode == nil {
			return denyOutcome(out, "structured_consent_missing_mode", "confirmed_mode", StatusMissing)
		}
		if *confirmation.ConfirmedMode != plan.Mode {
			return denyOutcome(out, "structured_consent_mode_mismatch", "confirmed_mode", StatusMismatched)
		}
		out.Fields["confirmed_mode"] = StatusMatched
	}
	if plan.RequireOutputs {
		if confirmation.ConfirmedOutputs == nil {
			return denyOutcome(out, "structured_consent_missing_outputs", "confirmed_outputs", StatusMissing)
		}
		if *confirmation.ConfirmedOutputs < plan.Outputs {
			return denyOutcome(out, "structured_consent_outputs_mismatch", "confirmed_outputs", StatusMismatched)
		}
		out.Fields["confirmed_outputs"] = StatusMatched
	}
	return out
}

func denyOutcome(out Outcome, code, field, status string) Outcome {
	out.Allowed = false
	out.Code = code
	out.Fields[field] = status
	return out
}

func (o Outcome) Summary() map[string]any {
	return map[string]any{
		"schema_version": o.SchemaVersion,
		"allowed":        o.Allowed,
		"code":           o.Code,
		"fields":         o.Fields,
	}
}

func (o Outcome) Error() string {
	if o.Code == "" {
		return "structured consent denied"
	}
	return fmt.Sprintf("structured consent denied: %s", o.Code)
}
