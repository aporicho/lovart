package consent

import "fmt"

const SchemaVersion = "structured-consent/v1"

const (
	StatusMatched     = "matched"
	StatusMismatched  = "mismatched"
	StatusMissing     = "missing"
	StatusNotRequired = "not_required"
)

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
	if confirmation.ConfirmPaid == nil {
		return deny(out, "structured_consent_missing_confirm_paid", "confirm_paid", StatusMissing)
	}
	if !*confirmation.ConfirmPaid {
		return deny(out, "structured_consent_confirm_paid_mismatch", "confirm_paid", StatusMismatched)
	}
	out.Fields["confirm_paid"] = StatusMatched

	maxTotal := confirmation.ConfirmedMaxTotalCredits
	field := "confirmed_max_total_credits"
	if maxTotal == nil && confirmation.ConfirmedMaxCredits != nil {
		maxTotal = confirmation.ConfirmedMaxCredits
		field = "confirmed_max_credits"
	}
	if maxTotal == nil {
		return deny(out, "structured_consent_missing_max_total_credits", "confirmed_max_total_credits", StatusMissing)
	}
	if *maxTotal < plan.TotalCredits {
		return deny(out, "structured_consent_max_total_credits_mismatch", field, StatusMismatched)
	}
	out.Fields[field] = StatusMatched

	if confirmation.ConfirmedRemoteRequests == nil {
		return deny(out, "structured_consent_missing_remote_requests", "confirmed_remote_requests", StatusMissing)
	}
	if *confirmation.ConfirmedRemoteRequests < plan.RemoteRequests {
		return deny(out, "structured_consent_remote_requests_mismatch", "confirmed_remote_requests", StatusMismatched)
	}
	out.Fields["confirmed_remote_requests"] = StatusMatched

	if confirmation.ConfirmedMaxActiveTasks == nil {
		return deny(out, "structured_consent_missing_max_active_tasks", "confirmed_max_active_tasks", StatusMissing)
	}
	if *confirmation.ConfirmedMaxActiveTasks < plan.MaxActiveTasks {
		return deny(out, "structured_consent_max_active_tasks_mismatch", "confirmed_max_active_tasks", StatusMismatched)
	}
	out.Fields["confirmed_max_active_tasks"] = StatusMatched

	if plan.RequireModel {
		if confirmation.ConfirmedModel == nil {
			return deny(out, "structured_consent_missing_model", "confirmed_model", StatusMissing)
		}
		if *confirmation.ConfirmedModel != plan.Model {
			return deny(out, "structured_consent_model_mismatch", "confirmed_model", StatusMismatched)
		}
		out.Fields["confirmed_model"] = StatusMatched
	}
	if plan.RequireMode {
		if confirmation.ConfirmedMode == nil {
			return deny(out, "structured_consent_missing_mode", "confirmed_mode", StatusMissing)
		}
		if *confirmation.ConfirmedMode != plan.Mode {
			return deny(out, "structured_consent_mode_mismatch", "confirmed_mode", StatusMismatched)
		}
		out.Fields["confirmed_mode"] = StatusMatched
	}
	if plan.RequireOutputs {
		if confirmation.ConfirmedOutputs == nil {
			return deny(out, "structured_consent_missing_outputs", "confirmed_outputs", StatusMissing)
		}
		if *confirmation.ConfirmedOutputs < plan.Outputs {
			return deny(out, "structured_consent_outputs_mismatch", "confirmed_outputs", StatusMismatched)
		}
		out.Fields["confirmed_outputs"] = StatusMatched
	}
	return out
}

func deny(out Outcome, code, field, status string) Outcome {
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
