# Lovart Reverse

Agent-first Lovart web reverse tooling. The CLI exposes model discovery, schema validation, pricing, zero-credit entitlement checks, update drift detection, generation submission, task lookup, and downloads as stable JSON commands.

Default policy: **zero-credit first**. Real generation is allowed only when preflight proves the request is covered by a zero-credit entitlement, or when the caller explicitly passes `--allow-paid --max-credits N`.

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
lovart config --global
lovart schema openai/gpt-image-2
lovart price openai/gpt-image-2 --body-file request.json --batch 10
lovart free openai/gpt-image-2 --body-file request.json --mode auto
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
lovart update check
lovart update sync --metadata-only
lovart doctor
```

See `docs/agent-contract.md` for Claude Code, Codex, and opencode usage. See `docs/reverse_workflow.md` for capture and reverse-maintenance work.

## Safety

Ignored runtime paths:

- `.lovart/`
- `scripts/creds.json`
- `captures/`
- `downloads/`
- `.lovart-chrome-profile/`
- `.mitmproxy/`
- `.venv/`

Use this only with Lovart requests produced by your own logged-in browser session. Do not use it to bypass login, quota, payment, rate limits, or access controls.
