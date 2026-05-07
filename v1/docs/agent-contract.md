# CLI JSON Reference

This is the field-level API reference for agents. For workflow and rules, read `README.md`. For installation and MCP setup, read `docs/mcp-install.md`.

## Envelope

Success:

```json
{"ok":true,"data":{},"execution_class":"local","network_required":false,"remote_write":false,"warnings":[]}
```

Failure:

```json
{"ok":false,"error":{"code":"...","message":"...","details":{}}}
```

stdout is the machine contract. stderr is diagnostic only.

MCP tools return the same envelope as JSON text in their `content[0].text` result. Agents should parse that text exactly like CLI stdout.

Execution metadata is part of the stable envelope:

- `execution_class=local`: local reads and diagnostics only.
- `execution_class=preflight`: current remote checks without remote writes.
- `execution_class=submit`: remote write or generation submission.
- `network_required`, `remote_write`, `submitted`, and `cache_used` make the side effects explicit.

Local caches are used for speed, validation, and resumability. They do not make generation or remote validation available without network access.

## Project Context

The user-visible project context is `project_id`.

- `lovart auth status` reports `project_id_present` and `project_context_ready` without exposing browser context values.
- `lovart project current` returns `project_id` and `project_context_ready`.
- `lovart project select <project_id>` validates the ID against Lovart projects before saving it locally.
- Generation tools may accept a `project_id` override, but internal browser context still comes from login/import.

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

## Quote

`lovart quote <model> --body-file request.json` calls Lovart's signed pricing endpoint.

The quote client mirrors the web UI: it syncs Lovart time once per client, reuses that offset for repeated signed pricing calls, and may add internal `original_unit_data` metadata to the pricing payload. `original_unit_data` is not a user config field and must not be written into user request JSON.

Important quote keys:

- `quoted`
- `credits`
- `payable_credits`
- `listed_credits`
- `credit_basis`
- `request_shape`
- `balance`
- `price_detail`
- `warnings`

`credits` is retained for compatibility and equals `payable_credits`. `payable_credits` comes from Lovart `data.price` and is the actual current-account spend used by generation gates. `listed_credits` comes from `price_detail.total_price` and is the detail/list price, not the actual spend.

Quote is not submit permission. Real single generation runs the generation gate internally before submission.

## Generate

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

`lovart jobs run <jobs.jsonl>` is the public batch generation capability. It validates, expands, internally quotes and gates, saves state, submits, waits, downloads, writes canvas state, and returns a compact summary. It returns:

- `summary.logical_jobs`
- `summary.remote_requests`
- `summary.requested_outputs`
- `summary.total_credits`
- `summary.total_payable_credits`
- `summary.total_listed_credits`
- `summary.cache_hits`
- `summary.cache_misses`
- `summary.signature_groups`
- `summary.remote_status_counts`
- `summary.error_counts`
- `batch_gate`
- `task_count`
- `tasks`
- `failed`
- `downloads`
- `recommended_actions`
- `run_dir`
- `state_file`

Paid batches require explicit `--allow-paid --max-total-credits N`. If the batch gate blocks, the error includes `run_dir`, `state_file`, `batch_gate.total_credits`, and safe `recommended_actions`. Agents should resume that saved state with the explicit budget instead of rebuilding their own quoting flow.

`lovart jobs resume <run_dir>` continues the saved state and never resubmits existing `task_id`s. Use `--retry-failed` only when the user explicitly authorizes retrying failed requests that were never successfully submitted.

Internal quote reuse is based on `cost_signature`. The signature includes model, mode, price-affecting parameters, output count, media input counts/types, and the quote signature version. It excludes prompt/title fields and schema-marked format-only fields. A 0-credit quote may be reused only for the same `cost_signature`.

If internal quoting cannot reach Lovart, the result includes one of `summary.error_counts.network_unavailable`, `summary.error_counts.timestamp_network_unavailable`, or `summary.error_counts.pricing_network_unavailable`, and the command keeps remaining retryable requests pending. The agent should fix DNS/network access to `www.lovart.ai`, then rerun `lovart jobs run <jobs.jsonl>` or `lovart jobs resume <run_dir>`.

`lovart jobs status` defaults to `detail=summary`. Use `--detail requests` when an agent needs every compact remote request, or `--detail full` only when full prompts, request bodies, raw quote data, and expanded jobs are needed.

Compact `summary` detail returns:

- `summary`
- `batch_gate`
- `task_count`
- `tasks`: up to 20 compact task samples, prioritizing failed/running/submitted tasks
- `task_sample_limit`
- `tasks_truncated`
- `download_count`
- `download_dir`
- `failed`
- `timed_out`
- `warnings`
- `recommended_actions`
- `state_file`

`requests` detail additionally returns compact `remote_requests[]` without prompts, full bodies, or raw task payloads.

`full` detail returns the legacy complete payload:

- `summary`
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

For MCP, do not rely on a single long `jobs_run` or `jobs_resume` call to wait for slow models. The MCP wrapper caps wait windows at 90 seconds and returns saved state plus recommended next actions. Agents should call `lovart_jobs_resume` repeatedly, or call `lovart_jobs_status`, until the summary shows no `submitted` or `running` remote requests. Existing `task_id`s in state must never be resubmitted with `jobs run`.

For batch artifact persistence, pass `download_dir` when the user expects files in a project folder; otherwise downloads use the runtime default `downloads/<task_id>/`. Download failures are recorded as `download_failed`, leave the remote request status as `completed`, and can be retried with `jobs_resume`.

Batch state is stored at `runs/<project>/jobs_state.json` with `jobs_file_hash`. If the source `jobs.jsonl` changes, `resume` refuses to continue.

## Common Errors

- `auth_missing`
- `metadata_stale`
- `signer_stale`
- `schema_invalid`
- `unknown_pricing`
- `network_unavailable`
- `credit_risk`
- `task_failed`
- `timeout`
- `input_error`
