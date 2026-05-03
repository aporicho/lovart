# Lovart Reverse Agent Contract

This repository exposes Lovart generation through a JSON-only CLI. Agents should call the CLI instead of reading credentials, captures, or ref files directly. Use `docs/concepts/LovartCLI生成专家.md` as the generation-methodology guide for turning user goals and prompt documents into executable CLI requests.

Assume `lovart` is on `PATH` via an activated virtualenv or installed wheel. For machine parsing, do not wrap calls with `uv run` because environment-sync messages can contaminate stdout.

## Default Flow

1. Run `lovart setup`.
2. If `data.ready` is false, follow `data.recommended_actions`.
3. Run `lovart models` when the user needs to compare model choices.
4. Run `lovart config <model>` before showing any model-specific choices to the user.
5. Run `lovart plan --intent image-concept` when the model is not fixed, or `lovart plan <model> --intent image-concept` for a fixed model, then present quality, cost, and speed routes before asking for detailed settings.
6. Ask only for route choice, free-input fields, reference assets, count, and budget confirmation when required.
7. Write the model request to a JSON file using route `body_patch` plus user-provided free input.
8. Run `lovart quote <model> --body-file request.json` before stating exact credit cost.
9. Run `lovart generate <model> --body-file request.json --mode auto --dry-run`.
10. Run `lovart generate <model> --body-file request.json --mode auto --wait --download`.
11. If generation fails, inspect `error.code` and `error.details.recommended_actions`.

## Batch Flow

Use this flow when the user asks for a batch, a full prompt document, or many concept images:

1. Run `lovart setup`.
2. Run `lovart plan --intent image-concept` and present the quality, cost, and speed routes.
3. Call `lovart config <model>` for the selected model before writing model-specific fields.
4. Convert the user's prompts into `runs/<project>/jobs.jsonl`; the user should not need to hand-write JSON.
5. Run `lovart jobs quote runs/<project>/jobs.jsonl`.
6. Report total credits, zero-credit jobs, paid jobs, and unknown-pricing jobs.
7. Run `lovart jobs dry-run runs/<project>/jobs.jsonl`.
8. Run `lovart jobs run runs/<project>/jobs.jsonl --wait --download` only if the batch is allowed by the gate.
9. If paid jobs exist, require explicit user budget and use `--allow-paid --max-total-credits N`.
10. Use `lovart jobs status runs/<project>` and `lovart jobs resume runs/<project>/jobs.jsonl --wait --download` after interruption or partial failure.

## Rules

- Treat stdout as the only machine contract.
- Do not parse stderr except for human diagnostics.
- Do not read `.lovart/creds.json`, `scripts/creds.json`, `captures/`, or browser profile files.
- Do not modify `ref/` unless the user explicitly requests reverse-maintenance work.
- Do not bypass `credit_risk`, `unknown_pricing`, `metadata_stale`, or `signer_stale` errors.
- Paid generation requires user intent plus `--allow-paid --max-credits N`.
- Treat `plan.routes[].quote.exact=true` as an exact live quote. If it is false, use `lovart quote` before stating exact cost.
- Do not guess model configuration values. Use only `field.values` returned by `lovart config <model>`.
- Do not infer legal values from descriptions. If `enumerable=false`, ask the user or extract the value from context.
- If the user requests a value outside `field.values`, explain that it is unsupported and show the legal values.
- Do not ask users to pick raw size/quality before showing `lovart plan` routes.
- For batches, do not submit one job before the whole batch has passed quote, dry-run, and budget checks.
- Do not rerun `jobs run` over an existing state with submitted tasks. Use `jobs resume` so existing `task_id` values are not submitted again.

## Config Discovery

Call this before asking the user model-specific questions:

```bash
lovart config <model>
```

For batch image generation, present values from config exactly:

- Size/aspect fields: use only `values`.
- Quality fields: use only `values`.
- Count fields: use `minimum` and `maximum`.
- Media fields: use `minItems` and `maxItems`.
- Prompt fields: `enumerable=false`, so ask the user for content.

## Route Planning

Call this after `config` and before detailed questions:

```bash
lovart plan --intent image-concept
lovart plan <model> --intent image-concept
```

Present all three routes:

- `quality_best`: highest legal quality found from config-derived candidates; may require paid confirmation.
- `cost_best`: lowest-cost route; searches from higher quality down until live quote or entitlement confirms zero credits.
- `speed_best`: fastest route by fast mode, fast variant, or fast entitlement signal; this is not measured wall-clock latency.

After the user chooses a route, merge its `body_patch` with user-provided free input. If a route has `requires_paid_confirmation=true`, ask for an explicit budget before using `--allow-paid --max-credits N`.

## Safe Commands

```bash
lovart setup
lovart models
lovart config <model>
lovart plan --intent image-concept
lovart plan <model>
lovart quote <model> --body-file request.json
lovart jobs quote runs/<project>/jobs.jsonl
lovart jobs dry-run runs/<project>/jobs.jsonl
lovart jobs run runs/<project>/jobs.jsonl --wait --download
lovart jobs status runs/<project>
lovart jobs resume runs/<project>/jobs.jsonl --wait --download
lovart config --global
lovart free <model> --body-file request.json --mode auto
lovart generate <model> --body-file request.json --mode auto --dry-run
lovart generate <model> --body-file request.json --mode auto --wait --download
lovart update check
```

## Error Handling

- `auth_missing`: ask the user to run capture/auth extraction.
- `metadata_stale`: run `lovart update sync --metadata-only`, then retry preflight.
- `signer_stale`: do not submit real generation until signing is revalidated.
- `schema_invalid`: fix the request JSON according to `schema_errors`.
- `unknown_pricing`: do not submit unless the user provides a confirmed budget.
- `credit_risk`: retry only with explicit `--allow-paid --max-credits N`.
- `task_failed` or `timeout`: report the task status and preserve downloaded artifacts, if any.
