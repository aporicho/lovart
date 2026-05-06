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

Normal users should use the release installer. It downloads the self-contained `lovart` binary, installs the Lovart Connector extension files, and configures supported MCP clients for `lovart mcp`. It requires GitHub CLI authentication:

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
lovart doctor
lovart mcp status
```

Load the extension from the path printed by the installer:

1. Open `chrome://extensions`.
2. Enable Developer mode.
3. Click `Load unpacked` and select `~/.lovart-reverse/extension/lovart-connector`.

Then connect your Lovart browser session:

```bash
lovart auth login
lovart doctor
lovart project list
lovart project select <project_id>
```

Direct binary download is the fallback path.

macOS arm64:

```bash
mkdir -p ~/.local/bin
gh release download --repo aporicho/lovart-reverse --pattern "lovart-macos-arm64" -O ~/.local/bin/lovart
chmod +x ~/.local/bin/lovart
lovart --version
lovart doctor
```

Linux x64:

```bash
mkdir -p ~/.local/bin
gh release download --repo aporicho/lovart-reverse --pattern "lovart-linux-x64" -O ~/.local/bin/lovart
chmod +x ~/.local/bin/lovart
lovart --version
lovart doctor
```

Windows x64:

```powershell
gh release download --repo aporicho/lovart-reverse --pattern "lovart-windows-x64.exe" -O "$env:USERPROFILE\bin\lovart.exe"
lovart --version
lovart doctor
```

If `lovart --version` shows an older command set or a different git commit than expected, replace the binary before using it from an agent.

The installer writes MCP config for detected MCP clients. Manual Codex config is:

```toml
[mcp_servers.lovart]
command = "/absolute/path/to/lovart"
args = ["mcp"]
```

MCP client configuration can also be inspected or written directly:

```bash
lovart mcp status --clients auto
lovart mcp install --clients auto --yes
lovart mcp install --clients codex --dry-run --yes
```

Supported client selectors are `auto`, `all`, `none`, or a comma-separated list of `codex`, `claude`, `opencode`, and `openclaw`.

Python installs are for developers and reverse maintainers:

```bash
uv tool install git+ssh://git@github.com/aporicho/lovart-reverse.git
uv tool install "git+ssh://git@github.com/aporicho/lovart-reverse.git#egg=lovart-reverse[reverse]"
```

If auth is missing and the connector cannot be used, advanced users can import copied browser credentials:

```bash
lovart auth import --help
```

Reverse maintainers can still capture and extract credentials from a Python environment with the `reverse` extra:

```bash
lovart-reverse start
lovart-reverse extract captures/<lovart-request>.json
```

`lovart-reverse start` launches mitmproxy, opens an isolated Chrome profile through the proxy, and writes Lovart traffic into `captures/`. Stop it with Ctrl-C after the browser flow is complete. `lovart-reverse capture` remains available as a low-level command printer when you need to start mitmproxy manually.

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
lovart auth status
lovart setup
lovart config openai/gpt-image-2
lovart quote openai/gpt-image-2 --body-file request.json
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
```

For quick single-prompt submissions, `generate` also accepts a minimal prompt body:

```bash
lovart generate openai/gpt-image-2 --prompt "a clean product render of a red cube" --mode auto --dry-run
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
lovart config seedream/seedream-5-0
lovart jobs quote runs/fanren/jobs.jsonl
lovart jobs dry-run runs/fanren/jobs.jsonl
lovart jobs run runs/fanren/jobs.jsonl --wait --download --download-dir runs/fanren/images --detail summary
lovart jobs status runs/fanren
lovart jobs resume runs/fanren --wait --download --download-dir runs/fanren/images --timeout-seconds 90 --detail summary
```

Paid batch generation must include a total budget:

```bash
lovart jobs run runs/fanren/jobs.jsonl --allow-paid --max-total-credits 300 --wait --download
```

## Config

`lovart config <model>` returns the legal fields for a model:

- `values` for enum fields.
- `minimum` / `maximum` for numeric fields.
- `minItems` / `maxItems` for array fields.
- `enumerable=false` for free input fields such as prompt or image URLs.

Run `lovart quote` on the final request before stating exact cost.

## Jobs Semantics

`lovart jobs` expands user-level jobs into `remote_requests`:

- GPT Image 2 with `outputs:10` becomes one remote request with `body.n=10`.
- Seedream 5 with `outputs:10` becomes one remote request with `body.max_images=10`.
- If a model supports only 4 outputs per request, `outputs:10` becomes `4 + 4 + 2`.
- If a model has no quantity field, `outputs:10` becomes 10 single-output remote requests.

Generation state is stored in `runs/<project>/jobs_state.json`. Quote progress is isolated per jobs file at `runs/<project>/.lovart_quote/<jobs-stem>-<jobs-hash>/jobs_quote_state.json`. The default quote report in that directory is lightweight; full quote detail is stored beside it as `jobs_quote_full.json`.

`lovart jobs quote` defaults to a lightweight summary and does not echo prompts or full request bodies to stdout. Use `--detail requests` for compact per-request status, and `--detail full` only when you really need the complete expanded jobs and quote raw data.

`lovart jobs status` also defaults to a lightweight summary. It returns counts, up to 20 compact task samples, warnings, and safe `recommended_actions`; it does not echo prompts, full request bodies, or raw task payloads unless `--detail full` is explicitly requested. Use `--detail requests` when an agent needs every compact remote request.

For long-running models, especially MCP calls, use short resumable polling windows instead of one very long tool call: `lovart jobs resume <run_dir> --wait --download --download-dir <images-dir> --timeout-seconds 90 --detail summary`. If the local wait times out, submitted `task_id`s are already saved in `jobs_state.json`; rerun `resume` or `status` to continue without resubmitting.

`--download` writes artifact files locally. Without `--download-dir`, files go under the runtime downloads directory, normally `downloads/<task_id>/`. With `--download-dir`, files go under `<download-dir>/<task_id>/`. Download failures keep the remote task marked `completed` and are resumable with `jobs resume --download`.

Batch quote reuses one web-style pricing client for each command run: Lovart time is synced once, signed pricing requests reuse that offset, and internal `original_unit_data` may be added only to the pricing payload. Users and agents should not put `original_unit_data` in request JSON.

The quote command computes a strict `cost_signature` from model, mode, price-affecting parameters, output count, and media input counts. Requests with the same signature share one remote quote; prompt/title changes alone do not trigger another quote. A 0-credit result is reusable only within the same signature, never across other parameter combinations.

If DNS or network access to `www.lovart.ai` fails, quote stops early with `network_unavailable` and leaves the remaining retryable requests pending. Fix network/DNS, then rerun the same `lovart jobs quote ...` command.

Batch quote credit fields:

- `summary.total_credits` equals `summary.total_payable_credits`.
- `payable_credits` comes from Lovart `data.price` and is the actual current-account spend used by gates.
- `listed_credits` comes from `price_detail.total_price` and is the detail/list price, not the actual spend.

## Error Handling

- `auth_missing`: run `lovart auth login`; use `lovart auth import --help` as an advanced fallback.
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
lovart version
lovart auth status
lovart auth login
lovart auth import --help
lovart auth logout --yes
lovart doctor
lovart self-test
lovart mcp
lovart mcp status
lovart balance
lovart models
lovart config <model>
lovart quote <model> --body-file request.json
lovart generate <model> --body-file request.json --mode auto --dry-run
lovart generate <model> --prompt "prompt text" --mode auto --dry-run
lovart generate <model> --body-file request.json --mode auto --wait --download
lovart jobs quote runs/<project>/jobs.jsonl
lovart jobs dry-run runs/<project>/jobs.jsonl
lovart jobs run runs/<project>/jobs.jsonl --wait --download
lovart jobs status runs/<project>
lovart jobs resume runs/<project> --wait --download
lovart update check
lovart update sync --metadata-only
lovart project admin repair-canvas [project_id]
lovart dev sign
```

## Execution Semantics

Every JSON success envelope identifies what the command actually did:

- `execution_class=local`: reads local files, credentials, registry data, or saved job state. It never contacts Lovart.
- `execution_class=preflight`: contacts Lovart or checks current remote state without creating generation tasks or mutating remote projects.
- `execution_class=submit`: performs a remote write, such as creating a generation task.

Local registry, manifest, quote state, and job state are caches for speed,
validation, and resumability. They are not a standalone operating mode;
generation and remote validation require network access to Lovart.

## Doctor And Self-Test

`lovart doctor` is the primary user diagnostic. By default it is local-only:
it does not submit tasks, generate images, spend credits, or contact Lovart.
Use `lovart doctor --online` when you also want Lovart network/update status.

`lovart self-test` remains available as the lower-level local diagnostic. Both
commands check credentials, project context, signer WASM, generator metadata,
and the local registry, then return one of three statuses:

- `ready`: local generation prerequisites are present.
- `needs_setup`: required runtime files or project context are missing.
- `broken`: a local file exists but is unreadable, malformed, or unusable.

Use the returned `recommended_actions` as the next commands to run.

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
