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
- `lovart_reverse/commands/` owns the safe command facade shared by CLI and MCP wrappers.
- `lovart_reverse/mcp/` owns the safe stdio MCP wrapper and must not expose capture, credential extraction, reverse replay submission, metadata sync, or direct `ref/` mutation.
- `packaging/pyinstaller/` owns the single-binary build spec. Release binaries expose both CLI commands and MCP through `lovart mcp`.
- `packaging/install/` owns release installer scripts. They download binaries, verify checksums, and call `lovart mcp install`; they must not implement generation logic.
- `.github/workflows/` owns release automation. It may build and upload binaries, but must not embed credentials or captures.
- `lovart_reverse/setup/` owns one-shot readiness checks for auth, refs, signer, update status, and runtime paths.
- `lovart_reverse/task/` owns task status normalization.
- `lovart_reverse/assets/` owns upload APIs only after capture evidence confirms them.
- `lovart_reverse/downloads/` owns artifact downloads.
- `lovart_reverse/update/` owns online drift detection, metadata sync, and manifest comparison.
- `lovart_reverse/capture/` owns mitm capture support and replay.
- `lovart_reverse/cli/` is the only command-line implementation. `cli/main.py` stays a thin console-script entry point; command parsing and dispatch live in named CLI application modules.
- `lovart_reverse/data/ref/` stores packaged read-only metadata snapshots so global installs work outside the source checkout.

## Primary Module Names

- `config/schema_config.py` turns Lovart request schema into agent-facing config fields.
- `discovery/generators.py` fetches live generator list and schema metadata.
- `entitlement/checks.py` checks fast zero-credit and relaxed unlimited eligibility.
- `setup/readiness.py` reports setup, auth, metadata, signer, and runtime readiness.
- `update/drift.py` compares online Lovart state with local metadata snapshots.
- `planning/planner.py` builds non-submitting quality, cost, and speed routes.
- `jobs/orchestrator.py` coordinates user-level batch quote, dry-run, run, status, and resume.
- `commands/facade.py` exposes safe command functions used by both CLI and MCP.
- `mcp/server.py` maps safe MCP tools to command facade calls and returns CLI-compatible JSON envelopes.
- `cli/application.py` owns argparse wiring and command dispatch; `cli/main.py` only delegates to it.

## Rules

- `__init__.py` files export module APIs only. Business logic lives in named modules.
- Do not add vague modules named `utils.py`, `helpers.py`, `common.py`, `service.py`, `legacy.py`, `compat.py`, or `glue.py`.
- Business modules must not import `lovart_reverse.cli`.
- CLI stdout must be JSON only. Diagnostics and live fallback warnings go to stderr.
- MCP tool results must wrap the same JSON envelope as CLI stdout.
- Normal agent distribution is binary-first. Python package installs are for development and reverse maintenance.
- Reverse-only dependencies such as mitmproxy must be optional extras, not default runtime dependencies.
- Global installs must not depend on the caller's current working directory. Read-only snapshots come from package data unless `LOVART_REVERSE_ROOT` or user-synced metadata overrides them.
- `scripts/` is limited to a single development entry point, `scripts/lovart.py`.
- Runtime artifacts stay out of git: credentials, captures, downloads, browser profiles, and local env files.
- Unconfirmed reverse surfaces are represented as explicit status APIs, not guessed production wrappers.

## Update Discipline

`ref/lovart_manifest.json` records hashes for the Lovart canvas HTML, static JS list, signer WASM, generator list, generator schema, pricing table, and entitlement shape. `lovart update check` is read-only. `lovart update sync --metadata-only` refreshes metadata snapshots without generating content.

When frontend or signer artifacts change, real generation must be treated as unsafe until signing fixtures are revalidated. When pricing or entitlement hashes change, batch generation must rerun live quote and free checks.
