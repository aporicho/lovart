package mcp

import (
	"context"
	stderrors "errors"

	"github.com/aporicho/lovart/internal/auth"
	"github.com/aporicho/lovart/internal/config"
	"github.com/aporicho/lovart/internal/discovery"
	"github.com/aporicho/lovart/internal/downloads"
	"github.com/aporicho/lovart/internal/envelope"
	"github.com/aporicho/lovart/internal/errors"
	"github.com/aporicho/lovart/internal/generation"
	"github.com/aporicho/lovart/internal/http"
	"github.com/aporicho/lovart/internal/jobs"
	"github.com/aporicho/lovart/internal/pricing"
	"github.com/aporicho/lovart/internal/project"
	"github.com/aporicho/lovart/internal/registry"
	"github.com/aporicho/lovart/internal/setup"
	"github.com/aporicho/lovart/internal/signing"
	"github.com/aporicho/lovart/internal/update"
	sharedvalidation "github.com/aporicho/lovart/internal/validation"
	"github.com/aporicho/lovart/internal/version"
)

// ProductionExecutor executes MCP tools against the Lovart runtime.
type ProductionExecutor struct{}

// Setup runs online setup checks plus local readiness.
func (ProductionExecutor) Setup(ctx context.Context, args SetupArgs) envelope.Envelope {
	readiness := setup.Readiness()
	data := map[string]any{
		"version":   version.Version,
		"readiness": readiness,
	}
	updateStatus, err := update.Check(ctx)
	if err != nil {
		data["status"] = "network_unavailable"
		data["ready"] = false
		data["update_error"] = map[string]any{
			"error":               err.Error(),
			"recommended_actions": []string{"check network connectivity to www.lovart.ai", "rerun `lovart setup`"},
		}
		return okPreflight(data, true)
	}
	data["status"] = "ok"
	data["ready"] = readiness.Ready
	data["update"] = updateStatus
	return okPreflight(data, true)
}

// Models lists known models from registry or remote metadata.
func (ProductionExecutor) Models(ctx context.Context, args ModelsArgs) envelope.Envelope {
	if !args.Refresh {
		reg, err := registry.Load()
		if err != nil {
			return envelope.Err(errors.CodeMetadataStale, "failed to load model registry", map[string]any{
				"error":               err.Error(),
				"recommended_actions": []string{"run `lovart update sync --all`"},
			})
		}
		models := make([]map[string]any, 0, len(reg.Models()))
		for _, record := range reg.Models() {
			models = append(models, map[string]any{
				"model":        record.Model,
				"display_name": record.DisplayName,
				"type":         record.Type,
			})
		}
		return okLocal(map[string]any{"models": models, "count": len(models), "source": "registry"}, true)
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	models, err := discovery.List(ctx, client, true)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "failed to fetch model list", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{"source": "remote", "count": len(models), "models": models}, false)
}

// Config returns legal model config.
func (ProductionExecutor) Config(ctx context.Context, args ConfigArgs) envelope.Envelope {
	result, err := config.ForModel(args.Model)
	if err != nil {
		return envelope.Err(errors.CodeSchemaInvalid, "config lookup failed", map[string]any{"error": err.Error()})
	}
	if !args.IncludeAll {
		visible := result.Fields[:0]
		for _, field := range result.Fields {
			if field.Type != "" {
				visible = append(visible, field)
			}
		}
		result.Fields = visible
	}
	return okLocal(result, true)
}

// Quote fetches live pricing for a single request.
func (ProductionExecutor) Quote(ctx context.Context, args QuoteArgs) envelope.Envelope {
	if validation := registry.ValidateRequest(args.Model, args.Body); !validation.OK {
		return envelope.Err(sharedvalidation.RequestErrorCode(validation), "request body failed schema validation", map[string]any{
			"validation":          validation,
			"recommended_actions": sharedvalidation.RequestRecommendedActions(validation),
		})
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	result, err := pricing.Quote(ctx, client, args.Model, args.Body)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "quote failed", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{
		"price":        result.Price,
		"balance":      result.Balance,
		"price_detail": result.PriceDetail,
	})
}

// GenerateDryRun preflights a request without submitting.
func (ProductionExecutor) GenerateDryRun(ctx context.Context, args GenerateArgs) envelope.Envelope {
	return generate(ctx, args, true)
}

// Generate submits a single generation request.
func (ProductionExecutor) Generate(ctx context.Context, args GenerateArgs) envelope.Envelope {
	return generate(ctx, args, false)
}

// JobsQuote quotes a jobs file.
func (ProductionExecutor) JobsQuote(ctx context.Context, args JobsQuoteArgs) envelope.Envelope {
	preparedJobs, validationErr, err := jobs.PrepareJobsFile(args.JobsFile)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "read jobs file", map[string]any{"error": err.Error()})
	}
	if validationErr != nil {
		return jobValidationEnvelope(validationErr)
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	result, err := jobs.QuotePreparedJobs(ctx, client, preparedJobs, true)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "quote jobs", map[string]any{"error": err.Error()})
	}
	return okPreflight(result)
}

// JobsDryRun validates, quotes, and gates a batch without submitting.
func (ProductionExecutor) JobsDryRun(ctx context.Context, args JobsDryRunArgs) envelope.Envelope {
	opts := jobs.JobsOptions{
		OutDir:          args.OutDir,
		AllowPaid:       args.AllowPaid,
		MaxTotalCredits: args.MaxTotalCredits,
		Detail:          args.Detail,
	}
	state, validationErr, err := jobs.PrepareRun(args.JobsFile, opts)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "read jobs file", map[string]any{"error": err.Error()})
	}
	if validationErr != nil {
		return jobValidationEnvelope(validationErr)
	}
	remote, env := newJobsRemote(ctx)
	if env != nil {
		return *env
	}
	result, err := jobs.DryRunPreparedJobs(ctx, remote, state, opts)
	return jobsResultEnvelope(result, err, "dry-run jobs", func(data any, _ bool) envelope.Envelope {
		return okPreflightSubmission(data, false)
	})
}

// JobsRun submits a batch.
func (ProductionExecutor) JobsRun(ctx context.Context, args JobsRunArgs) envelope.Envelope {
	opts := jobs.JobsOptions{
		OutDir:               args.OutDir,
		AllowPaid:            args.AllowPaid,
		MaxTotalCredits:      args.MaxTotalCredits,
		Wait:                 args.Wait,
		Download:             args.Download,
		Canvas:               args.Canvas,
		CanvasLayout:         args.CanvasLayout,
		DownloadDir:          args.DownloadDir,
		DownloadDirTemplate:  args.DownloadDirTemplate,
		DownloadFileTemplate: args.DownloadFileTemplate,
		TimeoutSeconds:       args.TimeoutSeconds,
		PollInterval:         args.PollInterval,
		ProjectID:            args.ProjectID,
		CID:                  args.CID,
		Detail:               args.Detail,
	}
	applyProjectContext(&opts)
	state, validationErr, err := jobs.PrepareRun(args.JobsFile, opts)
	if err != nil {
		return envelope.Err(errors.CodeInputError, "read jobs file", map[string]any{"error": err.Error()})
	}
	if validationErr != nil {
		return jobValidationEnvelope(validationErr)
	}
	state.ProjectID = opts.ProjectID
	state.CID = opts.CID
	if opts.ProjectID == "" || opts.CID == "" {
		return envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
			"project_id":         opts.ProjectID,
			"cid_present":        opts.CID != "",
			"recommended_action": "pass project_id and cid, or run `lovart project select <project_id>`",
		})
	}
	remote, env := newJobsRemote(ctx)
	if env != nil {
		return *env
	}
	result, err := jobs.RunPreparedJobs(ctx, remote, state, opts)
	return jobsResultEnvelope(result, err, "run jobs", okSubmit)
}

// JobsStatus reads local batch state, optionally refreshing active tasks.
func (ProductionExecutor) JobsStatus(ctx context.Context, args JobsStatusArgs) envelope.Envelope {
	opts := jobs.JobsOptions{Detail: args.Detail, Refresh: args.Refresh}
	var remote jobs.RemoteClient
	if args.Refresh {
		var env *envelope.Envelope
		remote, env = newJobsRemote(ctx)
		if env != nil {
			return *env
		}
	}
	result, err := jobs.StatusJobs(ctx, remote, args.RunDir, opts)
	if args.Refresh {
		return jobsResultEnvelope(result, err, "status jobs", func(data any, _ bool) envelope.Envelope {
			return okPreflight(data)
		})
	}
	return jobsResultEnvelope(result, err, "status jobs", func(data any, _ bool) envelope.Envelope {
		return okLocal(data)
	})
}

// JobsResume resumes a saved batch state.
func (ProductionExecutor) JobsResume(ctx context.Context, args JobsResumeArgs) envelope.Envelope {
	opts := jobs.JobsOptions{
		AllowPaid:            args.AllowPaid,
		MaxTotalCredits:      args.MaxTotalCredits,
		Wait:                 args.Wait,
		Download:             args.Download,
		Canvas:               args.Canvas,
		CanvasLayout:         args.CanvasLayout,
		DownloadDir:          args.DownloadDir,
		DownloadDirTemplate:  args.DownloadDirTemplate,
		DownloadFileTemplate: args.DownloadFileTemplate,
		RetryFailed:          args.RetryFailed,
		TimeoutSeconds:       args.TimeoutSeconds,
		PollInterval:         args.PollInterval,
		ProjectID:            args.ProjectID,
		CID:                  args.CID,
		Detail:               args.Detail,
	}
	applyProjectContext(&opts)
	remote, env := newJobsRemote(ctx)
	if env != nil {
		return *env
	}
	result, err := jobs.ResumeJobs(ctx, remote, args.RunDir, opts)
	return jobsResultEnvelope(result, err, "resume jobs", okSubmit)
}

func generate(ctx context.Context, args GenerateArgs, dryRun bool) envelope.Envelope {
	if validation := registry.ValidateRequest(args.Model, args.Body); !validation.OK {
		return envelope.Err(sharedvalidation.RequestErrorCode(validation), "request body failed schema validation", map[string]any{
			"validation":          validation,
			"recommended_actions": sharedvalidation.RequestRecommendedActions(validation),
		})
	}
	if pc, err := auth.LoadProjectContext(); err == nil && pc != nil {
		if args.ProjectID == "" {
			args.ProjectID = pc.ProjectID
		}
		if args.CID == "" {
			args.CID = pc.CID
		}
	}
	if !dryRun && (args.ProjectID == "" || args.CID == "") {
		return envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
			"project_id":         args.ProjectID,
			"cid_present":        args.CID != "",
			"recommended_action": "pass project_id and cid, or run `lovart project select <project_id>`",
		})
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	opts := generation.Options{
		Mode:       args.Mode,
		AllowPaid:  args.AllowPaid,
		MaxCredits: args.MaxCredits,
		ProjectID:  args.ProjectID,
		CID:        args.CID,
		Wait:       args.Wait,
		Download:   args.Download,
	}
	preflight, err := generation.Preflight(ctx, client, args.Model, args.Body, opts)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "preflight error", map[string]any{"error": err.Error()})
	}
	if dryRun {
		return okPreflightSubmission(map[string]any{
			"submitted":  false,
			"preflight":  preflight,
			"project_id": args.ProjectID,
		}, false)
	}
	if !preflight.CanSubmit {
		return envelope.Err(errors.CodeCreditRisk, "cannot submit", map[string]any{"preflight": preflight})
	}
	result, err := generation.Submit(ctx, client, args.Model, args.Body, opts)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "submit failed", map[string]any{"error": err.Error()})
	}
	output := map[string]any{
		"submitted":  true,
		"task_id":    result.TaskID,
		"status":     result.Status,
		"preflight":  preflight,
		"project_id": args.ProjectID,
	}
	if args.Wait {
		addCompletedGenerationArtifacts(ctx, client, output, result.TaskID, args)
	}
	return okSubmit(output, true)
}

func addCompletedGenerationArtifacts(ctx context.Context, client *http.Client, output map[string]any, taskID string, args GenerateArgs) {
	task, err := generation.Wait(ctx, client, taskID)
	if err != nil {
		output["poll_error"] = err.Error()
		return
	}
	output["task"] = task
	output["status"] = task["status"]
	if task["status"] != "completed" {
		return
	}
	if args.Download {
		downloadResult, err := downloads.DownloadArtifacts(ctx, downloads.ArtifactsFromTask(task), downloads.Options{
			RootDir:      args.DownloadDir,
			DirTemplate:  args.DownloadDirTemplate,
			FileTemplate: args.DownloadFileTemplate,
			TaskID:       taskID,
			Context: downloads.JobContext{
				Model: args.Model,
				Mode:  args.Mode,
				Body:  args.Body,
			},
		})
		if err != nil {
			output["download_error"] = err.Error()
		} else {
			output["downloads"] = downloadResult.Files
			if downloadResult.IndexError != "" {
				output["download_index_error"] = downloadResult.IndexError
			}
		}
	}
	if args.Canvas && args.ProjectID != "" && args.CID != "" {
		details, _ := task["artifact_details"].([]map[string]any)
		images := make([]project.CanvasImage, 0, len(details))
		for _, detail := range details {
			url, _ := detail["url"].(string)
			width, _ := detail["width"].(float64)
			height, _ := detail["height"].(float64)
			if url == "" {
				continue
			}
			if width == 0 {
				width = 1024
			}
			if height == 0 {
				height = 1024
			}
			images = append(images, project.CanvasImage{TaskID: taskID, URL: url, Width: int(width), Height: int(height)})
		}
		if len(images) == 0 {
			return
		}
		if err := project.AddToCanvas(ctx, client, args.ProjectID, args.CID, images); err != nil {
			output["canvas_error"] = err.Error()
		} else {
			output["canvas_updated"] = true
		}
	}
}

func newSignedClient(ctx context.Context) (*http.Client, error) {
	creds, err := auth.Load()
	if err != nil {
		return nil, err
	}
	signer, err := signing.NewSigner()
	if err != nil {
		return nil, err
	}
	client := http.NewClient(creds, signer)
	if err := client.SyncTime(ctx); err != nil {
		return nil, err
	}
	return client, nil
}

func newJobsRemote(ctx context.Context) (jobs.RemoteClient, *envelope.Envelope) {
	client, err := newSignedClient(ctx)
	if err != nil {
		env := envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
		return nil, &env
	}
	return jobs.NewHTTPRemoteClient(client), nil
}

func applyProjectContext(opts *jobs.JobsOptions) {
	pc, _ := auth.LoadProjectContext()
	if pc == nil {
		return
	}
	if opts.ProjectID == "" {
		opts.ProjectID = pc.ProjectID
	}
	if opts.CID == "" {
		opts.CID = pc.CID
	}
}

func jobValidationEnvelope(validationErr *jobs.ValidationError) envelope.Envelope {
	return envelope.Err(sharedvalidation.JobsErrorCode(validationErr), "jobs file failed schema validation", map[string]any{
		"validation":          validationErr,
		"recommended_actions": sharedvalidation.JobsRecommendedActions(validationErr),
	})
}

func jobsResultEnvelope(result *jobs.BatchResult, err error, message string, okFn func(any, bool) envelope.Envelope) envelope.Envelope {
	if err == nil {
		return okFn(result, result != nil && hasSubmittedJobs(result))
	}
	var validationErr *jobs.ValidationError
	if stderrors.As(err, &validationErr) {
		return jobValidationEnvelope(validationErr)
	}
	var gateErr *jobs.GateError
	if stderrors.As(err, &gateErr) {
		return envelope.Err(gateErr.Code, "batch gate blocked", map[string]any{"batch_gate": gateErr.Gate})
	}
	return envelope.Err(errors.CodeInternal, message, map[string]any{"error": err.Error()})
}

func hasSubmittedJobs(result *jobs.BatchResult) bool {
	counts := result.Summary.RemoteStatusCounts
	return counts[jobs.StatusSubmitted] > 0 || counts[jobs.StatusRunning] > 0 || counts[jobs.StatusCompleted] > 0 || counts[jobs.StatusDownloaded] > 0
}
