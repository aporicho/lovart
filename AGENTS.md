# Lovart Reverse Agent Contract

This repository exposes Lovart generation through a JSON-only CLI. Agents should call the CLI instead of reading credentials, captures, or ref files directly.

Assume `lovart` is on `PATH` via an activated virtualenv or installed wheel. For machine parsing, do not wrap calls with `uv run` because environment-sync messages can contaminate stdout.

## Default Flow

1. Run `lovart setup`.
2. If `data.ready` is false, follow `data.recommended_actions`.
3. Write the model request to a JSON file.
4. Run `lovart generate <model> --body-file request.json --mode auto --wait --download`.
5. If generation fails, inspect `error.code` and `error.details.recommended_actions`.

## Rules

- Treat stdout as the only machine contract.
- Do not parse stderr except for human diagnostics.
- Do not read `.lovart/creds.json`, `scripts/creds.json`, `captures/`, or browser profile files.
- Do not modify `ref/` unless the user explicitly requests reverse-maintenance work.
- Do not bypass `credit_risk`, `unknown_pricing`, `metadata_stale`, or `signer_stale` errors.
- Paid generation requires user intent plus `--allow-paid --max-credits N`.

## Safe Commands

```bash
lovart setup
lovart models
lovart schema <model>
lovart price <model> --body-file request.json
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
