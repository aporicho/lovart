# Structured Consent Review/Test Matrix

This matrix is the lightweight cross-layer gate for the structured consent design in `docs/architecture/structured-consent.md`. It is intentionally documentation-first: until runtime support lands, reviewers can use it to verify that new MCP schemas, CLI help text, and audit events mention the same Phase 1 fields before business logic is tightened.

## Phase 1 field surface

Phase 1 covers `lovart_generate`, `lovart_jobs_run`, and `lovart_jobs_resume`. Every field below must be kept visible in both the MCP schema review target and the CLI docs/review target when implementation lands.

| Field | Risk area | Phase 1 tools | MCP schema review target | CLI docs/review target | Regression check |
| --- | --- | --- | --- | --- | --- |
| `confirm_paid` | paid | `lovart_generate`, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirm_paid` | `--confirm-paid` or equivalent command docs/help | missing false, present true, zero-credit not-required case |
| `confirmed_max_credits` | paid | `lovart_generate`, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirmed_max_credits` | `--confirmed-max-credits` or equivalent command docs/help | exact match to `max_credits` / `max_total_credits`; actual quote over max blocks submit |
| `confirmed_model` | paid | `lovart_generate`, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirmed_model` | `--confirmed-model` or equivalent command docs/help | normalized model mismatch blocks submit |
| `confirmed_mode` | paid | `lovart_generate`, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirmed_mode` | `--confirmed-mode` or equivalent command docs/help | defaulted/normalized mode mismatch blocks submit |
| `confirmed_outputs` | paid | `lovart_generate`, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirmed_outputs` | `--confirmed-outputs` or equivalent command docs/help | output count / batch row count mismatch blocks submit |
| `confirmed_remote_requests` | paid | `lovart_generate`, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirmed_remote_requests` | `--confirmed-remote-requests` or equivalent command docs/help | generated submit count / resume new-submit count mismatch blocks submit |
| `confirmed_project_id` | remote_write | `lovart_generate`, `lovart_jobs_run` | tool input schema property named `confirmed_project_id` | `--confirmed-project-id` or equivalent command docs/help | explicit project id requires exact project confirmation |
| `confirm_current_project` | remote_write | `lovart_generate`, `lovart_jobs_run` | tool input schema property named `confirm_current_project` | `--confirm-current-project` or equivalent command docs/help | implicit current project requires current-project acknowledgment tied to resolved id |
| `confirmed_canonical_path` | local_write | `lovart_generate` downloads, `lovart_jobs_run`, `lovart_jobs_resume` | tool input schema property named `confirmed_canonical_path` | `--confirmed-canonical-path` or equivalent command docs/help | relative, WSL/Windows, symlink, and default-root path canonicalization cases |

## Cross-layer regression checklist

| Layer | Required checks |
| --- | --- |
| Design doc | `docs/architecture/structured-consent.md` lists each Phase 1 field and names the Phase 1 tools. |
| MCP schema | `lovart_generate`, `lovart_jobs_run`, and `lovart_jobs_resume` schemas expose the paid fields; project and path fields appear where the tool can write remotely or locally. |
| CLI docs/help | Generate/jobs commands mention equivalent flags/options using the same semantics as MCP. |
| Policy gate | Existing `confirm_policy` behavior remains unchanged; structured consent is a second business-confirmation layer. |
| Paid gate | Missing and mismatched paid fields block before submit; zero-credit quotes can skip `confirm_paid` but not remote/local write confirmations. |
| Jobs resume | `confirmed_remote_requests` means only the new submit requests planned for this invocation, not already-submitted historical task ids. |
| Local writes | Path comparison uses canonical absolute paths, not raw user input. |
| Error output | Missing/mismatch errors name the required field and normalized expected value without echoing prompts, full bodies, cookies, tokens, or credential-bearing URLs. |

## Audit/consent test recommendations

When audit support lands, add focused tests around the metadata shape rather than storing raw consent values.

| Audit event | Expected metadata | Negative assertion |
| --- | --- | --- |
| attempt | `correlation_id`; `structured_consent` or `consent.schema_version=structured-consent/v1`; per-field presence status for supplied Phase 1 fields | no prompt body, cookie, token, auth header, full request body, or file contents |
| policy | `correlation_id`; policy decision; whether `confirm_policy` or business confirmation unlocked the outer gate | no secret-bearing request payload |
| quote/gate | `correlation_id`; paid field statuses (`present`, `matched`, `mismatched`, `not_required`); quote credits and max-credit summary | no full prompt or jobs JSONL row |
| submit/write | `correlation_id`; remote request count, project id summary, canonical path summary, file count/overwrite flags where applicable | no credential-bearing download URL query, cookie, token, or file contents |
| finish | `correlation_id`; ok/error code; remote task id or local output summary | no prompt body or full generation body |

Reviewers should prefer asserting the presence of the `structured_consent` metadata key (or the nested `consent.schema_version=structured-consent/v1` equivalent) plus status enums. Do not require raw user-provided confirmation values in audit logs.
