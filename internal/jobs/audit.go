package jobs

import "github.com/aporicho/lovart/internal/audit"

func auditJobsEvent(opts JobsOptions, event audit.Event) {
	if opts.AuditLogger == nil {
		return
	}
	if event.Source == "" {
		event.Source = "jobs"
	}
	_ = opts.AuditLogger.Write(event)
}

func auditJobsStart(operation string, state *RunState, opts JobsOptions) {
	auditJobsEvent(opts, audit.Event{
		Operation: operation,
		Source:    "jobs",
		Action:    "start",
		Decision:  "attempt",
		RiskFlags: []string{"remote_write", "paid_possible"},
		Metadata:  jobsAuditMetadata(state, opts),
	})
}

func auditJobsFinish(operation string, state *RunState, opts JobsOptions, err error) {
	decision := "success"
	var errText string
	if err != nil {
		decision = "error"
		errText = err.Error()
	}
	auditJobsEvent(opts, audit.Event{
		Operation: operation,
		Source:    "jobs",
		Action:    "finish",
		Decision:  decision,
		RiskFlags: []string{"remote_write", "paid_possible"},
		Metadata:  jobsAuditMetadata(state, opts),
		Error:     errText,
	})
}

func auditJobsGateDenied(operation string, state *RunState, opts JobsOptions, err error) {
	auditJobsEvent(opts, audit.Event{
		Operation: operation,
		Source:    "jobs",
		Action:    "gate_denied",
		Decision:  "deny",
		RiskFlags: []string{"paid_possible"},
		Metadata:  jobsAuditMetadata(state, opts),
		Error:     errorString(err),
	})
}

func auditJobsSubmit(opts JobsOptions, state *RunState, request *RemoteRequest, taskID string, err error) {
	decision := "success"
	var errText string
	if err != nil {
		decision = "error"
		errText = err.Error()
	}
	metadata := map[string]any{
		"run_dir":    state.RunDir,
		"request_id": request.RequestID,
		"model":      request.Model,
		"mode":       request.Mode,
		"attempts":   request.Attempts,
	}
	if taskID != "" {
		metadata["task_id"] = taskID
	}
	auditJobsEvent(opts, audit.Event{
		Operation: "jobs.submit_request",
		Source:    "jobs",
		Action:    "submit",
		Decision:  decision,
		RiskFlags: []string{"remote_write", "paid_possible"},
		Metadata:  metadata,
		Error:     errText,
	})
}

func jobsAuditMetadata(state *RunState, opts JobsOptions) map[string]any {
	metadata := map[string]any{
		"allow_paid":                  opts.AllowPaid,
		"max_total_credits":           opts.MaxTotalCredits,
		"confirm_paid":                opts.ConfirmPaid,
		"confirmed_max_total_credits": opts.ConfirmedMaxTotalCredits,
		"confirmed_remote_requests":   opts.ConfirmedRemoteRequests,
		"confirmed_max_active_tasks":  opts.ConfirmedMaxActiveTasks,
		"wait":                        opts.Wait,
		"download":                    opts.Download,
		"canvas":                      opts.Canvas,
		"max_active_tasks":            opts.MaxActiveTasks,
	}
	if state == nil {
		return metadata
	}
	metadata["run_dir"] = state.RunDir
	metadata["project_id"] = state.ProjectID
	metadata["logical_jobs"] = len(state.Jobs)
	metadata["remote_requests"] = len(allRequests(state))
	if state.BatchGate != nil {
		metadata["gate_allowed"] = state.BatchGate.Allowed
		metadata["total_credits"] = state.BatchGate.TotalCredits
		metadata["selected_remote_requests"] = state.BatchGate.SelectedRemoteRequests
		if state.BatchGate.StructuredConsentOutcome != nil {
			metadata["structured_consent"] = state.BatchGate.StructuredConsentOutcome.Summary()
		}
	}
	return metadata
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
