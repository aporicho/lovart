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
lovart models
lovart config openai/gpt-image-2
lovart plan openai/gpt-image-2 --intent image-concept
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
```

Agents must call `config`, then `plan`, before presenting model-specific options. Agents should use `--dry-run` before a new model/body shape.

## Config Discovery

`lovart config <model>` returns exhaustive legal configuration values derived from the Lovart schema.

Each field includes:

- `key`
- `type`
- `required`
- `visible`
- `default`
- `description`
- `source`
- `enumerable`
- `values` when the legal value set is enumerable
- `minimum` / `maximum` for numeric ranges
- `minItems` / `maxItems` for arrays

Allowed sources:

- `schema.inline_enum`
- `schema.ref_enum:<Name>`
- `schema.boolean`
- `schema.range`
- `schema.array_limits`
- `schema.free_input`

Agents must not guess values. If `values` exists, it is the complete legal set. If `enumerable=false`, the field needs user/context input.

For example, an agent may say:

- `quality` supports only `auto`, `high`, `medium`, `low`.
- `size` supports only the exact strings returned in `size.values`.
- `n` supports integers from `minimum` to `maximum`.
- `prompt` is free input and must be supplied by the user or generated from the user's request.

Agents must not say values like `1280x720`, `portrait`, or `ultra` unless those exact values are returned in `field.values`.

## Route Planning

`lovart plan <model>` returns three non-submitting routes for user-facing choice:

- `quality_best`: highest-quality legal settings; paid confirmation may be required.
- `cost_best`: lowest-cost route; prioritizes zero-credit eligibility.
- `speed_best`: fastest route; prioritizes `fast` mode.

Useful flags:

```bash
lovart plan openai/gpt-image-2 --intent image-concept
lovart plan openai/gpt-image-2 --count 4
lovart plan openai/gpt-image-2 --body-file partial-request.json
lovart plan openai/gpt-image-2 --offline
```

Each route includes `body_patch`, `request_body`, `estimated_credits`, `zero_credit`, `requires_paid_confirmation`, `constraints`, and `user_message`. `body_patch` never fabricates free-input fields such as `prompt` or reference image URLs. Agents merge `body_patch` with user-provided free input before running `generate --dry-run`.

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
