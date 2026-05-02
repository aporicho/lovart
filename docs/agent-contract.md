# Agent Contract

The Lovart CLI is designed for Claude Code, Codex, opencode, and similar coding agents. The stable integration surface is the CLI JSON envelope.

Install or activate the project first so `lovart` is directly on `PATH`. Agent calls should invoke `lovart ...` directly, not `uv run lovart ...`, because `uv` may print environment messages before the JSON envelope.

## Envelope

Success:

```json
{"ok":true,"data":{},"warnings":[]}
```

Failure:

```json
{"ok":false,"error":{"code":"...","message":"...","details":{}}}
```

stdout must be parsed as JSON. stderr is diagnostic only.

## Recommended Agent Flow

```bash
lovart setup
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
```

Agents should use `--dry-run` before a new model/body shape. Agents may skip dry-run for repeated, already validated requests when `lovart setup` is ready and the user asked for generation.

## Preflight Fields

`generate` returns `data.preflight` with:

- `auth`: secret-safe auth status.
- `update`: Lovart drift status.
- `schema_errors`: request validation problems.
- `gate`: zero-credit or paid-budget decision.
- `can_submit`: whether real submission is allowed.
- `blocking_error`: machine-readable reason when submission is blocked.
- `recommended_actions`: next commands or manual actions.

## Generation Output

`lovart generate ... --wait --download` returns:

- `submitted`
- `task_id`
- `status`
- `artifacts`
- `downloads`
- `preflight`

Downloads are written to `downloads/<task_id>/`.

## Paid Safety

The default path allows zero-credit generation only. Paid generation requires both:

```bash
--allow-paid --max-credits N
```

Agents must not infer or invent a budget. The user must provide it.

## Reverse-Maintenance Boundary

Agents must not read or modify credential files, capture files, or `ref/` snapshots unless the user explicitly asks to reverse or sync Lovart behavior.
