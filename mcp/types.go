// Package mcp implements the Lovart stdio MCP server.
package mcp

import (
	"context"

	"github.com/aporicho/lovart/internal/envelope"
)

const (
	ProtocolVersion       = "2024-11-05"
	ServerName            = "lovart"
	MCPWaitTimeoutSeconds = 90.0
)

const (
	executionLocal     = "local"
	executionPreflight = "preflight"
	executionSubmit    = "submit"
)

// SetupArgs configures lovart_setup.
type SetupArgs struct{}

// ModelsArgs configures lovart_models.
type ModelsArgs struct {
	Refresh bool `json:"refresh"`
}

// ConfigArgs configures lovart_config.
type ConfigArgs struct {
	Model      string `json:"model"`
	IncludeAll bool   `json:"include_all"`
}

// QuoteArgs configures lovart_quote.
type QuoteArgs struct {
	Model string         `json:"model"`
	Body  map[string]any `json:"body"`
}

// ProjectSelectArgs configures lovart_project_select.
type ProjectSelectArgs struct {
	ProjectID string `json:"project_id"`
}

// ProjectCreateArgs configures lovart_project_create.
type ProjectCreateArgs struct {
	Name   string `json:"name"`
	Select bool   `json:"select"`
}

// ProjectShowArgs configures lovart_project_show.
type ProjectShowArgs struct {
	ProjectID string `json:"project_id"`
}

// ProjectOpenArgs configures lovart_project_open.
type ProjectOpenArgs struct {
	ProjectID string `json:"project_id"`
}

// ProjectRenameArgs configures lovart_project_rename.
type ProjectRenameArgs struct {
	ProjectID string `json:"project_id"`
	NewName   string `json:"new_name"`
}

// ProjectDeleteArgs configures lovart_project_delete.
type ProjectDeleteArgs struct {
	ProjectID        string `json:"project_id"`
	ConfirmProjectID string `json:"confirm_project_id"`
}

// ProjectRepairCanvasArgs configures lovart_project_repair_canvas.
type ProjectRepairCanvasArgs struct {
	ProjectID string `json:"project_id"`
}

// GenerateArgs configures single generation tools.
type GenerateArgs struct {
	Model                string         `json:"model"`
	Body                 map[string]any `json:"body"`
	Mode                 string         `json:"mode"`
	AllowPaid            bool           `json:"allow_paid"`
	MaxCredits           float64        `json:"max_credits"`
	ProjectID            string         `json:"project_id"`
	Wait                 bool           `json:"wait"`
	Download             bool           `json:"download"`
	Canvas               bool           `json:"canvas"`
	DownloadDir          string         `json:"download_dir"`
	DownloadDirTemplate  string         `json:"download_dir_template"`
	DownloadFileTemplate string         `json:"download_file_template"`
}

// JobsRunArgs configures lovart_jobs_run.
type JobsRunArgs struct {
	JobsFile        string  `json:"jobs_file"`
	AllowPaid       bool    `json:"allow_paid"`
	MaxTotalCredits float64 `json:"max_total_credits"`
	ProjectID       string  `json:"project_id"`
	DownloadDir     string  `json:"download_dir"`
}

// JobsStatusArgs configures lovart_jobs_status.
type JobsStatusArgs struct {
	RunDir  string `json:"run_dir"`
	Detail  string `json:"detail"`
	Refresh bool   `json:"refresh"`
}

// JobsResumeArgs configures lovart_jobs_resume.
type JobsResumeArgs struct {
	RunDir          string  `json:"run_dir"`
	AllowPaid       bool    `json:"allow_paid"`
	MaxTotalCredits float64 `json:"max_total_credits"`
	DownloadDir     string  `json:"download_dir"`
	RetryFailed     bool    `json:"retry_failed"`
}

// Executor runs validated MCP tool calls.
type Executor interface {
	AuthStatus(ctx context.Context) envelope.Envelope
	Setup(ctx context.Context, args SetupArgs) envelope.Envelope
	Models(ctx context.Context, args ModelsArgs) envelope.Envelope
	Config(ctx context.Context, args ConfigArgs) envelope.Envelope
	Balance(ctx context.Context) envelope.Envelope
	ProjectCurrent(ctx context.Context) envelope.Envelope
	ProjectList(ctx context.Context) envelope.Envelope
	ProjectCreate(ctx context.Context, args ProjectCreateArgs) envelope.Envelope
	ProjectSelect(ctx context.Context, args ProjectSelectArgs) envelope.Envelope
	ProjectShow(ctx context.Context, args ProjectShowArgs) envelope.Envelope
	ProjectOpen(ctx context.Context, args ProjectOpenArgs) envelope.Envelope
	ProjectRename(ctx context.Context, args ProjectRenameArgs) envelope.Envelope
	ProjectDelete(ctx context.Context, args ProjectDeleteArgs) envelope.Envelope
	ProjectRepairCanvas(ctx context.Context, args ProjectRepairCanvasArgs) envelope.Envelope
	Quote(ctx context.Context, args QuoteArgs) envelope.Envelope
	Generate(ctx context.Context, args GenerateArgs) envelope.Envelope
	JobsRun(ctx context.Context, args JobsRunArgs) envelope.Envelope
	JobsStatus(ctx context.Context, args JobsStatusArgs) envelope.Envelope
	JobsResume(ctx context.Context, args JobsResumeArgs) envelope.Envelope
}

func okLocal(data any, cacheUsed ...bool) envelope.Envelope {
	meta := envelope.ExecutionMetadata{
		ExecutionClass:  executionLocal,
		NetworkRequired: false,
		RemoteWrite:     false,
	}
	if len(cacheUsed) > 0 {
		meta.CacheUsed = boolPtr(cacheUsed[0])
	}
	return envelope.OKWithMetadata(data, meta)
}

func okPreflight(data any, cacheUsed ...bool) envelope.Envelope {
	meta := envelope.ExecutionMetadata{
		ExecutionClass:  executionPreflight,
		NetworkRequired: true,
		RemoteWrite:     false,
	}
	if len(cacheUsed) > 0 {
		meta.CacheUsed = boolPtr(cacheUsed[0])
	}
	return envelope.OKWithMetadata(data, meta)
}

func okSubmit(data any, submitted bool) envelope.Envelope {
	return envelope.OKWithMetadata(data, envelope.ExecutionMetadata{
		ExecutionClass:  executionSubmit,
		NetworkRequired: true,
		RemoteWrite:     submitted,
		Submitted:       boolPtr(submitted),
	})
}

func boolPtr(value bool) *bool {
	return &value
}
