# Capture governance

This repository may contain small, sanitized capture fixtures that document protocol behavior. It must not become the storage location for raw browser, proxy, or production traffic dumps.

## Collection

- Collect raw traffic only in local, ignored directories such as `captures/` or `captures_backup/` while debugging.
- Treat every raw capture as sensitive until it has been reviewed and minimized.
- Prefer purpose-built fixtures that exercise one endpoint or state transition over full-session dumps.

## Redaction

Before a capture is considered for commit, remove or replace:

- Authorization, Cookie, Set-Cookie, session, access token, refresh token, and API key values.
- User identifiers, email addresses, account names, project names, team names, and billing details.
- Request or response bodies that include prompts, generated assets, private project content, or third-party personal data.
- Large binary payloads, base64 media, fonts, images, wasm blobs, and CDN bundles unless they are intentionally tiny test fixtures.

Use stable placeholders such as `<redacted-token>` or `<redacted-user-id>` so tests can assert structure without preserving secrets.

## Repository policy

- New local debug captures under `captures/` and `captures_backup/` are ignored by default.
- Existing tracked capture history is left untouched during this governance phase.
- `scripts/check_captures.sh` blocks new or modified capture files in the checked diff when they exceed the configured size budget or contain secret-like values.
- CI runs the capture check through `scripts/check_architecture.sh`.

If a sanitized capture must be committed, keep it small, document why it is needed in the consuming test or fixture README, and make the test depend only on the minimal redacted fields.

## Intake checklist

1. Start from a local raw capture outside the committed fixture path.
2. Minimize the file to the smallest request/response fields needed by the test.
3. Replace sensitive values with deterministic placeholders.
4. Verify the file is below `MAX_CAPTURE_BYTES` (default: 262144 bytes).
5. Run `bash scripts/check_captures.sh` and `bash scripts/check_architecture.sh` before opening a pull request.
