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

// AuthLoginArgs configures lovart_auth_login.
type AuthLoginArgs struct {
	TimeoutSeconds float64 `json:"timeout_seconds"`
}

// ExtensionStatusArgs configures lovart_extension_status.
type ExtensionStatusArgs struct {
	ExtensionDir string `json:"extension_dir"`
}

// ExtensionInstallArgs configures lovart_extension_install.
type ExtensionInstallArgs struct {
	SourceDir    string `json:"source_dir"`
	ExtensionDir string `json:"extension_dir"`
	DryRun       bool   `json:"dry_run"`
	Open         bool   `json:"open"`
}

// ExtensionOpenArgs configures lovart_extension_open.
type ExtensionOpenArgs struct {
	ExtensionDir string `json:"extension_dir"`
}

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
	Mode  string         `json:"mode"`
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
	ProjectID          string `json:"project_id"`
	NewName            string `json:"new_name"`
	ConfirmedProjectID string `json:"confirmed_project_id"`
	ConfirmProjectID   string `json:"confirm_project_id"`
	ConfirmNewName     string `json:"confirm_new_name"`
}

// ProjectDeleteArgs configures lovart_project_delete.
type ProjectDeleteArgs struct {
	ProjectID          string `json:"project_id"`
	ConfirmProjectID   string `json:"confirm_project_id"`
	ConfirmedProjectID string `json:"confirmed_project_id"`
}

// TaskDownloadArgs configures lovart_task_download.
type TaskDownloadArgs struct {
	TaskID                    string `json:"task_id"`
	ArtifactIndex             int    `json:"artifact_index"`
	DownloadDir               string `json:"download_dir"`
	DownloadDirTemplate       string `json:"download_dir_template"`
	DownloadFileTemplate      string `json:"download_file_template"`
	Overwrite                 bool   `json:"overwrite"`
	ConfirmedCanonicalPath    string `json:"confirmed_canonical_path"`
	ConfirmOverwrite          bool   `json:"confirm_overwrite"`
	ConfirmOutsideDefaultRoot bool   `json:"confirm_outside_default_root"`
	Detail                    string `json:"detail"`
}

// TaskListArgs configures lovart_task_list.
type TaskListArgs struct {
	Active bool `json:"active"`
}

// TaskCancelArgs configures lovart_task_cancel.
type TaskCancelArgs struct {
	TaskIDs        []string `json:"task_ids"`
	ConfirmTaskIDs []string `json:"confirm_task_ids"`
}

// TaskStatusArgs configures lovart_task_status.
type TaskStatusArgs struct {
	TaskID string `json:"task_id"`
	Detail string `json:"detail"`
}

// TaskWaitArgs configures lovart_task_wait.
type TaskWaitArgs struct {
	TaskID         string  `json:"task_id"`
	Detail         string  `json:"detail"`
	TimeoutSeconds float64 `json:"timeout_seconds"`
	PollInterval   float64 `json:"poll_interval"`
}

// TaskCanvasArgs configures lovart_task_canvas.
type TaskCanvasArgs struct {
	TaskID                string `json:"task_id"`
	ProjectID             string `json:"project_id"`
	ConfirmedProjectID    string `json:"confirmed_project_id"`
	ConfirmCanvasWrite    bool   `json:"confirm_canvas_write"`
	ConfirmedCanvasLayout string `json:"confirmed_canvas_layout"`
	Detail                string `json:"detail"`
}

// CanvasArtifactsArgs configures lovart_canvas_artifacts.
type CanvasArtifactsArgs struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
	Limit     int    `json:"limit"`
	Offset    int    `json:"offset"`
	Detail    string `json:"detail"`
}

// CanvasArtifactArgs configures lovart_canvas_artifact.
type CanvasArtifactArgs struct {
	ProjectID  string `json:"project_id"`
	ArtifactID string `json:"artifact_id"`
	IncludeRaw bool   `json:"include_raw"`
}

// CanvasDownloadArgs configures lovart_canvas_download.
type CanvasDownloadArgs struct {
	ProjectID                 string `json:"project_id"`
	ConfirmedProjectID        string `json:"confirmed_project_id"`
	ArtifactID                string `json:"artifact_id"`
	ArtifactIndex             int    `json:"artifact_index"`
	TaskID                    string `json:"task_id"`
	All                       bool   `json:"all"`
	Original                  bool   `json:"original"`
	DownloadDir               string `json:"download_dir"`
	DownloadDirTemplate       string `json:"download_dir_template"`
	DownloadFileTemplate      string `json:"download_file_template"`
	Overwrite                 bool   `json:"overwrite"`
	ConfirmedCanonicalPath    string `json:"confirmed_canonical_path"`
	ConfirmOverwrite          bool   `json:"confirm_overwrite"`
	ConfirmOutsideDefaultRoot bool   `json:"confirm_outside_default_root"`
}

// GenerateArgs configures single generation tools.
type GenerateArgs struct {
	Model                     string         `json:"model"`
	Body                      map[string]any `json:"body"`
	Mode                      string         `json:"mode"`
	AllowPaid                 bool           `json:"allow_paid"`
	MaxCredits                float64        `json:"max_credits"`
	ConfirmPaid               bool           `json:"confirm_paid"`
	ConfirmedMaxCredits       float64        `json:"confirmed_max_credits"`
	ConfirmedModel            string         `json:"confirmed_model"`
	ConfirmedMode             string         `json:"confirmed_mode"`
	ConfirmedOutputs          int            `json:"confirmed_outputs"`
	ConfirmedRemoteRequests   int            `json:"confirmed_remote_requests"`
	ProjectID                 string         `json:"project_id"`
	ConfirmedProjectID        string         `json:"confirmed_project_id"`
	Wait                      bool           `json:"wait"`
	Download                  bool           `json:"download"`
	Canvas                    bool           `json:"canvas"`
	DownloadDir               string         `json:"download_dir"`
	DownloadDirTemplate       string         `json:"download_dir_template"`
	DownloadFileTemplate      string         `json:"download_file_template"`
	ConfirmedCanonicalPath    string         `json:"confirmed_canonical_path"`
	ConfirmOverwrite          bool           `json:"confirm_overwrite"`
	ConfirmOutsideDefaultRoot bool           `json:"confirm_outside_default_root"`
	ConfirmCanvasWrite        bool           `json:"confirm_canvas_write"`
	ConfirmedCanvasLayout     string         `json:"confirmed_canvas_layout"`
}

// JobsRunArgs configures lovart_jobs_run.
type JobsRunArgs struct {
	JobsFile                  string  `json:"jobs_file"`
	AllowPaid                 bool    `json:"allow_paid"`
	MaxTotalCredits           float64 `json:"max_total_credits"`
	ConfirmPaid               bool    `json:"confirm_paid"`
	ConfirmedMaxTotalCredits  float64 `json:"confirmed_max_total_credits"`
	ConfirmedRemoteRequests   int     `json:"confirmed_remote_requests"`
	ConfirmedMaxActiveTasks   int     `json:"confirmed_max_active_tasks"`
	ConfirmedModel            string  `json:"confirmed_model"`
	ConfirmedMode             string  `json:"confirmed_mode"`
	ConfirmedOutputs          int     `json:"confirmed_outputs"`
	ConfirmedMaxCredits       float64 `json:"confirmed_max_credits"`
	ProjectID                 string  `json:"project_id"`
	ConfirmedProjectID        string  `json:"confirmed_project_id"`
	DownloadDir               string  `json:"download_dir"`
	ConfirmedCanonicalPath    string  `json:"confirmed_canonical_path"`
	ConfirmOverwrite          bool    `json:"confirm_overwrite"`
	ConfirmOutsideDefaultRoot bool    `json:"confirm_outside_default_root"`
	SubmitIntervalSeconds     float64 `json:"submit_interval_seconds"`
	SubmitLimit               int     `json:"submit_limit"`
	MaxActiveTasks            int     `json:"max_active_tasks"`
}

// JobsStatusArgs configures lovart_jobs_status.
type JobsStatusArgs struct {
	RunDir  string `json:"run_dir"`
	Detail  string `json:"detail"`
	Refresh bool   `json:"refresh"`
}

// JobsResumeArgs configures lovart_jobs_resume.
type JobsResumeArgs struct {
	RunDir                    string  `json:"run_dir"`
	AllowPaid                 bool    `json:"allow_paid"`
	MaxTotalCredits           float64 `json:"max_total_credits"`
	ConfirmPaid               bool    `json:"confirm_paid"`
	ConfirmedMaxTotalCredits  float64 `json:"confirmed_max_total_credits"`
	ConfirmedRemoteRequests   int     `json:"confirmed_remote_requests"`
	ConfirmedMaxActiveTasks   int     `json:"confirmed_max_active_tasks"`
	ConfirmedModel            string  `json:"confirmed_model"`
	ConfirmedMode             string  `json:"confirmed_mode"`
	ConfirmedOutputs          int     `json:"confirmed_outputs"`
	ConfirmedMaxCredits       float64 `json:"confirmed_max_credits"`
	ConfirmedProjectID        string  `json:"confirmed_project_id"`
	DownloadDir               string  `json:"download_dir"`
	ConfirmedCanonicalPath    string  `json:"confirmed_canonical_path"`
	ConfirmOverwrite          bool    `json:"confirm_overwrite"`
	ConfirmOutsideDefaultRoot bool    `json:"confirm_outside_default_root"`
	RetryFailed               bool    `json:"retry_failed"`
	SubmitIntervalSeconds     float64 `json:"submit_interval_seconds"`
	SubmitLimit               int     `json:"submit_limit"`
	MaxActiveTasks            int     `json:"max_active_tasks"`
}

// JobsFinalizeArgs configures lovart_jobs_finalize.
type JobsFinalizeArgs struct {
	RunDir                    string `json:"run_dir"`
	Download                  bool   `json:"download"`
	Canvas                    bool   `json:"canvas"`
	ProjectID                 string `json:"project_id"`
	ConfirmedProjectID        string `json:"confirmed_project_id"`
	DownloadDir               string `json:"download_dir"`
	ConfirmedCanonicalPath    string `json:"confirmed_canonical_path"`
	ConfirmOverwrite          bool   `json:"confirm_overwrite"`
	ConfirmOutsideDefaultRoot bool   `json:"confirm_outside_default_root"`
	Detail                    string `json:"detail"`
	CanvasLayout              string `json:"canvas_layout"`
	ConfirmCanvasWrite        bool   `json:"confirm_canvas_write"`
	ConfirmedCanvasLayout     string `json:"confirmed_canvas_layout"`
}

// Executor runs validated MCP tool calls.
type Executor interface {
	AuthStatus(ctx context.Context) envelope.Envelope
	AuthLogin(ctx context.Context, args AuthLoginArgs) envelope.Envelope
	ExtensionStatus(ctx context.Context, args ExtensionStatusArgs) envelope.Envelope
	ExtensionInstall(ctx context.Context, args ExtensionInstallArgs) envelope.Envelope
	ExtensionOpen(ctx context.Context, args ExtensionOpenArgs) envelope.Envelope
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
	TaskStatus(ctx context.Context, args TaskStatusArgs) envelope.Envelope
	TaskList(ctx context.Context, args TaskListArgs) envelope.Envelope
	TaskCancel(ctx context.Context, args TaskCancelArgs) envelope.Envelope
	TaskWait(ctx context.Context, args TaskWaitArgs) envelope.Envelope
	TaskCanvas(ctx context.Context, args TaskCanvasArgs) envelope.Envelope
	TaskDownload(ctx context.Context, args TaskDownloadArgs) envelope.Envelope
	CanvasArtifacts(ctx context.Context, args CanvasArtifactsArgs) envelope.Envelope
	CanvasArtifact(ctx context.Context, args CanvasArtifactArgs) envelope.Envelope
	CanvasDownload(ctx context.Context, args CanvasDownloadArgs) envelope.Envelope
	Quote(ctx context.Context, args QuoteArgs) envelope.Envelope
	Generate(ctx context.Context, args GenerateArgs) envelope.Envelope
	JobsRun(ctx context.Context, args JobsRunArgs) envelope.Envelope
	JobsStatus(ctx context.Context, args JobsStatusArgs) envelope.Envelope
	JobsResume(ctx context.Context, args JobsResumeArgs) envelope.Envelope
	JobsFinalize(ctx context.Context, args JobsFinalizeArgs) envelope.Envelope
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
