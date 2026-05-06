# CLI JSON Reference

This is the field-level API reference for agents. For workflow and rules, read `README.md`. For installation and MCP setup, read `docs/mcp-install.md`.

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

`lovart jobs quote` defaults to lightweight stdout. It returns:

- `summary.logical_jobs`
- `summary.remote_requests`
- `summary.requested_outputs`
- `summary.total_credits`
- `summary.total_payable_credits`
- `summary.total_listed_credits`
- `summary.pending_quote_remote_requests`
- `summary.effective_limit`
- `summary.cache_hits`
- `summary.cache_misses`
- `summary.signature_groups`
- `summary.quoted_representative_requests`
- `state_file`
- `quote_file`
- `full_quote_file`
- `quote_cache_file`

Use `lovart jobs quote <jobs.jsonl> --detail requests` for compact per-request quote summaries. Use `--detail full` only when full prompts, request bodies, raw quote data, and expanded jobs are needed.

`lovart jobs quote` accepts either positional `<jobs.jsonl>` or `--jobs-file <jobs.jsonl>`. The default limit is `auto`: batches with more than 100 pending remote requests process 100 at a time. Use `--all` only when a caller intentionally wants to quote all pending requests in one command.

Quote state is isolated per jobs file at `<run_dir>/.lovart_quote/<jobs-stem>-<jobs-hash>/jobs_quote_state.json`, so multiple batch files can share one run directory without hash conflicts. Use `lovart jobs quote-status <run_dir>` to list all quote states, or `lovart jobs quote-status <run_dir> --jobs-file <jobs.jsonl>` to inspect one file.

Quote reuse is based on `cost_signature`. The signature includes model, mode, price-affecting parameters, output count, media input counts/types, and the quote signature version. It excludes prompt/title fields and schema-marked format-only fields. A 0-credit quote may be reused only for the same `cost_signature`.

If live quote cannot reach Lovart, the quote report includes `summary.network_unavailable_remote_requests`, one of `summary.error_counts.network_unavailable`, `summary.error_counts.timestamp_network_unavailable`, or `summary.error_counts.pricing_network_unavailable`, and a matching `quote_blocker.code` when the whole quote run is blocked. In that case the CLI stops early, keeps remaining retryable requests pending, and the agent should fix DNS/network access to `www.lovart.ai` before rerunning the same quote command.

`lovart jobs dry-run|run|resume` can return compact or full detail. CLI defaults to full for `run/resume` for backward compatibility, while MCP defaults to `detail=summary` to avoid oversized tool results. `lovart jobs status` defaults to `detail=summary`.

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

For batch artifact persistence, pass `download=true`. Pass `download_dir` when the user expects files in a project folder; otherwise downloads use the runtime default `downloads/<task_id>/`. Download failures are recorded as `download_failed`, leave the remote request status as `completed`, and can be retried with `jobs_resume` plus `download=true`.

Batch state is stored at `runs/<project>/jobs_state.json` with `jobs_file_hash`. If the source `jobs.jsonl` changes, `resume` refuses to continue.

Batch quote state is stored under `runs/<project>/.lovart_quote/` with `jobs_file_hash`. If the source `jobs.jsonl` changes, `jobs quote` creates a new per-file state instead of reusing the old state. `--refresh` discards the current file's quote state and starts over.

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
