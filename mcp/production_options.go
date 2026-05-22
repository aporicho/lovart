package mcp

import (
	"github.com/aporicho/lovart/internal/audit"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/jobs"
)

func defaultMCPBatchOptions() jobs.JobsOptions {
	return jobs.JobsOptions{
		Wait:                  false,
		Download:              false,
		Canvas:                false,
		CanvasLayout:          jobs.CanvasLayoutFrame,
		TimeoutSeconds:        MCPWaitTimeoutSeconds,
		PollInterval:          5,
		SubmitIntervalSeconds: 2,
		MaxActiveTasks:        10,
		Detail:                "summary",
	}
}

func jobsRunOptions(args JobsRunArgs, logger *audit.Logger) jobs.JobsOptions {
	opts := defaultMCPBatchOptions()
	opts.AuditLogger = logger
	opts.AllowPaid = args.AllowPaid
	opts.MaxTotalCredits = args.MaxTotalCredits
	opts.ConfirmPaid = args.ConfirmPaid
	opts.ConfirmedMaxTotalCredits = args.ConfirmedMaxTotalCredits
	opts.ConfirmedRemoteRequests = args.ConfirmedRemoteRequests
	opts.ConfirmedMaxActiveTasks = args.ConfirmedMaxActiveTasks
	opts.ConfirmedModel = args.ConfirmedModel
	opts.ConfirmedMode = args.ConfirmedMode
	opts.ConfirmedOutputs = args.ConfirmedOutputs
	opts.ConfirmedMaxCredits = args.ConfirmedMaxCredits
	opts.ProjectID = args.ProjectID
	opts.ConfirmedProjectID = args.ConfirmedProjectID
	opts.DownloadDir = args.DownloadDir
	opts.ConfirmedCanonicalPath = args.ConfirmedCanonicalPath
	opts.ConfirmOverwrite = args.ConfirmOverwrite
	opts.ConfirmOutsideDefaultRoot = args.ConfirmOutsideDefaultRoot
	opts.SubmitIntervalSeconds = args.SubmitIntervalSeconds
	opts.SubmitLimit = args.SubmitLimit
	opts.MaxActiveTasks = args.MaxActiveTasks
	return opts
}

func jobsResumeOptions(args JobsResumeArgs, logger *audit.Logger) jobs.JobsOptions {
	opts := defaultMCPBatchOptions()
	opts.AuditLogger = logger
	opts.AllowPaid = args.AllowPaid
	opts.MaxTotalCredits = args.MaxTotalCredits
	opts.ConfirmPaid = args.ConfirmPaid
	opts.ConfirmedMaxTotalCredits = args.ConfirmedMaxTotalCredits
	opts.ConfirmedRemoteRequests = args.ConfirmedRemoteRequests
	opts.ConfirmedMaxActiveTasks = args.ConfirmedMaxActiveTasks
	opts.ConfirmedModel = args.ConfirmedModel
	opts.ConfirmedMode = args.ConfirmedMode
	opts.ConfirmedOutputs = args.ConfirmedOutputs
	opts.ConfirmedMaxCredits = args.ConfirmedMaxCredits
	opts.ConfirmedProjectID = args.ConfirmedProjectID
	opts.DownloadDir = args.DownloadDir
	opts.ConfirmedCanonicalPath = args.ConfirmedCanonicalPath
	opts.ConfirmOverwrite = args.ConfirmOverwrite
	opts.ConfirmOutsideDefaultRoot = args.ConfirmOutsideDefaultRoot
	opts.RetryFailed = args.RetryFailed
	opts.SubmitIntervalSeconds = args.SubmitIntervalSeconds
	opts.SubmitLimit = args.SubmitLimit
	opts.MaxActiveTasks = args.MaxActiveTasks
	return opts
}

func generationOptions(args GenerateArgs, cid string) generation.Options {
	return generation.Options{
		Mode:                      args.Mode,
		AllowPaid:                 args.AllowPaid,
		MaxCredits:                args.MaxCredits,
		ConfirmPaid:               args.ConfirmPaid,
		ConfirmedMaxCredits:       args.ConfirmedMaxCredits,
		ConfirmedModel:            args.ConfirmedModel,
		ConfirmedMode:             args.ConfirmedMode,
		ConfirmedOutputs:          args.ConfirmedOutputs,
		ConfirmedRemoteRequests:   args.ConfirmedRemoteRequests,
		ProjectID:                 args.ProjectID,
		ConfirmedProjectID:        args.ConfirmedProjectID,
		CID:                       cid,
		Wait:                      args.Wait,
		Download:                  args.Download,
		DownloadDir:               args.DownloadDir,
		Overwrite:                 args.Overwrite,
		ConfirmedCanonicalPath:    args.ConfirmedCanonicalPath,
		ConfirmOverwrite:          args.ConfirmOverwrite,
		ConfirmOutsideDefaultRoot: args.ConfirmOutsideDefaultRoot,
		Canvas:                    args.Canvas,
		CanvasLayout:              args.CanvasLayout,
		ConfirmCanvasWrite:        args.ConfirmCanvasWrite,
		ConfirmedCanvasLayout:     args.ConfirmedCanvasLayout,
	}
}
