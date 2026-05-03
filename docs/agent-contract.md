# CLI JSON Reference

This is the field-level API reference for agents. For workflow and rules, read `README.md`. For installation and MCP setup, read `docs/agent-install.md`.

## Envelope

Success:

```json
{"ok":true,"data":{},"warnings":[]}
```

Failure:

```json
{"ok":false,"error":{"code":"...","message":"...","details":{}}}
```

stdout is the machine contract. stderr is diagnostic only.

MCP tools return the same envelope as JSON text in their `content[0].text` result. Agents should parse that text exactly like CLI stdout.

## Config

`lovart config <model>` returns legal model configuration fields.

Important field keys:

- `key`
- `type`
- `required`
- `visible`
- `default`
- `description`
- `source`
- `enumerable`
- `values`
- `minimum` / `maximum`
- `minItems` / `maxItems`
- `route_role`

If `values` exists, it is the legal enumerable set. If `enumerable=false`, the value must come from user/context input.

## Plan

`lovart plan` returns non-submitting route candidates.

Important route keys:

- `id`
- `model`
- `mode`
- `body_patch`
- `request_body`
- `quote`
- `zero_credit`
- `requires_paid_confirmation`
- `constraints`
- `degraded_steps`
- `quality_score`
- `cost_score`
- `speed_score`
- `user_message`

If `quote.exact=true`, `quote.credits` is exact. If false, run `lovart quote` on the final request before stating exact cost.

## Quote

`lovart quote <model> --body-file request.json` calls Lovart's signed pricing endpoint.

Important quote keys:

- `quoted`
- `credits`
- `balance`
- `price_detail`
- `warnings`

Quote is not submit permission. Use dry-run before real generation.

## Generate

`lovart generate ... --dry-run` returns:

- `submitted`
- `preflight`
- `request`

`lovart generate ... --wait --download` returns:

- `submitted`
- `task_id`
- `status`
- `artifacts`
- `downloads`
- `preflight`

Important `preflight` keys:

- `auth`
- `update`
- `schema_errors`
- `gate`
- `can_submit`
- `blocking_error`
- `recommended_actions`

## Jobs

Each `jobs.jsonl` line is a user-level job:

```json
{"job_id":"001","title":"青竹峰晨雾中的韩立","model":"seedream/seedream-5-0","mode":"relax","outputs":10,"body":{"prompt":"...","aspect_ratio":"4:3","resolution":"2K","response_format":"url","watermark":false}}
```

When `outputs` is present, `body` must not include `n`, `max_images`, or `count`.

`lovart jobs quote|dry-run|run|resume` return:

- `summary.logical_jobs`
- `summary.remote_requests`
- `summary.requested_outputs`
- `summary.total_credits`
- `batch_gate`
- `submitted`
- `remote_requests`
- `tasks`
- `downloads`
- `failed`
- `timed_out`
- `state_file`
- `jobs`

Important `remote_requests[]` keys:

- `request_id`
- `job_id`
- `model`
- `mode`
- `output_count`
- `body`
- `quote`
- `preflight`
- `task_id`
- `status`
- `downloads`

Batch state is stored at `runs/<project>/jobs_state.json` with `jobs_file_hash`. If the source `jobs.jsonl` changes, `resume` refuses to continue.

## Common Errors

- `auth_missing`
- `metadata_stale`
- `signer_stale`
- `schema_invalid`
- `unknown_pricing`
- `credit_risk`
- `task_failed`
- `timeout`
- `input_error`
