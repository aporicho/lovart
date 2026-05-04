"""Manifest helpers for Lovart reverse snapshots."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from lovart_reverse.io_json import hash_bytes, hash_value, read_json, write_json
from lovart_reverse.paths import MANIFEST_FILE, SIGNER_WASM, WRITABLE_MANIFEST_FILE


MANIFEST_KEYS = (
    "canvas_html_hash",
    "static_js_hash",
    "sentry_release_id",
    "signer_wasm_url",
    "signer_wasm_hash",
    "generator_list_hash",
    "generator_schema_hash",
    "pricing_table_hash",
    "entitlement_shape_hash",
)


def load_manifest() -> dict[str, Any] | None:
    return read_json(MANIFEST_FILE) if MANIFEST_FILE.exists() else None


def save_manifest(snapshot: dict[str, Any]) -> None:
    write_json(WRITABLE_MANIFEST_FILE, snapshot)


def manifest_from_parts(parts: dict[str, Any]) -> dict[str, Any]:
    result = {
        "generated_at": datetime.now(timezone.utc).isoformat(),
        "lovart_canvas_url": "https://www.lovart.ai/canvas",
    }
    result.update({key: parts.get(key) for key in MANIFEST_KEYS})
    result["details"] = parts.get("details", {})
    return result


def local_ref_manifest(generator_list: Any, generator_schema: Any, pricing: Any = None, entitlements: Any = None) -> dict[str, Any]:
    wasm_hash = hash_bytes(SIGNER_WASM.read_bytes()) if SIGNER_WASM.exists() else None
    return manifest_from_parts(
        {
            "signer_wasm_url": SIGNER_WASM.name if SIGNER_WASM.exists() else None,
            "signer_wasm_hash": wasm_hash,
            "generator_list_hash": hash_value(generator_list),
            "generator_schema_hash": hash_value(generator_schema),
            "pricing_table_hash": hash_value(pricing) if pricing is not None else None,
            "entitlement_shape_hash": hash_value(entitlements) if entitlements is not None else None,
            "details": {},
        }
    )
