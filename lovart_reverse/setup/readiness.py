"""Readiness checks for the agent-facing Lovart CLI."""

from __future__ import annotations

from typing import Any

from lovart_reverse.auth.store import status as auth_status
from lovart_reverse.paths import (
    CAPTURES_DIR,
    DOWNLOADS_DIR,
    GENERATOR_LIST_FILE,
    GENERATOR_SCHEMA_FILE,
    MANIFEST_FILE,
    PRICING_TABLE_FILE,
    SIGNATURE_JS,
    SIGNER_WASM,
)
from lovart_reverse.update import check_update
from lovart_reverse.update.manifest import load_manifest


def offline_update_status() -> dict[str, Any]:
    manifest = load_manifest()
    return {
        "status": "offline_cached" if manifest else "missing_manifest",
        "changes": {},
        "signer_maybe_stale": False if manifest else True,
        "recommended_actions": [] if manifest else ["run lovart update sync --metadata-only"],
        "local_generated_at": manifest.get("generated_at") if manifest else None,
    }


def _safe_update_status(offline: bool) -> dict[str, Any]:
    if offline:
        return offline_update_status()
    try:
        return check_update()
    except Exception as exc:
        return {
            "status": "error",
            "changes": {},
            "signer_maybe_stale": True,
            "recommended_actions": ["rerun lovart setup when network is available", "run lovart update check"],
            "error": {"type": exc.__class__.__name__, "message": str(exc)},
        }


def setup_status(offline: bool = False) -> dict[str, Any]:
    auth = auth_status()
    update = _safe_update_status(offline=offline)
    refs = {
        "manifest": {"path": str(MANIFEST_FILE), "exists": MANIFEST_FILE.exists()},
        "generator_list": {"path": str(GENERATOR_LIST_FILE), "exists": GENERATOR_LIST_FILE.exists()},
        "generator_schema": {"path": str(GENERATOR_SCHEMA_FILE), "exists": GENERATOR_SCHEMA_FILE.exists()},
        "pricing_table": {"path": str(PRICING_TABLE_FILE), "exists": PRICING_TABLE_FILE.exists()},
    }
    paths = {
        "captures": {"path": str(CAPTURES_DIR), "exists": CAPTURES_DIR.exists(), "git_ignored": True},
        "downloads": {"path": str(DOWNLOADS_DIR), "exists": DOWNLOADS_DIR.exists(), "git_ignored": True},
    }
    signer = {
        "signature_js_exists": SIGNATURE_JS.exists(),
        "wasm_exists": SIGNER_WASM.exists(),
        "maybe_stale": bool(update.get("signer_maybe_stale")),
    }
    auth_ready = bool(auth.get("exists") and auth.get("header_names"))
    refs_ready = all(item["exists"] for item in refs.values())
    update_ready = update.get("status") in {"fresh", "offline_cached"}
    signer_ready = signer["signature_js_exists"] and signer["wasm_exists"] and not signer["maybe_stale"]
    recommended_actions: list[str] = []
    if not auth_ready:
        recommended_actions.extend(["run lovart reverse capture", "run lovart auth extract <capture.json>"])
    if not refs_ready or update.get("status") in {"missing_manifest", "stale"}:
        recommended_actions.append("run lovart update sync --metadata-only")
    if signer["maybe_stale"]:
        recommended_actions.append("rerun signing fixture before real generation")
    recommended_actions.extend(str(action) for action in update.get("recommended_actions", []) if action not in recommended_actions)
    return {
        "ready": auth_ready and refs_ready and update_ready and signer_ready,
        "real_generation_enabled": auth_ready and refs_ready and update_ready and signer_ready,
        "auth": auth,
        "refs": refs,
        "paths": paths,
        "signer": signer,
        "update": update,
        "recommended_actions": recommended_actions,
        "allowed_when_not_ready": ["models", "schema", "quote", "free", "generate --dry-run", "update", "reverse"],
    }
