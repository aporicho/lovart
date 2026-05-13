package mcp

import (
	"context"
	stderrors "errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/aporicho/lovart/internal/auth"
	internalbrowser "github.com/aporicho/lovart/internal/browser"
	"github.com/aporicho/lovart/internal/config"
	"github.com/aporicho/lovart/internal/connector"
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
	sharedvalidation "github.com/aporicho/lovart/internal/validation"
	"github.com/aporicho/lovart/internal/version"
)

// ProductionExecutor executes MCP tools against the Lovart runtime.
type ProductionExecutor struct{}

var openProjectURL = func(url string) error {
	return exec.Command("open", url).Start()
}

func lovartErrorEnvelope(err error, fallback string) envelope.Envelope {
	if lovartErr, ok := err.(*errors.LovartError); ok {
		return envelope.Err(lovartErr.Code, lovartErr.Message, lovartErr.Details)
	}
	return envelope.Err(errors.CodeInternal, fallback, map[string]any{"error": err.Error()})
}

// AuthStatus reports credential presence without exposing secret values.
func (ProductionExecutor) AuthStatus(ctx context.Context) envelope.Envelope {
	return okLocal(auth.GetStatus(), true)
}

// AuthLogin starts browser-extension login and waits for the approved session.
func (ProductionExecutor) AuthLogin(ctx context.Context, args AuthLoginArgs) envelope.Envelope {
	timeout := time.Duration(args.TimeoutSeconds * float64(time.Second))
	result, err := auth.RunBrowserExtensionLogin(ctx, auth.BrowserLoginOptions{
		Timeout:              timeout,
		OpenBrowser:          internalbrowser.OpenURL,
		RequireBrowserOpened: true,
	})
	if err != nil {
		return lovartErrorEnvelope(err, "auth login failed")
	}
	return okLocal(map[string]any{
		"authenticated":  result.Authenticated,
		"status":         result.Status,
		"callback_port":  result.CallbackPort,
		"expires_at":     result.ExpiresAt,
		"opened_browser": result.OpenedBrowser,
		"next_steps":     result.NextSteps,
	}, false)
}

// ExtensionStatus reports whether the local Connector extension files exist.
func (ProductionExecutor) ExtensionStatus(ctx context.Context, args ExtensionStatusArgs) envelope.Envelope {
	result, err := connector.Status(connector.Options{ExtensionDir: args.ExtensionDir})
	if err != nil {
		return lovartErrorEnvelope(err, "extension status failed")
	}
	return okLocal(result, true)
}

// ExtensionInstall prepares Connector extension files for Chrome's Load unpacked flow.
func (ProductionExecutor) ExtensionInstall(ctx context.Context, args ExtensionInstallArgs) envelope.Envelope {
	result, err := connector.Install(connector.Options{
		SourceDir:    args.SourceDir,
		ExtensionDir: args.ExtensionDir,
		DryRun:       args.DryRun,
		Open:         args.Open,
		OpenURL:      internalbrowser.OpenURL,
	})
	if err != nil {
		return lovartErrorEnvelope(err, "extension install failed")
	}
	return okLocal(result, true)
}

// ExtensionOpen opens Chrome extension management for manual loading.
func (ProductionExecutor) ExtensionOpen(ctx context.Context, args ExtensionOpenArgs) envelope.Envelope {
	result, err := connector.Open(connector.Options{
		ExtensionDir: args.ExtensionDir,
		OpenURL:      internalbrowser.OpenURL,
	})
	if err != nil {
		return lovartErrorEnvelope(err, "extension open failed")
	}
	return okLocal(result, true)
}

// Setup runs local readiness checks without exposing secrets.
func (ProductionExecutor) Setup(ctx context.Context, args SetupArgs) envelope.Envelope {
	readiness := setup.Readiness()
	data := map[string]any{
		"version":   version.Version,
		"readiness": readiness,
		"status":    "ok",
		"ready":     readiness.Ready,
	}
	return okLocal(data, true)
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

// Balance returns the current account balance.
func (ProductionExecutor) Balance(ctx context.Context) envelope.Envelope {
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	balance, err := pricing.Balance(ctx, client)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "fetch balance", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{"balance": balance})
}

// ProjectCurrent returns the selected project context without exposing auth values.
func (ProductionExecutor) ProjectCurrent(ctx context.Context) envelope.Envelope {
	pc, err := auth.LoadProjectContext()
	if err != nil {
		return envelope.Err(errors.CodeInputError, "no project context", map[string]any{
			"error":               err.Error(),
			"recommended_actions": []string{"run `lovart project list`", "run `lovart project select <project_id>`"},
		})
	}
	return okLocal(map[string]any{
		"project_id":            pc.ProjectID,
		"project_context_ready": pc.ProjectID != "" && pc.CID != "",
	}, true)
}

// ProjectList lists projects available to the current account.
func (ProductionExecutor) ProjectList(ctx context.Context) envelope.Envelope {
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	projects, err := project.List(ctx, client)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "list projects", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{"count": len(projects), "projects": projects})
}

// ProjectCreate creates a new Lovart project and selects it by default.
func (ProductionExecutor) ProjectCreate(ctx context.Context, args ProjectCreateArgs) envelope.Envelope {
	pc, _ := auth.LoadProjectContext()
	cid := ""
	if pc != nil {
		cid = pc.CID
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	createdProject, err := project.Create(ctx, client, cid, args.Name)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "create project", map[string]any{"error": err.Error()})
	}
	if args.Select {
		if err := auth.SetProjectContext(createdProject.ID, cid); err != nil {
			return envelope.Err(errors.CodeInternal, "set project", map[string]any{"error": err.Error()})
		}
	}
	return okSubmit(map[string]any{
		"created":               true,
		"selected":              args.Select,
		"project_id":            createdProject.ID,
		"project_name":          createdProject.Name,
		"project_context_ready": args.Select && cid != "",
		"canvas_url":            canvasURL(createdProject.ID),
	}, true)
}

// ProjectSelect stores the selected project for future generation calls.
func (ProductionExecutor) ProjectSelect(ctx context.Context, args ProjectSelectArgs) envelope.Envelope {
	pc, _ := auth.LoadProjectContext()
	cid := ""
	if pc != nil {
		cid = pc.CID
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	projects, err := project.List(ctx, client)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "list projects", map[string]any{"error": err.Error()})
	}
	selectedProject, ok := project.FindByID(projects, args.ProjectID)
	if !ok {
		return envelope.Err(errors.CodeInputError, "project not found", map[string]any{
			"project_id": args.ProjectID,
			"recommended_actions": []string{
				"run `lovart project list`",
				"select a project_id from the returned projects",
			},
		})
	}
	if err := auth.SetProjectContext(args.ProjectID, cid); err != nil {
		return envelope.Err(errors.CodeInternal, "set project", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{
		"selected":              true,
		"project_id":            selectedProject.ID,
		"project_name":          selectedProject.Name,
		"project_context_ready": cid != "",
	})
}

// ProjectShow returns project details without exposing auth values.
func (ProductionExecutor) ProjectShow(ctx context.Context, args ProjectShowArgs) envelope.Envelope {
	projectID, cid, env := projectIDAndOptionalCID(args.ProjectID)
	if env != nil {
		return *env
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	p, err := project.Query(ctx, client, projectID, cid)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "query project", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{
		"project_id":   p.ID,
		"project_name": p.Name,
		"canvas_url":   canvasURL(p.ID),
	})
}

// ProjectOpen opens the project in the local browser.
func (ProductionExecutor) ProjectOpen(ctx context.Context, args ProjectOpenArgs) envelope.Envelope {
	projectID, env := projectIDOrCurrent(args.ProjectID)
	if env != nil {
		return *env
	}
	url := canvasURL(projectID)
	if err := openProjectURL(url); err != nil {
		return envelope.Err(errors.CodeInternal, "open project", map[string]any{
			"project_id": projectID,
			"canvas_url": url,
			"error":      err.Error(),
		})
	}
	return okLocal(map[string]any{
		"opened":     true,
		"project_id": projectID,
		"canvas_url": url,
		"url":        url,
	}, true)
}

// ProjectRename renames a Lovart project.
func (ProductionExecutor) ProjectRename(ctx context.Context, args ProjectRenameArgs) envelope.Envelope {
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	if err := project.Rename(ctx, client, args.ProjectID, args.NewName); err != nil {
		return envelope.Err(errors.CodeInternal, "rename project", map[string]any{"error": err.Error()})
	}
	return okSubmit(map[string]any{
		"renamed":      true,
		"project_id":   args.ProjectID,
		"project_name": args.NewName,
		"canvas_url":   canvasURL(args.ProjectID),
	}, true)
}

// ProjectDelete deletes a Lovart project and clears local selection when needed.
func (ProductionExecutor) ProjectDelete(ctx context.Context, args ProjectDeleteArgs) envelope.Envelope {
	if args.ConfirmProjectID != args.ProjectID {
		return envelope.Err(errors.CodeInputError, "confirm_project_id must match project_id", nil)
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	if err := project.Delete(ctx, client, args.ProjectID); err != nil {
		return envelope.Err(errors.CodeInternal, "delete project", map[string]any{"error": err.Error()})
	}

	clearedCurrent := false
	if pc, _ := auth.LoadProjectContext(); pc != nil && pc.ProjectID == args.ProjectID {
		if err := auth.ClearProjectContext(); err != nil {
			return envelope.Err(errors.CodeInternal, "clear selected project", map[string]any{"error": err.Error()})
		}
		clearedCurrent = true
	}
	return okSubmit(map[string]any{
		"deleted":         true,
		"project_id":      args.ProjectID,
		"cleared_current": clearedCurrent,
	}, true)
}

// ProjectRepairCanvas repairs the selected or provided project's canvas.
func (ProductionExecutor) ProjectRepairCanvas(ctx context.Context, args ProjectRepairCanvasArgs) envelope.Envelope {
	projectID, cid, env := projectIDAndOptionalCID(args.ProjectID)
	if env != nil {
		return *env
	}
	client, err := newSignedClient(ctx)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "setup client", map[string]any{"error": err.Error()})
	}
	result, err := project.RepairCanvas(ctx, client, projectID, cid)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "repair canvas", map[string]any{"error": err.Error()})
	}
	return okSubmit(map[string]any{
		"project_id": projectID,
		"repair":     result,
	}, result != nil && result.Changed)
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
	result, err := pricing.QuoteWithOptions(ctx, client, args.Model, args.Body, pricing.QuoteOptions{Mode: args.Mode})
	if err != nil {
		return envelope.Err(errors.CodeInternal, "quote failed", map[string]any{"error": err.Error()})
	}
	return okPreflight(map[string]any{
		"price":           result.Price,
		"balance":         result.Balance,
		"price_detail":    result.PriceDetail,
		"pricing_context": result.PricingContext,
	})
}

// Generate submits a single generation request.
func (ProductionExecutor) Generate(ctx context.Context, args GenerateArgs) envelope.Envelope {
	return generate(ctx, args)
}

// JobsRun runs a complete batch.
func (ProductionExecutor) JobsRun(ctx context.Context, args JobsRunArgs) envelope.Envelope {
	opts := defaultMCPBatchOptions()
	opts.AllowPaid = args.AllowPaid
	opts.MaxTotalCredits = args.MaxTotalCredits
	opts.ProjectID = args.ProjectID
	opts.DownloadDir = args.DownloadDir
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
			"project_id":            opts.ProjectID,
			"project_context_ready": false,
			"recommended_actions": []string{
				"run `lovart auth login`",
				"run `lovart project list`",
				"run `lovart project select <project_id>`",
			},
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
	opts := defaultMCPBatchOptions()
	opts.AllowPaid = args.AllowPaid
	opts.MaxTotalCredits = args.MaxTotalCredits
	opts.DownloadDir = args.DownloadDir
	opts.RetryFailed = args.RetryFailed
	applyProjectContext(&opts)
	remote, env := newJobsRemote(ctx)
	if env != nil {
		return *env
	}
	result, err := jobs.ResumeJobs(ctx, remote, args.RunDir, opts)
	return jobsResultEnvelope(result, err, "resume jobs", okSubmit)
}

func defaultMCPBatchOptions() jobs.JobsOptions {
	return jobs.JobsOptions{
		Wait:           true,
		Download:       true,
		Canvas:         true,
		CanvasLayout:   jobs.CanvasLayoutFrame,
		TimeoutSeconds: MCPWaitTimeoutSeconds,
		PollInterval:   5,
		Detail:         "summary",
	}
}

func generate(ctx context.Context, args GenerateArgs) envelope.Envelope {
	if validation := registry.ValidateRequest(args.Model, args.Body); !validation.OK {
		return envelope.Err(sharedvalidation.RequestErrorCode(validation), "request body failed schema validation", map[string]any{
			"validation":          validation,
			"recommended_actions": sharedvalidation.RequestRecommendedActions(validation),
		})
	}
	cid := ""
	if pc, err := auth.LoadProjectContext(); err == nil && pc != nil {
		if args.ProjectID == "" {
			args.ProjectID = pc.ProjectID
		}
		cid = pc.CID
	}
	if args.ProjectID == "" || cid == "" {
		return envelope.Err(errors.CodeInputError, "missing project context", map[string]any{
			"project_id":            args.ProjectID,
			"project_context_ready": false,
			"recommended_actions": []string{
				"run `lovart auth login`",
				"run `lovart project list`",
				"run `lovart project select <project_id>`",
			},
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
		CID:        cid,
		Wait:       args.Wait,
		Download:   args.Download,
	}
	preflight, err := generation.Preflight(ctx, client, args.Model, args.Body, opts)
	if err != nil {
		return envelope.Err(errors.CodeInternal, "preflight error", map[string]any{"error": err.Error()})
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
	var warnings []string
	if args.Wait {
		var failure *envelope.Envelope
		warnings, failure = addCompletedGenerationArtifacts(ctx, client, output, result.TaskID, args, cid)
		if failure != nil {
			return *failure
		}
	}
	env := okSubmit(output, true)
	env.Warnings = warnings
	return env
}

func addCompletedGenerationArtifacts(ctx context.Context, client *http.Client, output map[string]any, taskID string, args GenerateArgs, cid string) ([]string, *envelope.Envelope) {
	var warnings []string
	task, err := generation.Wait(ctx, client, taskID)
	if err != nil {
		output["poll_error"] = err.Error()
		warnings = append(warnings, "task was submitted but polling failed; rerun a status or resume-capable command when available")
		return warnings, nil
	}
	output["task"] = task
	output["status"] = task["status"]
	if task["status"] == "failed" {
		env := envelope.Err(errors.CodeTaskFailed, "generation task failed", map[string]any{
			"task_id": taskID,
			"task":    task,
		})
		return nil, &env
	}
	if task["status"] != "completed" {
		return warnings, nil
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
			warnings = append(warnings, "artifacts were generated but download failed; retry artifact download when available")
		} else {
			output["downloads"] = downloadResult.Files
			if downloadResult.IndexError != "" {
				output["download_index_error"] = downloadResult.IndexError
				warnings = append(warnings, "artifacts were downloaded but the download index could not be fully written")
			}
		}
	}
	if args.Canvas && args.ProjectID != "" && cid != "" {
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
			return warnings, nil
		}
		if err := project.AddToCanvas(ctx, client, args.ProjectID, cid, images); err != nil {
			output["canvas_error"] = err.Error()
			warnings = append(warnings, "artifacts were generated but project canvas writeback failed")
		} else {
			output["canvas_updated"] = true
		}
	}
	return warnings, nil
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

func projectIDOrCurrent(projectID string) (string, *envelope.Envelope) {
	if projectID != "" {
		return projectID, nil
	}
	pc, err := auth.LoadProjectContext()
	if err != nil || pc == nil || pc.ProjectID == "" {
		env := missingProjectContextEnvelope(projectID, err)
		return "", &env
	}
	return pc.ProjectID, nil
}

func projectIDAndOptionalCID(projectID string) (string, string, *envelope.Envelope) {
	pc, err := auth.LoadProjectContext()
	cid := ""
	if pc != nil {
		cid = pc.CID
		if projectID == "" {
			projectID = pc.ProjectID
		}
	}
	if projectID == "" {
		env := missingProjectContextEnvelope(projectID, err)
		return "", "", &env
	}
	return projectID, cid, nil
}

func missingProjectContextEnvelope(projectID string, err error) envelope.Envelope {
	details := map[string]any{
		"project_id":            projectID,
		"project_context_ready": false,
		"recommended_actions": []string{
			"run `lovart auth login`",
			"run `lovart project list`",
			"run `lovart project select <project_id>`",
		},
	}
	if err != nil {
		details["error"] = err.Error()
	}
	return envelope.Err(errors.CodeInputError, "missing project context", details)
}

func canvasURL(projectID string) string {
	return fmt.Sprintf("https://www.lovart.ai/canvas?projectId=%s", projectID)
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
		return envelope.Err(gateErr.Code, "batch gate blocked", map[string]any{
			"batch_gate": gateErr.Gate,
			"run_dir":    gateErr.RunDir,
			"state_file": gateErr.StateFile,
		})
	}
	return envelope.Err(errors.CodeInternal, message, map[string]any{"error": err.Error()})
}

func hasSubmittedJobs(result *jobs.BatchResult) bool {
	counts := result.Summary.RemoteStatusCounts
	return counts[jobs.StatusSubmitted] > 0 || counts[jobs.StatusRunning] > 0 || counts[jobs.StatusCompleted] > 0 || counts[jobs.StatusDownloaded] > 0
}
