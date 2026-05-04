# Reverse Maintainer Workflow

This workflow is for updating reverse evidence, credentials, metadata, and unconfirmed endpoints. Normal agents should prefer `lovart setup` and `lovart generate`.

## 1. Capture

Start a full capture session:

```bash
lovart reverse start
```

This starts mitmproxy, opens an isolated Chrome profile through `http://127.0.0.1:8080`, and navigates to Lovart Canvas. Perform one narrow flow, then press Ctrl-C in the terminal. Capture files stay in `captures/`, which is ignored by git.

For lower-level debugging without opening a browser:

```bash
lovart reverse capture
```

## 2. Extract Auth

After a Lovart request is captured:

```bash
lovart auth extract captures/<lovart-request>.json
lovart setup
```

The status output shows header names only. It must never print token, cookie, email, or account IDs.

## 3. Sync Metadata

Check for online drift:

```bash
lovart update check
lovart update diff
```

Refresh metadata without generation:

```bash
lovart update sync --metadata-only
```

This rewrites generator list/schema, pricing table, and manifest in `ref/`, then runs offline registry/pricing/entitlement checks.

## 4. Validate Generation Safely

Use dry-run before real submission:

```bash
lovart generate openai/gpt-image-2 --body-file request.json --mode auto --dry-run
```

Real submission is allowed only if preflight passes. Paid testing must include `--allow-paid --max-credits N`.

## 5. Replay Evidence

Replay is for reverse validation, not stable API use:

```bash
lovart reverse replay captures/<request>.json
lovart reverse replay captures/<request>.json --submit
```

Only promote behavior into package modules after capture evidence confirms the endpoint, request shape, response shape, and failure states.
