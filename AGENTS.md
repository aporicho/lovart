# Agent Instructions

Read `README.md` first. It is the single main manual for this project.

This file is only the hard-rule checklist for coding agents.

## Hard Rules

- Prefer the `lovart-mcp` MCP server when available; otherwise call the `lovart` CLI.
- Do not read credentials, captures, browser profiles, or `ref/` snapshots directly.
- Parse stdout as the machine contract. stderr is diagnostics only.
- Do not wrap machine calls with `uv run lovart ...`; `uv` may print non-JSON messages.
- Run `lovart --version` and `lovart self-test` when entering a new environment.
- If `lovart --version` does not match the expected package/commit or lacks current commands, reinstall from the private GitHub repo.
- Do not guess model parameters. Use `lovart config <model>`.
- Do not treat `quote` alone as permission to submit. Real generation needs `dry-run` and the generation gate.
- Do not bypass `auth_missing`, `metadata_stale`, `signer_stale`, `unknown_pricing`, or `credit_risk`.
- Paid single generation requires explicit user budget and `--allow-paid --max-credits N`.
- Paid batch generation requires explicit user budget and `--allow-paid --max-total-credits N`.
- Batch JSONL is user-level: one line per concept/task, with top-level `outputs` for image count.
- Do not manually split one concept into many JSONL rows; let the CLI expand `remote_requests`.
- After a partial batch run, use `lovart jobs resume`, not `lovart jobs run`.
- Do not modify `ref/` unless the user explicitly asks for reverse-maintenance work.

## Role Method References

- Concept design: `docs/concepts/概念设计师.md`
- AIGC prompt design: `docs/concepts/AIGC提示词设计师.md`
- Lovart CLI usage method: `docs/concepts/LovartCLI生成专家.md`

For command fields and JSON shapes, use `docs/agent-contract.md`.
For installation and MCP configuration, use `docs/agent-install.md`.
