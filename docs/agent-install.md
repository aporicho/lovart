# Agent Install

This project is distributed to agents as a self-contained `lovart` binary. The same binary provides both the JSON CLI and the safe MCP stdio server.

## Binary Install

macOS arm64:

```bash
mkdir -p ~/.local/bin
gh release download --repo aporicho/lovart-reverse --pattern "lovart-macos-arm64" -O ~/.local/bin/lovart
chmod +x ~/.local/bin/lovart
lovart --version
lovart self-test
```

Linux x64:

```bash
mkdir -p ~/.local/bin
gh release download --repo aporicho/lovart-reverse --pattern "lovart-linux-x64" -O ~/.local/bin/lovart
chmod +x ~/.local/bin/lovart
lovart --version
lovart self-test
```

Windows x64:

```powershell
gh release download --repo aporicho/lovart-reverse --pattern "lovart-windows-x64.exe" -O "$env:USERPROFILE\bin\lovart.exe"
lovart --version
lovart self-test
```

If `lovart --version` shows an unexpected version, commit, or command set, replace the binary before calling generation commands.

## MCP Config

Use stdio through the single binary:

```toml
[mcp_servers.lovart]
command = "/absolute/path/to/lovart"
args = ["mcp"]
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

When MCP is unavailable, use the same binary directly:

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

## Developer And Reverse Maintainer Install

Python installs are for local development and reverse maintenance only:

```bash
uv tool install git+ssh://git@github.com/aporicho/lovart-reverse.git
uv tool install "git+ssh://git@github.com/aporicho/lovart-reverse.git#egg=lovart-reverse[reverse]"
```

The `reverse` extra installs capture dependencies such as `mitmproxy`; normal agent users do not need them.
