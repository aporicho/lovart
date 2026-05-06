# Lovart v2 — Architecture

## Overview

v2 is a full redesign of the Lovart reverse-engineering toolkit into independent runtimes. The Go runtime owns local generation execution and keeps Lovart metadata plus signer WASM in an explicit runtime cache.

```
┌──────────────────────────────────────────────────────────────┐
│ go.mod (root)                                                │
│                                                              │
│  ┌─────────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ Go              │  │ Python       │  │ TypeScript    │  │
│  │ CLI + MCP +     │  │ Reverse      │  │ Extension     │  │
│  │ Protocol Core   │  │ (independent)│  │ (standalone)  │  │
│  │                 │  │              │  │               │  │
│  │ lovart binary   │  │ lovart-reverse│  │ Chrome MV3    │  │
│  └────────┬────────┘  └──────────────┘  └───────┬───────┘  │
│           │                                      │           │
│           │    ┌───────────────────┐                         │
│           └───→│ .lovart/          │                         │
│                │ signing + metadata│                         │
│                └──────[runtime self-update]────────────────┘ │
└──────────────────────────────────────────────────────────────┘

v1/  — legacy Python project (preserved as-is)
```

Architecture changes must follow `docs/architecture/file-architecture-philosophy.md`.
For Go code, package directories are module boundaries and exported package
symbols are the standard API; callers should not depend on private file-level
implementation details.

## Module Boundaries

### Go (`cmd/lovart`, `internal/`, `cli/`, `mcp/`)
- **CLI**: single `lovart` binary with subcommands (cobra)
- **MCP**: stdio JSON-RPC server, maps tools to core functions
- **Core** (`internal/`): protocol library — signing, HTTP, pricing, generation, jobs, projects
- **Build**: `go build` → single static binary (no Python, no Node.js, no CGO)

### Python (`reverse/`)
- **Scope**: reverse engineering only — capture sessions, credential extraction, metadata drift detection
- **Independent**: `pip install lovart-reverse`, separate CLI, zero Go dependency
- **Legacy**: v1 codebase preserved at `v1/` for reference

### TypeScript (`extension/`)
- **Scope**: Chrome Extension MV3, injects batch panel into Lovart web page
- **Self-contained**: zero dependency on local Go/Python binaries
- **Runtime**: content-script + service worker + popup
- **Background**: service worker continues generation after tab close

## Runtime Metadata

### Signer WASM (`.lovart/signing/`)
- `current.wasm` is the only production signer source for Go.
- `manifest.json` records source URL, SHA256, frontend hashes, and sync time.
- `lovart update sync --signer` bootstraps from public Lovart frontend assets without an existing signer.

### Generator Metadata (`.lovart/metadata/`)
- `generator_list.json` and `generator_schema.json` are runtime cache files.
- `manifest.json` records stable hashes and the signer SHA used for sync.
- `lovart update sync --all` refreshes signer first, then signed generator metadata.

## Directory Layout

```
lovart-reverse/
├── cmd/lovart/main.go
├── internal/
│   ├── envelope/          # JSON envelope types
│   ├── errors/            # Error codes + types
│   ├── paths/             # Runtime path resolution
│   ├── auth/store.go      # Credential persistence
│   ├── signing/           # Signer interface + wazero impl
│   ├── http/              # Signed Lovart HTTP client
│   ├── discovery/         # Model list + schema
│   ├── registry/          # Model validation
│   ├── config/            # Schema → config fields
│   ├── pricing/           # Credit quotes
│   ├── entitlement/       # Free tier checks
│   ├── project/           # Project mgmt (create, list, select)
│   ├── generation/        # Single generation (preflight + submit + poll)
│   ├── task/              # Task status polling
│   ├── downloads/         # Artifact download
│   ├── jobs/              # Batch generation (quote, run, resume, status)
│   ├── setup/             # Readiness check
│   └── update/            # Drift detection + metadata sync
├── cli/                   # Cobra command definitions
├── mcp/                   # MCP stdio server
├── internal/signing/testdata/ # Non-production WASM fixture for signer tests
├── reverse/               # Python reverse tooling
├── extension/             # Chrome extension
├── packaging/             # release installers + extension build
├── docs/architecture/     # Architecture principles and module rules
├── docs/                  # Documentation
├── v1/                    # Legacy Python project (preserved)
├── go.mod
├── go.sum
├── Makefile
└── .gitignore
```

## Go Core Interfaces

### Signer
```go
type Signer interface {
    Sign(ctx context.Context, payload SigningPayload) (*SigningResult, error)
    Health() error
}
func NewSigner() (Signer, error)  // wazero + .lovart/signing/current.wasm
```

### Auth
```go
type Credentials struct {
    Cookie string `json:"cookie"`
    Token  string `json:"token"`
    CSRF   string `json:"csrf"`
}
type ProjectContext struct {
    ProjectID string `json:"project_id"`
    CID       string `json:"cid"`
}
func LoadCreds() (*Credentials, error)
func LoadProjectContext() (*ProjectContext, error)
func SetProject(projectID, cid string) error
```

### HTTP Client
```go
type Client struct { ... }
func NewClient(creds Credentials, signer signing.Signer) *Client
func (c *Client) Do(ctx context.Context, method, path string, body any) (*http.Response, error)
```

### Generation
```go
type PreflightResult struct {
    CanSubmit         bool
    BlockingError     *LovartError
    CreditRisk        bool
    PaidRequired      bool
    QuotedCredits     float64
}
func Preflight(model, body, mode string, allowPaid bool, maxCredits float64) (*PreflightResult, error)
func DryRun(model string, body map[string]any) (*DryRunResult, error)
func Submit(ctx context.Context, model, body, mode string) (*SubmitResult, error)
func Wait(ctx context.Context, taskID string) (*TaskInfo, error)
```

### Project
```go
type Project struct {
    ID           string `json:"project_id"`
    Name         string `json:"name"`
    CanvasURL    string `json:"canvas_url"`
}
func ListProjects() ([]Project, error)
func CreateProject(name string) (*Project, error)
```

### Jobs
```go
type JobLine struct {
    JobID   string         `json:"job_id"`
    Model   string         `json:"model"`
    Mode    string         `json:"mode"`
    Outputs int            `json:"outputs"`
    Body    map[string]any `json:"body"`
}
func QuoteJobs(jobsFile string, opts QuoteOptions) (*QuoteSummary, error)
func DryRunJobs(jobsFile string, opts JobsOptions) (*DryRunSummary, error)
func RunJobs(jobsFile string, opts JobsOptions) (*RunSummary, error)
func ResumeJobs(jobsFile string, opts JobsOptions) (*RunSummary, error)
func StatusJobs(runDir string, detail string) (*StatusSummary, error)
```

### Envelope
```go
type Envelope struct {
    OK              bool       `json:"ok"`
    Data            any        `json:"data,omitempty"`
    ExecutionClass  string     `json:"execution_class,omitempty"`  // local | preflight | submit
    NetworkRequired *bool      `json:"network_required,omitempty"`
    RemoteWrite     *bool      `json:"remote_write,omitempty"`
    Submitted       *bool      `json:"submitted,omitempty"`
    CacheUsed       *bool      `json:"cache_used,omitempty"`
    Warnings        []string   `json:"warnings,omitempty"`
    Error           *ErrorBody `json:"error,omitempty"`
}
```

Execution classes are user-facing semantics, not a separate runtime mode:

- `local`: reads local files, credentials, registry data, or saved job state. Network is not required.
- `preflight`: contacts Lovart or validates against current remote state, but does not create generation tasks or mutate remote projects.
- `submit`: performs a remote write, such as creating a generation task or mutating a project.

Local registry, manifest, quote state, and job state are caches used for speed,
resumability, and validation. They do not make Lovart generation usable without
network access.

## CLI Command Tree

```
lovart --version
lovart version
lovart setup
lovart self-test
lovart doctor
lovart models [--refresh]
lovart config <model> [--all] [--example defaults|zero_credit] [--global]
lovart project list
lovart project create <name>
lovart project select <id>
lovart project show [id]
lovart project open [id]
lovart project repair-canvas [id] [--cid <cid>]
lovart quote <model> --body-file <file>
lovart generate <model> --body-file <file> [--project-id <id>] [--cid <cid>] [--mode] [--dry-run] [--allow-paid] [--no-wait] [--no-download] [--no-canvas]
lovart task <task_id>
lovart jobs quote <jobs.jsonl> [--detail]
lovart jobs quote-status <run_dir>
lovart jobs dry-run <jobs.jsonl>
lovart jobs run <jobs.jsonl> [--no-wait] [--no-download] [--no-canvas] [--canvas-layout frame|plain]
lovart jobs resume <run_dir> [--no-wait] [--no-download] [--no-canvas] [--canvas-layout frame|plain] [--retry-failed]
lovart jobs status <run_dir> [--detail]
lovart update check
lovart update diff
lovart update sync --all
lovart update sync --signer
lovart update sync --metadata-only
lovart mcp
lovart mcp status [--clients auto|all|none|codex,claude,opencode,openclaw]
lovart mcp install --clients auto --yes [--dry-run] [--force]
```

## MCP Tools (11)

```
lovart_setup, lovart_models, lovart_config,
lovart_quote,
lovart_generate_dry_run, lovart_generate,
lovart_jobs_quote,
lovart_jobs_dry_run, lovart_jobs_run, lovart_jobs_status,
lovart_jobs_resume
```

## Chrome Extension Flow

```
User opens Lovart page
  → Content script injects right-side panel (React)
  → Reads DOM: project_id, model, mode
  → User enters N prompts → clicks "Batch Generate"
  → Content script → Service Worker (chrome.runtime.sendMessage)
  → SW reads cookies, loads wasm signer, submits tasks
  → SW polls status, stores in chrome.storage.local
  → Progress pushed to content script in real-time
  → Tab closed → SW continues, chrome.notifications on completion
  → Tab reopens → content script restores state from chrome.storage
```

## API Endpoints

| Method | Path | Base | Purpose |
|--------|------|------|---------|
| GET | /v1/generator/list | lgw.lovart.ai | Model list |
| GET | /v1/generator/schema | lgw.lovart.ai | Model schemas |
| POST | /v1/generator/tasks | lgw.lovart.ai | Submit generation |
| GET | /v1/generator/tasks?task_id= | lgw.lovart.ai | Task status |
| POST | /api/canva/agent-cashier/task/take/slot | www.lovart.ai | Take gen slot (needs project_id) |
| POST | /api/canva/agent-cashier/task/set/unlimited | www.lovart.ai | Set fast/relax mode |

## Implementation Priority

| Priority | Module | Status |
|----------|--------|--------|
| P0 | Go core (signing, http, auth, envelope, errors, paths) | Partial |
| P1 | Go CLI (single generation, config, quote) | Partial |
| P2 | Project module (create, list, select, canvas writeback) | Partial |
| P3 | Jobs batch generation + resume | Partial |
| P4 | MCP server | MVP implemented |
| P5 | Extension (content script + SW + signing) | Skeleton |
| P6 | Reverse tooling (Python, mitmproxy) | Partial |
