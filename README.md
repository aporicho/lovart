# Lovart Reverse

Agent-first Lovart web reverse tooling. The CLI exposes model discovery, schema validation, live credit quotes, zero-credit entitlement checks, update drift detection, generation submission, local batch jobs, task lookup, and downloads as stable JSON commands.

Default policy: **zero-credit first**. Real generation is allowed only when preflight proves the request is covered by a zero-credit entitlement, or when the caller explicitly passes `--allow-paid --max-credits N`. Batch generation requires `--allow-paid --max-total-credits N` when any job costs credits.

## 5-Minute Quickstart

Install dependencies:

```bash
uv sync --no-editable
source .venv/bin/activate
```

Check readiness:

```bash
lovart setup
```

List models and inspect exact legal config values:

```bash
lovart models
lovart plan --intent image-concept
lovart config openai/gpt-image-2
lovart plan openai/gpt-image-2 --intent image-concept --quote live
lovart quote openai/gpt-image-2 --body-file request.json
```

If setup reports missing auth, capture a browser request and extract credentials:

```bash
lovart reverse capture
lovart auth extract captures/<lovart-request>.json
```

Create a request file:

```json
{
  "prompt": "a clean product render of a red cube on a white background",
  "quality": "low",
  "size": "1024*1024"
}
```

Dry-run preflight without submitting:

```bash
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
```

Submit, wait, and download artifacts:

```bash
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
```

Paid generation must be explicit:

```bash
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --allow-paid --max-credits 5 --wait --download
```

## Batch Quickstart

Agents convert prompt documents into `jobs.jsonl`; humans should not need to hand-write batch JSON. Each line is one Lovart generation request:

```json
{"job_id":"001","title":"青竹峰晨雾中的韩立","model":"openai/gpt-image-2","mode":"auto","body":{"prompt":"...","quality":"high","size":"1024*1024","n":1}}
```

Quote, dry-run, submit the whole batch, then wait and download:

```bash
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

## JSON Contract

stdout is JSON only. Diagnostics go to stderr.

Success:

```json
{"ok":true,"data":{},"warnings":[]}
```

Failure:

```json
{"ok":false,"error":{"code":"auth_missing","message":"...","details":{}}}
```

Generated files are saved under `downloads/<task_id>/` when `--download` is used.

## Main Commands

```bash
lovart setup
lovart models
lovart config openai/gpt-image-2
lovart plan --intent image-concept
lovart plan openai/gpt-image-2 --intent image-concept --quote live
lovart quote openai/gpt-image-2 --body-file request.json
lovart jobs quote runs/fanren/jobs.jsonl
lovart jobs dry-run runs/fanren/jobs.jsonl
lovart jobs run runs/fanren/jobs.jsonl --wait --download
lovart jobs status runs/fanren
lovart jobs resume runs/fanren/jobs.jsonl --wait --download
lovart config --global
lovart schema openai/gpt-image-2
lovart free openai/gpt-image-2 --body-file request.json --mode auto
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
lovart update check
lovart update sync --metadata-only
lovart doctor
```

See `AGENTS.md` for the machine entry contract, `docs/concepts/LovartCLI生成专家.md` for generation methodology, `docs/agent-contract.md` for Claude Code, Codex, and opencode usage, and `docs/reverse_workflow.md` for capture and reverse-maintenance work.

## Safety

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
