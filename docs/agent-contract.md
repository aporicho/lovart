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
lovart plan --intent image-concept
lovart plan openai/gpt-image-2 --intent image-concept
lovart quote openai/gpt-image-2 --body-file request.json
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --wait --download
lovart jobs quote runs/fanren/jobs.jsonl
lovart jobs dry-run runs/fanren/jobs.jsonl
lovart jobs run runs/fanren/jobs.jsonl --wait --download
```

Agents must call `plan` before presenting route choices. For a fixed model, call `config`, then `plan <model>`. If the model is not fixed, call `plan --intent image-concept` to compare model candidates, then call `config <selected-model>` before offering parameter edits. Agents should use `--dry-run` before a new model/body shape.

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

`lovart plan` returns three non-submitting routes for user-facing choice. The model argument is optional:

- `quality_best`: highest legal settings found from config-derived candidates; paid confirmation may be required.
- `cost_best`: lowest-cost route; searches from higher quality down until quote or entitlement confirms zero credits.
- `speed_best`: fastest route by `fast` mode, fast model variant, or fast entitlement signal; it is not measured latency.

Useful flags:

```bash
lovart plan --intent image-concept
lovart plan openai/gpt-image-2 --intent image-concept
lovart plan openai/gpt-image-2 --count 4
lovart plan openai/gpt-image-2 --body-file partial-request.json
lovart plan openai/gpt-image-2 --quote live
lovart plan openai/gpt-image-2 --quote auto
lovart plan openai/gpt-image-2 --quote offline
```

Each route includes `model`, `mode`, `body_patch`, `request_body`, `quote`, `zero_credit`, `requires_paid_confirmation`, `constraints`, `degraded_steps`, `quality_score`, `cost_score`, `speed_score`, and `user_message`. `body_patch` never fabricates free-input fields such as `prompt` or reference image URLs. When `quote.exact=true`, `quote.credits` is the live pricing result. When `quote.exact=false`, run `lovart quote` on the final request before stating exact cost. Agents merge `body_patch` with user-provided free input before running `generate --dry-run`.

## Exact Quote

`lovart quote <model> --body-file request.json` calls Lovart's signed `POST /v1/generator/pricing` endpoint. Use it before stating exact credit cost.

The quote response includes:

- `credits`: exact pre-submit credit cost shown by Lovart.
- `balance`: current account balance returned by Lovart.
- `price_detail`: Lovart's cost breakdown, including normalized resolution, unit price, unit count, and surcharge fields.

Agents may use `plan` to present candidate routes. If the chosen route already has `quote.exact=true`, its `quote.credits` can be used for budget confirmation; otherwise use `quote` on the final request.

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

## Batch Jobs

`lovart jobs` is the stable local queue interface for batch generation. It does not require Lovart to expose a native batch endpoint. Agents build a JSONL file and let the CLI manage quote, dry-run, submission, task polling, downloads, and resume.

Each JSONL line is one job:

```json
{"job_id":"001","title":"青竹峰晨雾中的韩立","model":"openai/gpt-image-2","mode":"auto","body":{"prompt":"...","quality":"high","size":"1024*1024","n":1}}
```

Commands:

```bash
lovart jobs quote runs/fanren/jobs.jsonl
lovart jobs dry-run runs/fanren/jobs.jsonl
lovart jobs run runs/fanren/jobs.jsonl --wait --download
lovart jobs run runs/fanren/jobs.jsonl --allow-paid --max-total-credits 300 --wait --download
lovart jobs status runs/fanren
lovart jobs resume runs/fanren/jobs.jsonl --wait --download
```

`jobs run` uses a two-stage contract: the entire batch must pass schema validation, live quote, update/signing readiness, and paid-budget gate before any job is submitted. After that, all pending jobs are submitted first; once task IDs are recorded, the CLI polls and downloads results.

Batch state is stored at `runs/<project>/jobs_state.json`. Agents must use `jobs resume` after interruption so submitted jobs with existing `task_id` values are not submitted again.

Batch statuses:

- `pending`
- `submitted`
- `running`
- `completed`
- `downloaded`
- `failed`
- `skipped`

The jobs envelope includes `summary`, `batch_gate`, `submitted`, `tasks`, `downloads`, `failed`, `timed_out`, `state_file`, and full per-job state.

## Paid Safety

The default path allows zero-credit generation only. Paid generation requires both:

```bash
--allow-paid --max-credits N
```

Agents must not infer or invent a budget. The user must provide it.

For batches, paid generation requires both:

```bash
--allow-paid --max-total-credits N
```

The batch is refused if any job has unknown pricing, if the total quote exceeds `N`, or if paid jobs exist without explicit budget authorization.

## Reverse-Maintenance Boundary

Agents must not read or modify credential files, capture files, or `ref/` snapshots unless the user explicitly asks to reverse or sync Lovart behavior.
