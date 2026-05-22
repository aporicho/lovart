package mcp

import "testing"

func TestJobsRunOptionsForwardStructuredPhase2Consent(t *testing.T) {
	opts := jobsRunOptions(JobsRunArgs{
		AllowPaid:                 true,
		MaxTotalCredits:           12,
		ConfirmPaid:               true,
		ConfirmedMaxTotalCredits:  12,
		ConfirmedRemoteRequests:   7,
		ConfirmedMaxActiveTasks:   9,
		ConfirmedModel:            "openai/gpt-image-2",
		ConfirmedMode:             "auto",
		ConfirmedOutputs:          7,
		ConfirmedMaxCredits:       3,
		ProjectID:                 "proj_123",
		ConfirmedProjectID:        "proj_123",
		DownloadDir:               "runs/x/images",
		ConfirmedCanonicalPath:    "/tmp/lovart/jobs/x",
		ConfirmOverwrite:          true,
		ConfirmOutsideDefaultRoot: true,
		SubmitIntervalSeconds:     1.5,
		SubmitLimit:               7,
		MaxActiveTasks:            9,
	}, nil)

	if !opts.AllowPaid || opts.MaxTotalCredits != 12 || !opts.ConfirmPaid || opts.ConfirmedMaxTotalCredits != 12 || opts.ConfirmedRemoteRequests != 7 || opts.ConfirmedMaxActiveTasks != 9 || opts.ConfirmedModel != "openai/gpt-image-2" || opts.ConfirmedMode != "auto" || opts.ConfirmedOutputs != 7 || opts.ConfirmedMaxCredits != 3 || opts.ProjectID != "proj_123" || opts.ConfirmedProjectID != "proj_123" || opts.DownloadDir != "runs/x/images" || opts.ConfirmedCanonicalPath != "/tmp/lovart/jobs/x" || !opts.ConfirmOverwrite || !opts.ConfirmOutsideDefaultRoot || opts.SubmitIntervalSeconds != 1.5 || opts.SubmitLimit != 7 || opts.MaxActiveTasks != 9 {
		t.Fatalf("jobs run options dropped structured consent fields: %#v", opts)
	}
}

func TestJobsResumeOptionsForwardStructuredPhase2Consent(t *testing.T) {
	opts := jobsResumeOptions(JobsResumeArgs{
		AllowPaid:                 true,
		MaxTotalCredits:           24,
		ConfirmPaid:               true,
		ConfirmedMaxTotalCredits:  24,
		ConfirmedRemoteRequests:   5,
		ConfirmedMaxActiveTasks:   8,
		ConfirmedModel:            "openai/gpt-image-2",
		ConfirmedMode:             "fast",
		ConfirmedOutputs:          5,
		ConfirmedMaxCredits:       4,
		ConfirmedProjectID:        "proj_resume",
		DownloadDir:               "runs/x/images",
		ConfirmedCanonicalPath:    "/tmp/lovart/resume/x",
		ConfirmOverwrite:          true,
		ConfirmOutsideDefaultRoot: true,
		RetryFailed:               true,
		SubmitIntervalSeconds:     3,
		SubmitLimit:               5,
		MaxActiveTasks:            8,
	}, nil)

	if !opts.AllowPaid || opts.MaxTotalCredits != 24 || !opts.ConfirmPaid || opts.ConfirmedMaxTotalCredits != 24 || opts.ConfirmedRemoteRequests != 5 || opts.ConfirmedMaxActiveTasks != 8 || opts.ConfirmedModel != "openai/gpt-image-2" || opts.ConfirmedMode != "fast" || opts.ConfirmedOutputs != 5 || opts.ConfirmedMaxCredits != 4 || opts.ConfirmedProjectID != "proj_resume" || opts.DownloadDir != "runs/x/images" || opts.ConfirmedCanonicalPath != "/tmp/lovart/resume/x" || !opts.ConfirmOverwrite || !opts.ConfirmOutsideDefaultRoot || !opts.RetryFailed || opts.SubmitIntervalSeconds != 3 || opts.SubmitLimit != 5 || opts.MaxActiveTasks != 8 {
		t.Fatalf("jobs resume options dropped structured consent fields: %#v", opts)
	}
}
