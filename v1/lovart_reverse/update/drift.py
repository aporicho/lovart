"""Read-only online drift detection and metadata sync."""

from __future__ import annotations

import re
import sys
from typing import Any
from urllib.parse import urljoin

import requests

from lovart_reverse.discovery import generator_list, generator_schema
from lovart_reverse.entitlement.checks import fetch_unlimited
from lovart_reverse.io_json import canonical_json, hash_value, read_json, write_json
from lovart_reverse.paths import (
    GENERATOR_LIST_FILE,
    GENERATOR_SCHEMA_FILE,
    PRICING_TABLE_FILE,
    WRITABLE_GENERATOR_LIST_FILE,
    WRITABLE_GENERATOR_SCHEMA_FILE,
    WRITABLE_MANIFEST_FILE,
    WRITABLE_PRICING_TABLE_FILE,
)
from lovart_reverse.pricing.table import fetch_pricing_payload
from lovart_reverse.registry import load_ref_registry, model_records
from lovart_reverse.update.manifest import MANIFEST_KEYS, load_manifest, manifest_from_parts, save_manifest

CANVAS_URL = "https://www.lovart.ai/canvas"


def _safe_get_text(url: str) -> str:
    response = requests.get(url, timeout=30)
    response.raise_for_status()
    return response.text


def _static_js_urls(html: str) -> list[str]:
    pattern = r"(?:https?:)?//[^\"')<> ]+lovart_canvas_online/static/js/[^\"')<> ]+\.js|/?lovart_canvas_online/static/js/[^\"')<> ]+\.js"
    matches = sorted(set(re.findall(pattern, html)))
    return [_absolute_asset_url(match) for match in matches]


def _wasm_urls(text: str) -> list[str]:
    pattern = r"(?:https?:)?//[^\"')<> ]+lovart_canvas_online/static/[^\"')<> ]+\.wasm|/?lovart_canvas_online/static/[^\"')<> ]+\.wasm"
    matches = sorted(set(re.findall(pattern, text)))
    return [_absolute_asset_url(match) for match in matches]


def _absolute_asset_url(value: str) -> str:
    if value.startswith("//"):
        return "https:" + value
    if value.startswith("http"):
        return value
    return urljoin(CANVAS_URL, "/" + value.lstrip("/"))


def _sentry_release(texts: list[str]) -> str | None:
    patterns = [
        r"SENTRY_RELEASE\s*[:=]\s*\{?\s*id\s*[:=]\s*['\"]([^'\"]+)['\"]",
        r"release\s*[:=]\s*['\"]([^'\"]+)['\"]",
    ]
    for text in texts:
        for pattern in patterns:
            match = re.search(pattern, text)
            if match:
                return match.group(1)
    return None


def _entitlement_shape() -> dict[str, Any]:
    fast, fast_source = fetch_unlimited("fast", live=True)
    relax, relax_source = fetch_unlimited("relax", live=True)
    return {"fast": _shape_payload(fast, fast_source), "relax": _shape_payload(relax, relax_source)}


def _shape_payload(payload: Any, source: str) -> dict[str, Any]:
    aliases: set[str] = set()
    extra_keys: set[str] = set()
    item_count = 0

    def walk(node: Any) -> None:
        nonlocal item_count
        if isinstance(node, dict):
            if any(key in node for key in ("alias", "model", "modelName", "extraItem", "supportModels")):
                item_count += 1
                for key in ("alias", "model", "modelName"):
                    value = node.get(key)
                    if isinstance(value, str):
                        aliases.add(value)
                for alias_key in ("supportModels", "alias_list", "aliases", "models"):
                    values = node.get(alias_key)
                    if isinstance(values, list):
                        aliases.update(str(value) for value in values if isinstance(value, str))
                extra = node.get("extraItem")
                if isinstance(extra, dict):
                    extra_keys.update(str(key) for key in extra.keys())
                elif isinstance(extra, str):
                    extra_keys.update(["raw_string"])
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    walk(payload)
    return {"source": source, "item_count": item_count, "aliases": sorted(aliases), "extra_keys": sorted(extra_keys)}


def fetch_online_snapshot() -> tuple[dict[str, Any], dict[str, Any]]:
    html = _safe_get_text(CANVAS_URL)
    js_urls = _static_js_urls(html)
    js_texts: list[str] = []
    wasm_urls: list[str] = []
    for url in js_urls:
        try:
            text = _safe_get_text(url)
            js_texts.append(text)
            wasm_urls.extend(_wasm_urls(text))
        except Exception as exc:
            print(f"warning: failed to fetch static JS {url}: {exc}", file=sys.stderr)
    sentry = _sentry_release(js_texts)
    signer_wasm_url = sorted(set(wasm_urls))[0] if wasm_urls else None
    signer_wasm_hash = None
    if signer_wasm_url:
        try:
            signer_wasm_hash = hash_value(requests.get(signer_wasm_url, timeout=30).content.hex())
        except Exception as exc:
            print(f"warning: failed to fetch signer WASM {signer_wasm_url}: {exc}", file=sys.stderr)
    list_payload = generator_list(live=True)
    schema_payload = generator_schema(live=True)
    pricing_payload = fetch_pricing_payload(live=True)
    entitlement_payload = _entitlement_shape()
    parts = {
        "canvas_html_hash": hash_value(_normalized_canvas_html(html)),
        "static_js_hash": hash_value(js_urls),
        "sentry_release_id": sentry,
        "signer_wasm_url": signer_wasm_url,
        "signer_wasm_hash": signer_wasm_hash,
        "generator_list_hash": _stable_hash(list_payload),
        "generator_schema_hash": _stable_hash(schema_payload),
        "pricing_table_hash": _stable_hash(pricing_payload),
        "entitlement_shape_hash": _stable_hash(entitlement_payload),
        "details": {
            "static_js_urls": js_urls,
            "entitlement_shape": entitlement_payload,
        },
    }
    raw = {
        "generator_list": list_payload,
        "generator_schema": schema_payload,
        "pricing_table": pricing_payload,
        "entitlements": entitlement_payload,
    }
    return manifest_from_parts(parts), raw


def _compare(local: dict[str, Any] | None, online: dict[str, Any]) -> dict[str, bool]:
    if not local:
        return {key: True for key in _change_key_map().values()}
    return {
        "frontend_bundle": local.get("static_js_hash") != online.get("static_js_hash")
        or local.get("sentry_release_id") != online.get("sentry_release_id"),
        "signer_wasm": local.get("signer_wasm_url") != online.get("signer_wasm_url")
        or local.get("signer_wasm_hash") != online.get("signer_wasm_hash"),
        "generator_list": local.get("generator_list_hash") != online.get("generator_list_hash"),
        "generator_schema": local.get("generator_schema_hash") != online.get("generator_schema_hash"),
        "pricing": local.get("pricing_table_hash") != online.get("pricing_table_hash"),
        "entitlements": local.get("entitlement_shape_hash") != online.get("entitlement_shape_hash"),
    }


def _change_key_map() -> dict[str, str]:
    return {
        "static_js_hash": "frontend_bundle",
        "signer_wasm_hash": "signer_wasm",
        "generator_list_hash": "generator_list",
        "generator_schema_hash": "generator_schema",
        "pricing_table_hash": "pricing",
        "entitlement_shape_hash": "entitlements",
    }


def _normalized_canvas_html(html: str) -> dict[str, Any]:
    return {"static_js_urls": _static_js_urls(html)}


def _stable_hash(value: Any) -> str:
    return hash_value(_stable_value(value))


def _stable_value(value: Any) -> Any:
    if isinstance(value, dict):
        return {key: _stable_value(value[key]) for key in sorted(value.keys())}
    if isinstance(value, list):
        normalized = [_stable_value(item) for item in value]
        return sorted(normalized, key=canonical_json)
    return value


def _recommended(changes: dict[str, bool]) -> list[str]:
    actions: list[str] = []
    if any(changes.values()):
        actions.append("run lovart update sync --metadata-only")
    if changes.get("frontend_bundle") or changes.get("signer_wasm"):
        actions.append("rerun signing fixture before real generation")
    if changes.get("pricing"):
        actions.append("review pricing diff before batch generation")
    if changes.get("entitlements"):
        actions.append("rerun free checks before generation")
    if changes.get("generator_schema"):
        actions.append("rebuild and validate model registry")
    return actions


def check_update() -> dict[str, Any]:
    local = load_manifest()
    online, _ = fetch_online_snapshot()
    changes = _compare(local, online)
    return {
        "status": "missing_manifest" if not local else ("stale" if any(changes.values()) else "fresh"),
        "changes": changes,
        "signer_maybe_stale": bool(changes.get("frontend_bundle") or changes.get("signer_wasm")),
        "recommended_actions": _recommended(changes),
        "local_generated_at": local.get("generated_at") if local else None,
        "online_generated_at": online.get("generated_at"),
        "online": {key: online.get(key) for key in MANIFEST_KEYS},
    }


def _model_names(payload: dict[str, Any]) -> set[str]:
    items = payload.get("items")
    if isinstance(items, list):
        return {str(item.get("name")) for item in items if isinstance(item, dict) and item.get("name")}
    paths = payload.get("paths")
    if isinstance(paths, dict):
        return {str(path).strip("/") for path in paths.keys()}
    return set()


def diff_update() -> dict[str, Any]:
    local_manifest = load_manifest()
    online_manifest, online_raw = fetch_online_snapshot()
    local_list = read_json(GENERATOR_LIST_FILE) if GENERATOR_LIST_FILE.exists() else {}
    local_schema = read_json(GENERATOR_SCHEMA_FILE) if GENERATOR_SCHEMA_FILE.exists() else {}
    local_pricing = read_json(PRICING_TABLE_FILE) if PRICING_TABLE_FILE.exists() else None
    local_models = _model_names(local_list)
    online_models = _model_names(online_raw["generator_list"])
    return {
        "manifest_changes": _compare(local_manifest, online_manifest),
        "models": {
            "added": sorted(online_models - local_models),
            "removed": sorted(local_models - online_models),
            "changed_count_hint": int(local_manifest.get("generator_schema_hash") != online_manifest.get("generator_schema_hash"))
            if local_manifest
            else None,
        },
        "pricing_changed": hash_value(local_pricing) != online_manifest.get("pricing_table_hash") if local_pricing is not None else True,
        "schema_paths_changed": sorted(_model_names(online_raw["generator_schema"]) ^ _model_names(local_schema)),
        "online_manifest": {key: online_manifest.get(key) for key in MANIFEST_KEYS},
    }


def sync_metadata() -> dict[str, Any]:
    manifest, raw = fetch_online_snapshot()
    write_json(WRITABLE_GENERATOR_LIST_FILE, raw["generator_list"])
    write_json(WRITABLE_GENERATOR_SCHEMA_FILE, raw["generator_schema"])
    write_json(WRITABLE_PRICING_TABLE_FILE, raw["pricing_table"])
    save_manifest(manifest)
    checks = _post_sync_checks()
    return {
        "written": [
            str(WRITABLE_GENERATOR_LIST_FILE),
            str(WRITABLE_GENERATOR_SCHEMA_FILE),
            str(WRITABLE_PRICING_TABLE_FILE),
            str(WRITABLE_MANIFEST_FILE),
        ],
        "manifest": {key: manifest.get(key) for key in MANIFEST_KEYS},
        "local_checks": checks,
    }


def _post_sync_checks() -> dict[str, Any]:
    from lovart_reverse.entitlement import free_check
    from lovart_reverse.pricing.table import fetch_pricing_payload

    registry_count = len(model_records(load_ref_registry()))
    pricing_payload = fetch_pricing_payload(live=False)
    entitlement = free_check("openai/gpt-image-2", {"prompt": "x", "quality": "low", "size": "1024*1024"}, mode="auto", live=False)
    return {
        "registry_models": registry_count,
        "pricing_table_cached": pricing_payload is not None,
        "entitlement_checked": "zero_credit" in entitlement,
    }
