# Agent Install

This project exposes Lovart through two agent-friendly surfaces:

- `lovart`: global JSON CLI and only execution layer.
- `lovart-mcp`: safe MCP stdio wrapper around the CLI command facade.

## Private GitHub Install

Recommended:

```bash
uv tool install git+ssh://git@github.com/aporicho/lovart-reverse.git
lovart --version
lovart self-test
```

Alternative:

```bash
pipx install git+ssh://git@github.com/aporicho/lovart-reverse.git
lovart --version
lovart self-test
```

If `lovart --version` shows an unexpected version, commit, or command set, reinstall before calling generation commands.

## MCP Config

Use stdio:

```json
{
  "mcpServers": {
    "lovart": {
      "command": "lovart-mcp",
      "args": []
    }
  }
}
```

MCP tools return the same JSON envelope as the CLI:

```json
{"ok":true,"data":{},"warnings":[]}
```

```json
{"ok":false,"error":{"code":"credit_risk","message":"...","details":{}}}
```

## Safe MCP Tools

- `lovart_setup`
- `lovart_models`
- `lovart_config`
- `lovart_plan`
- `lovart_quote`
- `lovart_generate_dry_run`
- `lovart_generate`
- `lovart_jobs_quote`
- `lovart_jobs_dry_run`
- `lovart_jobs_run`
- `lovart_jobs_status`
- `lovart_jobs_resume`

The MCP server does not expose capture, credential extraction, reverse replay submission, metadata sync, or direct `ref/` mutation.

## CLI Fallback

When MCP is unavailable, use the CLI directly:

```bash
lovart setup
lovart config <model>
lovart plan --intent image-concept
lovart quote <model> --body-file request.json
lovart generate <model> --body-file request.json --mode auto --dry-run
lovart generate <model> --body-file request.json --mode auto --wait --download
```

For batch generation:

```bash
lovart jobs quote runs/<project>/jobs.jsonl
lovart jobs dry-run runs/<project>/jobs.jsonl
lovart jobs run runs/<project>/jobs.jsonl --wait --download
lovart jobs resume runs/<project>/jobs.jsonl --wait --download
```

Paid generation still requires explicit budget flags in both MCP and CLI calls.
