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

// GenerateArgs configures single generation tools.
type GenerateArgs struct {
	Model                string         `json:"model"`
	Body                 map[string]any `json:"body"`
	Mode                 string         `json:"mode"`
	AllowPaid            bool           `json:"allow_paid"`
	MaxCredits           float64        `json:"max_credits"`
	ProjectID            string         `json:"project_id"`
	CID                  string         `json:"cid"`
	Wait                 bool           `json:"wait"`
	Download             bool           `json:"download"`
	Canvas               bool           `json:"canvas"`
	DownloadDir          string         `json:"download_dir"`
	DownloadDirTemplate  string         `json:"download_dir_template"`
	DownloadFileTemplate string         `json:"download_file_template"`
}

// JobsQuoteArgs configures lovart_jobs_quote.
type JobsQuoteArgs struct {
	JobsFile string `json:"jobs_file"`
}

// JobsDryRunArgs configures lovart_jobs_dry_run.
type JobsDryRunArgs struct {
	JobsFile        string  `json:"jobs_file"`
	OutDir          string  `json:"out_dir"`
	AllowPaid       bool    `json:"allow_paid"`
	MaxTotalCredits float64 `json:"max_total_credits"`
	Detail          string  `json:"detail"`
}

// JobsRunArgs configures lovart_jobs_run.
type JobsRunArgs struct {
	JobsFile             string  `json:"jobs_file"`
	OutDir               string  `json:"out_dir"`
	AllowPaid            bool    `json:"allow_paid"`
	MaxTotalCredits      float64 `json:"max_total_credits"`
	Wait                 bool    `json:"wait"`
	Download             bool    `json:"download"`
	Canvas               bool    `json:"canvas"`
	CanvasLayout         string  `json:"canvas_layout"`
	DownloadDir          string  `json:"download_dir"`
	DownloadDirTemplate  string  `json:"download_dir_template"`
	DownloadFileTemplate string  `json:"download_file_template"`
	TimeoutSeconds       float64 `json:"timeout_seconds"`
	PollInterval         float64 `json:"poll_interval"`
	ProjectID            string  `json:"project_id"`
	CID                  string  `json:"cid"`
	Detail               string  `json:"detail"`
}

// JobsStatusArgs configures lovart_jobs_status.
type JobsStatusArgs struct {
	RunDir  string `json:"run_dir"`
	Detail  string `json:"detail"`
	Refresh bool   `json:"refresh"`
}

// JobsResumeArgs configures lovart_jobs_resume.
type JobsResumeArgs struct {
	RunDir               string  `json:"run_dir"`
	AllowPaid            bool    `json:"allow_paid"`
	MaxTotalCredits      float64 `json:"max_total_credits"`
	Wait                 bool    `json:"wait"`
	Download             bool    `json:"download"`
	Canvas               bool    `json:"canvas"`
	CanvasLayout         string  `json:"canvas_layout"`
	DownloadDir          string  `json:"download_dir"`
	DownloadDirTemplate  string  `json:"download_dir_template"`
	DownloadFileTemplate string  `json:"download_file_template"`
	RetryFailed          bool    `json:"retry_failed"`
	TimeoutSeconds       float64 `json:"timeout_seconds"`
	PollInterval         float64 `json:"poll_interval"`
	ProjectID            string  `json:"project_id"`
	CID                  string  `json:"cid"`
	Detail               string  `json:"detail"`
}

// Executor runs validated MCP tool calls.
type Executor interface {
	Setup(ctx context.Context, args SetupArgs) envelope.Envelope
	Models(ctx context.Context, args ModelsArgs) envelope.Envelope
	Config(ctx context.Context, args ConfigArgs) envelope.Envelope
	Quote(ctx context.Context, args QuoteArgs) envelope.Envelope
	GenerateDryRun(ctx context.Context, args GenerateArgs) envelope.Envelope
	Generate(ctx context.Context, args GenerateArgs) envelope.Envelope
	JobsQuote(ctx context.Context, args JobsQuoteArgs) envelope.Envelope
	JobsDryRun(ctx context.Context, args JobsDryRunArgs) envelope.Envelope
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

func okPreflightSubmission(data any, submitted bool) envelope.Envelope {
	return envelope.OKWithMetadata(data, envelope.ExecutionMetadata{
		ExecutionClass:  executionPreflight,
		NetworkRequired: true,
		RemoteWrite:     false,
		Submitted:       boolPtr(submitted),
	})
}

func boolPtr(value bool) *bool {
	return &value
}
