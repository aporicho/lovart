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

	CodeStructuredConsentProjectIDMissing          DenialCode = "structured_consent_project_id_missing"
	CodeStructuredConsentProjectIDMismatch         DenialCode = "structured_consent_project_id_mismatch"
	CodeStructuredConsentTaskIDsMissing            DenialCode = "structured_consent_task_ids_missing"
	CodeStructuredConsentTaskIDsMismatch           DenialCode = "structured_consent_task_ids_mismatch"
	CodeStructuredConsentNewNameMissing            DenialCode = "structured_consent_new_name_missing"
	CodeStructuredConsentNewNameMismatch           DenialCode = "structured_consent_new_name_mismatch"
	CodeStructuredConsentCurrentProjectMissing     DenialCode = "structured_consent_current_project_missing"
	CodeStructuredConsentProjectNameMissing        DenialCode = "structured_consent_project_name_missing"
	CodeStructuredConsentProjectNameMismatch       DenialCode = "structured_consent_project_name_mismatch"
	CodeStructuredConsentSelectAfterCreateMissing  DenialCode = "structured_consent_select_after_create_missing"
	CodeStructuredConsentSelectAfterCreateMismatch DenialCode = "structured_consent_select_after_create_mismatch"
	CodeStructuredConsentCanonicalPathMissing      DenialCode = "structured_consent_canonical_path_missing"
	CodeStructuredConsentCanonicalPathMismatch     DenialCode = "structured_consent_canonical_path_mismatch"
	CodeStructuredConsentOverwriteMissing          DenialCode = "structured_consent_overwrite_missing"
	CodeStructuredConsentOutsideRootMissing        DenialCode = "structured_consent_outside_default_root_missing"
	CodeStructuredConsentFileCountMissing          DenialCode = "structured_consent_file_count_missing"
	CodeStructuredConsentFileCountMismatch         DenialCode = "structured_consent_file_count_mismatch"
	CodeStructuredConsentCanvasWriteMissing        DenialCode = "structured_consent_canvas_write_missing"
	CodeStructuredConsentCanvasShapeIDsMissing     DenialCode = "structured_consent_canvas_shape_ids_missing"
	CodeStructuredConsentCanvasShapeIDsMismatch    DenialCode = "structured_consent_canvas_shape_ids_mismatch"
	CodeStructuredConsentCanvasLayoutMissing       DenialCode = "structured_consent_canvas_layout_missing"
	CodeStructuredConsentCanvasLayoutMismatch      DenialCode = "structured_consent_canvas_layout_mismatch"
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

// DestructiveConsent is the caller-provided object confirmation for destructive
// or destructive-like remote mutations such as project delete/rename and task
// cancel. Empty fields are treated as missing when the matching scope field is
// required.
type DestructiveConsent struct {
	ConfirmProjectID string   `json:"confirm_project_id,omitempty"`
	ConfirmTaskIDs   []string `json:"confirm_task_ids,omitempty"`
	ConfirmNewName   string   `json:"confirm_new_name,omitempty"`
}

// DestructiveScope describes the actual destructive object mutation.
type DestructiveScope struct {
	ProjectID string   `json:"project_id,omitempty"`
	TaskIDs   []string `json:"task_ids,omitempty"`
	NewName   string   `json:"new_name,omitempty"`
}

func ValidateDestructiveConsent(consent DestructiveConsent, scope DestructiveScope) ValidationResult {
	if scope.ProjectID != "" {
		if consent.ConfirmProjectID == "" {
			return denyValidation(CodeStructuredConsentProjectIDMissing, "confirm_project_id is required for destructive project mutation")
		}
		if consent.ConfirmProjectID != scope.ProjectID {
			return denyValidation(CodeStructuredConsentProjectIDMismatch, fmt.Sprintf("confirm_project_id %q does not match project_id %q", consent.ConfirmProjectID, scope.ProjectID))
		}
	}
	if len(scope.TaskIDs) > 0 {
		if len(consent.ConfirmTaskIDs) == 0 {
			return denyValidation(CodeStructuredConsentTaskIDsMissing, "confirm_task_ids is required for destructive task mutation")
		}
		if !sameStringSet(consent.ConfirmTaskIDs, scope.TaskIDs) {
			return denyValidation(CodeStructuredConsentTaskIDsMismatch, "confirm_task_ids must match the target task_ids set")
		}
	}
	if scope.NewName != "" {
		if consent.ConfirmNewName == "" {
			return denyValidation(CodeStructuredConsentNewNameMissing, "confirm_new_name is required for destructive rename mutation")
		}
		if consent.ConfirmNewName != scope.NewName {
			return denyValidation(CodeStructuredConsentNewNameMismatch, fmt.Sprintf("confirm_new_name %q does not match new_name %q", consent.ConfirmNewName, scope.NewName))
		}
	}
	return ValidationResult{Allowed: true}
}

// RemoteWriteConsent is the caller-provided confirmation for non-paid,
// non-destructive writes to Lovart remote state.
type RemoteWriteConsent struct {
	ConfirmedProjectID         string `json:"confirmed_project_id,omitempty"`
	ConfirmCurrentProject      bool   `json:"confirm_current_project,omitempty"`
	ConfirmedProjectName       string `json:"confirmed_project_name,omitempty"`
	ConfirmedSelectAfterCreate *bool  `json:"confirmed_select_after_create,omitempty"`
	ConfirmedCanvasWrite       bool   `json:"confirmed_canvas_write,omitempty"`
}

// RemoteWriteScope describes the actual remote write target. ProjectID means an
// explicit target project and takes precedence over ConfirmCurrentProject.
type RemoteWriteScope struct {
	ProjectID         string `json:"project_id,omitempty"`
	CurrentProjectID  string `json:"current_project_id,omitempty"`
	ProjectName       string `json:"project_name,omitempty"`
	SelectAfterCreate *bool  `json:"select_after_create,omitempty"`
	CanvasWrite       bool   `json:"canvas_write,omitempty"`
}

func ValidateRemoteWriteConsent(consent RemoteWriteConsent, scope RemoteWriteScope) ValidationResult {
	if scope.ProjectID != "" {
		if consent.ConfirmedProjectID == "" {
			return denyValidation(CodeStructuredConsentProjectIDMissing, "confirmed_project_id is required for explicit remote project write")
		}
		if consent.ConfirmedProjectID != scope.ProjectID {
			return denyValidation(CodeStructuredConsentProjectIDMismatch, fmt.Sprintf("confirmed_project_id %q does not match project_id %q", consent.ConfirmedProjectID, scope.ProjectID))
		}
	} else if scope.CurrentProjectID != "" {
		if consent.ConfirmedProjectID != "" {
			if consent.ConfirmedProjectID != scope.CurrentProjectID {
				return denyValidation(CodeStructuredConsentProjectIDMismatch, fmt.Sprintf("confirmed_project_id %q does not match current project_id %q", consent.ConfirmedProjectID, scope.CurrentProjectID))
			}
		} else if !consent.ConfirmCurrentProject {
			return denyValidation(CodeStructuredConsentCurrentProjectMissing, "confirm_current_project or matching confirmed_project_id is required for current-project remote write")
		}
	}
	if scope.ProjectName != "" {
		if consent.ConfirmedProjectName == "" {
			return denyValidation(CodeStructuredConsentProjectNameMissing, "confirmed_project_name is required for remote project name write")
		}
		if consent.ConfirmedProjectName != scope.ProjectName {
			return denyValidation(CodeStructuredConsentProjectNameMismatch, fmt.Sprintf("confirmed_project_name %q does not match project_name %q", consent.ConfirmedProjectName, scope.ProjectName))
		}
	}
	if scope.SelectAfterCreate != nil {
		if consent.ConfirmedSelectAfterCreate == nil {
			return denyValidation(CodeStructuredConsentSelectAfterCreateMissing, "confirmed_select_after_create is required for project create selection write")
		}
		if *consent.ConfirmedSelectAfterCreate != *scope.SelectAfterCreate {
			return denyValidation(CodeStructuredConsentSelectAfterCreateMismatch, "confirmed_select_after_create does not match select_after_create")
		}
	}
	if scope.CanvasWrite && !consent.ConfirmedCanvasWrite {
		return denyValidation(CodeStructuredConsentCanvasWriteMissing, "confirmed_canvas_write must be true for remote canvas write")
	}
	return ValidationResult{Allowed: true}
}

// LocalWriteConsent is the caller-provided confirmation for local filesystem
// writes. ConfirmedCanonicalPath must already be the normalized absolute path
// used by the caller; this package performs comparison only and no I/O.
type LocalWriteConsent struct {
	ConfirmedCanonicalPath    string `json:"confirmed_canonical_path,omitempty"`
	ConfirmOverwrite          bool   `json:"confirm_overwrite,omitempty"`
	ConfirmOutsideDefaultRoot bool   `json:"confirm_outside_default_root,omitempty"`
	ConfirmedFileCount        int    `json:"confirmed_file_count,omitempty"`
}

// LocalWriteScope describes the actual local write target.
type LocalWriteScope struct {
	CanonicalPath      string `json:"canonical_path,omitempty"`
	Overwrite          bool   `json:"overwrite,omitempty"`
	OutsideDefaultRoot bool   `json:"outside_default_root,omitempty"`
	FileCount          int    `json:"file_count,omitempty"`
	RequireFileCount   bool   `json:"require_file_count,omitempty"`
}

func ValidateLocalWriteConsent(consent LocalWriteConsent, scope LocalWriteScope) ValidationResult {
	if scope.CanonicalPath != "" {
		if consent.ConfirmedCanonicalPath == "" {
			return denyValidation(CodeStructuredConsentCanonicalPathMissing, "confirmed_canonical_path is required for local write")
		}
		if consent.ConfirmedCanonicalPath != scope.CanonicalPath {
			return denyValidation(CodeStructuredConsentCanonicalPathMismatch, fmt.Sprintf("confirmed_canonical_path %q does not match canonical_path %q", consent.ConfirmedCanonicalPath, scope.CanonicalPath))
		}
	}
	if scope.Overwrite && !consent.ConfirmOverwrite {
		return denyValidation(CodeStructuredConsentOverwriteMissing, "confirm_overwrite must be true for overwrite local write")
	}
	if scope.OutsideDefaultRoot && !consent.ConfirmOutsideDefaultRoot {
		return denyValidation(CodeStructuredConsentOutsideRootMissing, "confirm_outside_default_root must be true for writes outside the default root")
	}
	if scope.RequireFileCount {
		if consent.ConfirmedFileCount == 0 {
			return denyValidation(CodeStructuredConsentFileCountMissing, "confirmed_file_count is required for this local write")
		}
		if consent.ConfirmedFileCount != scope.FileCount {
			return denyValidation(CodeStructuredConsentFileCountMismatch, fmt.Sprintf("confirmed_file_count %d does not match file_count %d", consent.ConfirmedFileCount, scope.FileCount))
		}
	}
	return ValidationResult{Allowed: true}
}

// CanvasConsent is the caller-provided confirmation for canvas writes and
// canvas object mutations.
type CanvasConsent struct {
	ConfirmedProjectID    string   `json:"confirmed_project_id,omitempty"`
	ConfirmCurrentProject bool     `json:"confirm_current_project,omitempty"`
	ConfirmCanvasWrite    bool     `json:"confirm_canvas_write,omitempty"`
	ConfirmCanvasShapeIDs []string `json:"confirm_canvas_shape_ids,omitempty"`
	ConfirmedCanvasLayout string   `json:"confirmed_canvas_layout,omitempty"`
}

// CanvasScope describes the actual canvas mutation target.
type CanvasScope struct {
	ProjectID        string   `json:"project_id,omitempty"`
	CurrentProjectID string   `json:"current_project_id,omitempty"`
	CanvasWrite      bool     `json:"canvas_write,omitempty"`
	CanvasShapeIDs   []string `json:"canvas_shape_ids,omitempty"`
	CanvasLayout     string   `json:"canvas_layout,omitempty"`
}

func ValidateCanvasConsent(consent CanvasConsent, scope CanvasScope) ValidationResult {
	if scope.ProjectID != "" {
		if consent.ConfirmedProjectID == "" {
			return denyValidation(CodeStructuredConsentProjectIDMissing, "confirmed_project_id is required for explicit canvas project write")
		}
		if consent.ConfirmedProjectID != scope.ProjectID {
			return denyValidation(CodeStructuredConsentProjectIDMismatch, fmt.Sprintf("confirmed_project_id %q does not match project_id %q", consent.ConfirmedProjectID, scope.ProjectID))
		}
	} else if scope.CurrentProjectID != "" {
		if consent.ConfirmedProjectID != "" {
			if consent.ConfirmedProjectID != scope.CurrentProjectID {
				return denyValidation(CodeStructuredConsentProjectIDMismatch, fmt.Sprintf("confirmed_project_id %q does not match current project_id %q", consent.ConfirmedProjectID, scope.CurrentProjectID))
			}
		} else if !consent.ConfirmCurrentProject {
			return denyValidation(CodeStructuredConsentCurrentProjectMissing, "confirm_current_project or matching confirmed_project_id is required for current-project canvas write")
		}
	}
	if scope.CanvasWrite && !consent.ConfirmCanvasWrite {
		return denyValidation(CodeStructuredConsentCanvasWriteMissing, "confirm_canvas_write must be true for canvas write")
	}
	if len(scope.CanvasShapeIDs) > 0 {
		if len(consent.ConfirmCanvasShapeIDs) == 0 {
			return denyValidation(CodeStructuredConsentCanvasShapeIDsMissing, "confirm_canvas_shape_ids is required for canvas object mutation")
		}
		if !sameStringSet(consent.ConfirmCanvasShapeIDs, scope.CanvasShapeIDs) {
			return denyValidation(CodeStructuredConsentCanvasShapeIDsMismatch, "confirm_canvas_shape_ids must match the target canvas_shape_ids set")
		}
	}
	if scope.CanvasLayout != "" {
		if consent.ConfirmedCanvasLayout == "" {
			return denyValidation(CodeStructuredConsentCanvasLayoutMissing, "confirmed_canvas_layout is required for canvas layout write")
		}
		if consent.ConfirmedCanvasLayout != scope.CanvasLayout {
			return denyValidation(CodeStructuredConsentCanvasLayoutMismatch, "confirmed_canvas_layout does not match canvas_layout")
		}
	}
	return ValidationResult{Allowed: true}
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	counts := make(map[string]int, len(left))
	for _, value := range left {
		counts[value]++
	}
	for _, value := range right {
		counts[value]--
		if counts[value] < 0 {
			return false
		}
	}
	return true
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
