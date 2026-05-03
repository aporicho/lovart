# Lovart Reverse

Lovart Reverse is an agent-first Lovart generation wrapper. The stable execution layer is a JSON CLI, and the MCP server is a safe thin wrapper around the same commands.

This README is the main manual. Other docs are references or role methods; they must not redefine the core workflow.

## Golden Rules

- Parse stdout only. Every command returns a JSON envelope; stderr is human diagnostics.
- Prefer the `lovart mcp` stdio server when the agent supports MCP; otherwise call the `lovart` CLI directly.
- Do not read `.lovart/`, `scripts/creds.json`, `captures/`, browser profiles, or `ref/` directly.
- Legal model parameters come from `lovart config <model>`. Do not guess sizes, quality values, aspect ratios, modes, or counts.
- `quote` tells the credit cost. `dry-run` and the generation gate decide whether submission is allowed.
- Default real generation is zero-credit only.
- Paid single generation requires `--allow-paid --max-credits N`.
- Paid batch generation requires `--allow-paid --max-total-credits N`.
- Batch `jobs.jsonl` is user-level: one line is one concept/task. Use top-level `outputs` for requested image count.
- Do not manually split one concept into many JSONL rows. The CLI expands `outputs` into remote requests.
- After interruption, use `lovart jobs resume`, not `jobs run`. Existing `task_id` values must not be submitted again.
- `jobs resume` refuses changed `jobs.jsonl` files because the state stores `jobs_file_hash`.

## Install

Normal users should use the release installer. It downloads the self-contained `lovart` binary and configures supported MCP clients for `lovart mcp`. It requires GitHub CLI authentication:

```bash
gh auth login
```

macOS / Linux:

```bash
gh release download --repo aporicho/lovart-reverse --pattern install.sh -O /tmp/lovart-install.sh
bash /tmp/lovart-install.sh --mcp-clients auto --yes
```

Windows:

```powershell
gh release download --repo aporicho/lovart-reverse --pattern install.ps1 -O "$env:TEMP\lovart-install.ps1"
powershell -ExecutionPolicy Bypass -File "$env:TEMP\lovart-install.ps1" -McpClients auto -Yes
```

Verify:

```bash
lovart --version
lovart self-test
lovart mcp status
```

Direct binary download is the fallback path.

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

If `lovart --version` shows an older command set or a different git commit than expected, replace the binary before using it from an agent.

The installer writes MCP config for detected MCP clients. Manual Codex config is:

```toml
[mcp_servers.lovart]
command = "/absolute/path/to/lovart"
args = ["mcp"]
```

Python installs are for developers and reverse maintainers:

```bash
uv tool install git+ssh://git@github.com/aporicho/lovart-reverse.git
uv tool install "git+ssh://git@github.com/aporicho/lovart-reverse.git#egg=lovart-reverse[reverse]"
```

If auth is missing, a reverse maintainer can capture and extract credentials from a Python environment with the `reverse` extra:

```bash
lovart reverse capture
lovart auth extract captures/<lovart-request>.json
```

## JSON Envelope

Success:

```json
{"ok":true,"data":{},"warnings":[]}
```

Failure:

```json
{"ok":false,"error":{"code":"auth_missing","message":"...","details":{}}}
```

## Single Generation

Use this flow for one request:

```bash
lovart setup
lovart plan --intent image-concept
lovart config openai/gpt-image-2
lovart quote openai/gpt-image-2 --body-file request.json
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
```

Example `request.json`:

```json
{
  "prompt": "a clean product render of a red cube on a white background",
  "quality": "low",
  "size": "1024*1024"
}
```

Paid generation must be explicit:

```bash
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --allow-paid --max-credits 5 --wait --download
```

## Batch Generation

Agents convert prompt documents into `jobs.jsonl`; humans should not need to hand-write it. Each line is one user-level concept job:

```json
{"job_id":"001","title":"青竹峰晨雾中的韩立","model":"seedream/seedream-5-0","mode":"relax","outputs":10,"body":{"prompt":"...","aspect_ratio":"4:3","resolution":"2K","response_format":"url","watermark":false}}
```

`outputs` means desired image count for that concept. When present, `body` must not contain `n`, `max_images`, or `count`; the CLI maps `outputs` to the model quantity field and splits into multiple remote requests only when needed.

Batch flow:

```bash
lovart setup
lovart plan --intent image-concept
lovart config seedream/seedream-5-0
lovart jobs quote runs/fanren/jobs.jsonl
lovart jobs dry-run runs/fanren/jobs.jsonl
lovart jobs run runs/fanren/jobs.jsonl --wait --download
lovart jobs status runs/fanren
lovart jobs resume runs/fanren/jobs.jsonl --wait --download
```

Paid batch generation must include a total budget:

```bash
lovart jobs run runs/fanren/jobs.jsonl --allow-paid --max-total-credits 300 --wait --download
```

## Config And Planning

`lovart config <model>` returns the legal fields for a model:

- `values` for enum fields.
- `minimum` / `maximum` for numeric fields.
- `minItems` / `maxItems` for array fields.
- `enumerable=false` for free input fields such as prompt or image URLs.

`lovart plan` returns three non-submitting routes:

- `quality_best`: highest legal settings found from config-derived candidates.
- `cost_best`: lowest-cost route, preferring zero-credit combinations.
- `speed_best`: route with fast mode / fast entitlement / fast variant signal. It is not measured wall-clock latency.

If a route has `quote.exact=true`, its credits are exact. If false, run `lovart quote` on the final request before stating exact cost.

## Jobs Semantics

`lovart jobs` expands user-level jobs into `remote_requests`:

- GPT Image 2 with `outputs:10` becomes one remote request with `body.n=10`.
- Seedream 5 with `outputs:10` becomes one remote request with `body.max_images=10`.
- If a model supports only 4 outputs per request, `outputs:10` becomes `4 + 4 + 2`.
- If a model has no quantity field, `outputs:10` becomes 10 single-output remote requests.

State is stored in `runs/<project>/jobs_state.json`. Quote reports are stored in `runs/<project>/jobs_quote.json`. Downloads are saved under `downloads/<task_id>/`.

## Error Handling

- `auth_missing`: run capture/auth extraction.
- `metadata_stale`: run `lovart update sync --metadata-only`, then retry.
- `signer_stale`: do not submit real generation until signing is revalidated.
- `schema_invalid`: fix request JSON according to schema errors.
- `unknown_pricing`: do not submit unless the user provides explicit budget.
- `credit_risk`: retry only with the correct paid budget flags.
- `task_failed` / `timeout`: inspect status, keep state, and use resume when appropriate.

## Main Commands

```bash
lovart setup
lovart --version
lovart self-test
lovart mcp
lovart mcp status
lovart mcp install --clients auto --yes
lovart models
lovart config <model>
lovart plan --intent image-concept
lovart quote <model> --body-file request.json
lovart generate <model> --body-file request.json --mode auto --dry-run
lovart generate <model> --body-file request.json --mode auto --wait --download
lovart jobs quote runs/<project>/jobs.jsonl
lovart jobs dry-run runs/<project>/jobs.jsonl
lovart jobs run runs/<project>/jobs.jsonl --wait --download
lovart jobs status runs/<project>
lovart jobs resume runs/<project>/jobs.jsonl --wait --download
lovart update check
lovart update sync --metadata-only
lovart doctor
```

## Agent Self-Test

An agent understands this project if it can answer:

1. What is the stable machine interface?
   JSON-only stdout envelope from the `lovart` CLI.
2. How do you know legal model parameters?
   Use `lovart config <model>`; never guess.
3. How do you confirm credit cost and submit safety?
   Use `quote` for cost, then `dry-run`/gate before real generation.
4. For 100 concepts with 10 images each, how many JSONL rows?
   100 rows, each with `outputs:10`.
5. Why use `jobs resume` after interruption?
   State may already contain `task_id`; rerunning can duplicate generation and spend. Resume also checks `jobs_file_hash`.

## Reference Docs

- `AGENTS.md`: short hard rules for coding agents.
- `docs/mcp-install.md`: binary install and MCP client setup.
- `docs/agent-contract.md`: field-level CLI JSON reference.
- `docs/reverse_workflow.md`: reverse-maintenance workflow.
- `docs/architecture/file-architecture-philosophy.md`: package architecture rules.

Creative expert methods are intentionally maintained outside this repository. Lovart is the execution backend: CLI contract, safety gates, quoting, jobs, downloads, and reverse-maintenance evidence.

## Runtime Safety

Ignored runtime paths:

- `.lovart/`
- `scripts/creds.json`
- `captures/`
- `downloads/`
- `runs/*/jobs_quote.json`
- `runs/*/jobs_state.json`
- `.lovart-chrome-profile/`
- `.mitmproxy/`
- `.venv/`

Use this only with Lovart requests produced by your own logged-in browser session. Do not use it to bypass login, quota, payment, rate limits, or access controls.
