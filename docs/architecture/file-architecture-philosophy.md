# Lovart Reverse File Architecture Philosophy

This project is an agent-facing reverse-engineering toolkit. Its file structure must make the stable API boundary obvious and keep capture evidence, runtime credentials, and production command logic separate.

## Package Boundaries

- `lovart_reverse/auth/` owns credential loading, capture extraction, and secret-safe summaries.
- `lovart_reverse/signing/` owns LGW signing, time sync, and signing fixtures.
- `lovart_reverse/http/` owns Lovart HTTP sessions and base URLs.
- `lovart_reverse/discovery/` owns live and reference generator list/schema retrieval.
- `lovart_reverse/config/` owns exhaustive agent-facing configuration discovery from schema values.
- `lovart_reverse/registry/` owns model records, schema lookup, and request validation.
- `lovart_reverse/pricing/` owns live quote requests and raw pricing metadata fetches used only for update drift checks.
- `lovart_reverse/entitlement/` owns fast zero-credit and relaxed unlimited checks.
- `lovart_reverse/planning/` owns non-submitting route planning from config, pricing, entitlement, and readiness.
- `lovart_reverse/generation/` owns dry-run previews, paid gate evaluation, and submission.
- `lovart_reverse/jobs/` owns local batch queue parsing, whole-batch quote/preflight, submission orchestration, state, resume, and batch downloads.
- `lovart_reverse/setup/` owns one-shot readiness checks for auth, refs, signer, update status, and runtime paths.
- `lovart_reverse/task/` owns task status normalization.
- `lovart_reverse/assets/` owns upload APIs only after capture evidence confirms them.
- `lovart_reverse/downloads/` owns artifact downloads.
- `lovart_reverse/update/` owns online drift detection, metadata sync, and manifest comparison.
- `lovart_reverse/capture/` owns mitm capture support and replay.
- `lovart_reverse/cli/` is the only command-line implementation.

## Rules

- `__init__.py` files export module APIs only. Business logic lives in named modules.
- Do not add vague modules named `utils.py`, `helpers.py`, `common.py`, `legacy.py`, `compat.py`, or `glue.py`.
- Business modules must not import `lovart_reverse.cli`.
- CLI stdout must be JSON only. Diagnostics and live fallback warnings go to stderr.
- `scripts/` is limited to a single development entry point, `scripts/lovart.py`.
- Runtime artifacts stay out of git: credentials, captures, downloads, browser profiles, and local env files.
- Unconfirmed reverse surfaces are represented as explicit status APIs, not guessed production wrappers.

## Update Discipline

`ref/lovart_manifest.json` records hashes for the Lovart canvas HTML, static JS list, signer WASM, generator list, generator schema, pricing table, and entitlement shape. `lovart update check` is read-only. `lovart update sync --metadata-only` refreshes metadata snapshots without generating content.

When frontend or signer artifacts change, real generation must be treated as unsafe until signing fixtures are revalidated. When pricing or entitlement hashes change, batch generation must rerun live quote and free checks.
