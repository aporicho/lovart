# Agent Install

This project is distributed to agents as a self-contained `lovart` binary. The same binary provides both the JSON CLI and the safe MCP stdio server.

## Installer

The recommended path is the release installer. It uses `gh release download`, verifies checksums, installs the matching binary, and configures detected agents for `lovart mcp`.

Authenticate first:

```bash
gh auth login
```

macOS / Linux:

```bash
gh release download --repo aporicho/lovart-reverse --pattern install.sh -O /tmp/lovart-install.sh
bash /tmp/lovart-install.sh --agents auto --yes
lovart --version
lovart self-test
lovart agent status
```

Windows:

```powershell
gh release download --repo aporicho/lovart-reverse --pattern install.ps1 -O "$env:TEMP\lovart-install.ps1"
powershell -ExecutionPolicy Bypass -File "$env:TEMP\lovart-install.ps1" -Agents auto -Yes
lovart --version
lovart self-test
lovart agent status
```

If `lovart --version` shows an unexpected version, commit, or command set, replace the binary before calling generation commands.

## MCP Config

The installer writes MCP config automatically. Manual Codex config uses stdio through the single binary:

```toml
[mcp_servers.lovart]
command = "/absolute/path/to/lovart"
args = ["mcp"]
```

Direct binary download remains available as a fallback:

```bash
mkdir -p ~/.local/bin
gh release download --repo aporicho/lovart-reverse --pattern "lovart-macos-arm64" -O ~/.local/bin/lovart
chmod +x ~/.local/bin/lovart
lovart --version
lovart self-test
lovart agent install --agents auto --yes
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
