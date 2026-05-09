# Lovart v2 — Architecture

## Overview

v2 is a single Lovart user runtime. The Go binary owns local generation execution, MCP integration, browser-session auth handoff, and the runtime cache for Lovart metadata plus signer WASM.

```
┌──────────────────────────────────────────────────────────────┐
│ go.mod (root)                                                │
│                                                              │
│  ┌──────────────────────────┐     ┌───────────────────────┐  │
│  │ Go                       │     │ TypeScript            │  │
│  │ CLI + MCP +              │     │ Chrome MV3 Extension  │  │
│  │ Protocol Core            │     │ auth handoff          │  │
│  │                          │     │                       │  │
│  │ lovart binary            │     │ Lovart Connector      │  │
│  └────────────┬─────────────┘     └───────────┬───────────┘  │
│               │                               │              │
│               │    ┌──────────────────────────▼────┐         │
│               └───→│ ~/.lovart/                    │         │
│                    │ auth + metadata + runs + tmp  │         │
│                    └──────[runtime self-update]────┘         │
└──────────────────────────────────────────────────────────────┘
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

### Chrome Extension (`extension/`)
- **Scope**: Lovart Connector MV3 extension, browser-session auth handoff, optional page UI
- **Self-contained**: static extension assets, no Go core logic
- **Runtime**: content-script + service worker + popup
- **Background**: service worker observes Lovart request headers and posts approved auth to local CLI

## Runtime Metadata

The runtime root defaults to `~/.lovart` and can be overridden with `LOVART_HOME` for isolated test or automation runs. User-owned data is kept there until the user explicitly runs `lovart clean`. Runtime intermediate files are placed under `~/.lovart/tmp` or written as hidden atomic `.*.tmp` files under the root, and stale intermediate files are removed automatically on CLI startup.

### Signer WASM (`~/.lovart/signing/`)
- `current.wasm` is the only production signer source for Go.
- `manifest.json` records source URL, SHA256, frontend hashes, and sync time.
- `lovart update sync --signer` bootstraps from public Lovart frontend assets without an existing signer.

### Generator Metadata (`~/.lovart/metadata/`)
- `generator_list.json` and `generator_schema.json` are runtime cache files.
- `manifest.json` records stable hashes and the signer SHA used for sync.
- `lovart update sync --all` refreshes signer first, then signed generator metadata.

## Directory Layout

```
lovart/
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
│   ├── setup/             # Runtime readiness and repair checks
│   └── update/            # Drift detection + metadata sync
├── cli/                   # Cobra command definitions
├── mcp/                   # MCP stdio server
├── internal/signing/testdata/ # Non-production WASM fixture for signer tests
├── extension/             # Chrome extension
├── packaging/             # release installers + extension build
├── docs/architecture/     # Architecture principles and module rules
├── docs/                  # Documentation
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
func NewSigner() (Signer, error)  // wazero + ~/.lovart/signing/current.wasm
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
}
type Session struct {
    Cookie    string `json:"cookie,omitempty"`
    Token     string `json:"token,omitempty"`
    CSRF      string `json:"csrf,omitempty"`
    ProjectID string `json:"project_id,omitempty"`
    Source    string `json:"source,omitempty"`
}
func LoadCreds() (*Credentials, error)
func LoadProjectContext() (*ProjectContext, error)
func SaveSession(session Session) error
func GetStatus() Status
func SetProject(projectID string) error
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

Local registry, manifest, pricing quote data, and job state are caches used for speed,
resumability, and validation. They do not make Lovart generation usable without
network access.

## CLI Command Tree

```
lovart --version
lovart -v
lovart upgrade [--check] [--dry-run] [--yes] [--force] [--version latest|vX.Y.Z] [--repo OWNER/REPO] [--install-path <path>] [--extension-dir <path>] [--no-extension]
lovart uninstall [--dry-run] [--yes] [--data] [--install-path <path>] [--extension-dir <path>] [--clients auto|all|none|codex,claude,opencode,openclaw] [--keep-mcp] [--keep-extension] [--force]
lovart auth status
lovart auth login
lovart auth logout --yes
lovart clean [--dry-run] [--runs|--downloads|--cache|--auth|--extension|--all] [--yes]
lovart setup
lovart self-test
lovart doctor
lovart balance
lovart models [--refresh]
lovart config <model> [--all]
lovart project list
lovart project create <name>
lovart project select <id>
lovart project show [id]
lovart project open [id]
lovart project admin rename <id> <name>
lovart project admin delete <id>
lovart project admin repair-canvas [id]
lovart quote <model> --body-file <file> [--mode auto|fast|relax]
lovart generate <model> (--body-file <file>|--prompt <text>) [--project-id <id>] [--mode auto|fast|relax] [--allow-paid] [--no-wait] [--no-download] [--no-canvas]
lovart jobs run <jobs.jsonl> [--project-id <id>] [--allow-paid --max-total-credits N] [--download-dir <dir>]
lovart jobs resume <run_dir> [--allow-paid --max-total-credits N] [--download-dir <dir>] [--retry-failed]
lovart jobs status <run_dir> [--refresh] [--detail summary|requests|full]
lovart update check
lovart update diff
lovart update sync --all
lovart update sync --signer
lovart update sync --metadata-only
lovart mcp
lovart mcp status [--clients auto|all|none|codex,claude,opencode,openclaw]
lovart mcp install --clients auto --yes [--dry-run] [--force]
lovart dev sign
lovart dev auth-login [--timeout-seconds N] [--debug-port N]
```

`lovart update sync` refreshes runtime signer and generator metadata. `lovart upgrade` updates the installed CLI binary and, by default, the Lovart Connector extension files.

## MCP Tools (19)

```
lovart_auth_status,
lovart_setup, lovart_models, lovart_config,
lovart_balance,
lovart_project_current, lovart_project_list, lovart_project_create,
lovart_project_select, lovart_project_show, lovart_project_open,
lovart_project_rename, lovart_project_delete, lovart_project_repair_canvas,
lovart_quote,
lovart_generate,
lovart_jobs_run, lovart_jobs_status,
lovart_jobs_resume
```

## Chrome Extension Flow

```
User runs `lovart auth login`
  → CLI starts a one-time callback on 127.0.0.1:47821-47830
  → CLI opens Lovart with `lovart_cli_auth=1`
  → Content script detects the local callback and shows a Connect prompt
  → User clicks Connect on the Lovart page
  → Service worker reads Lovart cookies and recent auth headers
  → Service worker posts the approved session to the local callback
  → CLI stores `~/.lovart/creds.json` and prints safe next steps
```

Developer auth uses `lovart dev auth-login`. It restarts the daily Google Chrome app with a local DevTools debugging port, opens Lovart, captures the signed-in browser session through Chrome DevTools Protocol, validates the captured session against Lovart project APIs, and only then writes `~/.lovart/creds.json`. It is intentionally under `dev` so normal users only see the extension-based login path.

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
| P5 | Extension (auth connector content script + SW + popup) | Implemented |
