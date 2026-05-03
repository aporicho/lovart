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
lovart reverse start
lovart auth extract captures/<lovart-request>.json
```

`lovart reverse start` launches mitmproxy, opens an isolated Chrome profile through the proxy, and writes Lovart traffic into `captures/`. Stop it with Ctrl-C after the browser flow is complete. `lovart reverse capture` remains available as a low-level command printer when you need to start mitmproxy manually.

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
lovart jobs quote runs/fanren/jobs.jsonl --limit 25
lovart jobs quote-status runs/fanren
lovart jobs dry-run runs/fanren/jobs.jsonl
lovart jobs run runs/fanren/jobs.jsonl --wait --download --detail summary
lovart jobs status runs/fanren
lovart jobs resume runs/fanren/jobs.jsonl --wait --download --timeout-seconds 90 --detail summary
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

Generation state is stored in `runs/<project>/jobs_state.json`. Quote progress is isolated per jobs file at `runs/<project>/.lovart_quote/<jobs-stem>-<jobs-hash>/jobs_quote_state.json`. The default quote report in that directory is lightweight; full quote detail is stored beside it as `jobs_quote_full.json`.

`lovart jobs quote` defaults to a lightweight summary and does not echo prompts or full request bodies to stdout. Use `--detail requests` for compact per-request status, and `--detail full` only when you really need the complete expanded jobs and quote raw data.

`lovart jobs status` also defaults to a lightweight summary. It returns counts, up to 20 compact task samples, warnings, and safe `recommended_actions`; it does not echo prompts, full request bodies, or raw task payloads unless `--detail full` is explicitly requested. Use `--detail requests` when an agent needs every compact remote request.

For long-running models, especially MCP calls, use short resumable polling windows instead of one very long tool call: `lovart jobs resume <jobs.jsonl> --wait --download --timeout-seconds 90 --detail summary`. If the local wait times out, submitted `task_id`s are already saved in `jobs_state.json`; rerun `resume` or `status` to continue without resubmitting.

Batch quote reuses one web-style pricing client for each command run: Lovart time is synced once, signed pricing requests reuse that offset, and internal `original_unit_data` may be added only to the pricing payload. Users and agents should not put `original_unit_data` in request JSON.

The quote command computes a strict `cost_signature` from model, mode, price-affecting parameters, output count, and media input counts. Requests with the same signature share one live quote; prompt/title changes alone do not trigger another quote. A 0-credit result is reusable only within the same signature, never across other parameter combinations.

For large batches, `--limit auto` is the default: more than 100 pending remote requests are processed 100 at a time. Rerun the same `lovart jobs quote <jobs.jsonl>` command until `summary.pending_quote_remote_requests` is `0`, or pass `--all` when you intentionally want to quote every pending request in one command.

If DNS or network access to `www.lovart.ai` fails, quote stops early with `network_unavailable` and leaves the remaining retryable requests pending. Fix network/DNS, then rerun the same `lovart jobs quote ...` command. Use `--refresh` only when you intentionally want to discard that jobs file's quote state.

Batch quote credit fields:

- `summary.total_credits` equals `summary.total_payable_credits`.
- `payable_credits` comes from Lovart `data.price` and is the actual current-account spend used by gates.
- `listed_credits` comes from `price_detail.total_price` and is the detail/list price, not the actual spend.

## Error Handling

- `auth_missing`: run capture/auth extraction.
- `metadata_stale`: run `lovart update sync --metadata-only`, then retry.
- `signer_stale`: do not submit real generation until signing is revalidated.
- `schema_invalid`: fix request JSON according to schema errors.
- `unknown_pricing`: do not submit unless the user provides explicit budget.
- `network_unavailable` / `timestamp_network_unavailable` / `pricing_network_unavailable`: fix DNS/network access to `www.lovart.ai`, then rerun quote.
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
- `runs/`
- `.lovart-chrome-profile/`
- `.mitmproxy/`
- `.venv/`

Use this only with Lovart requests produced by your own logged-in browser session. Do not use it to bypass login, quota, payment, rate limits, or access controls.
