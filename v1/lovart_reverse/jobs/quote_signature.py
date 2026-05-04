"""Strict quote equivalence keys for batch pricing reuse."""

from __future__ import annotations

import json
from functools import lru_cache
from hashlib import sha256
from typing import Any

from lovart_reverse.config import config_for_model
from lovart_reverse.planning.field_roles import MEDIA_INPUT_FIELDS

COST_SIGNATURE_VERSION = "quote-cost-v1"
ALWAYS_EXCLUDED_FIELDS = {"prompt", "negative_prompt", "job_id", "request_id", "title"}


def cost_signature_for_request(model: str, mode: str, body: dict[str, Any]) -> dict[str, Any]:
    """Return a strict pricing-equivalence signature payload and hash."""

    fields = _fields_for_model(model)
    components: dict[str, Any] = {
        "version": COST_SIGNATURE_VERSION,
        "model": model.strip("/"),
        "mode": mode,
        "body": {},
        "media_inputs": {},
    }
    for key in sorted(body):
        value = body[key]
        if _is_excluded(key, fields):
            continue
        if key in MEDIA_INPUT_FIELDS:
            components["media_inputs"][key] = _media_signature(value)
            continue
        components["body"][key] = value
    canonical = json.dumps(components, ensure_ascii=False, sort_keys=True, separators=(",", ":"))
    return {
        "signature": sha256(canonical.encode("utf-8")).hexdigest(),
        "version": COST_SIGNATURE_VERSION,
        "basis": components,
    }


@lru_cache(maxsize=128)
def _fields_for_model(model: str) -> dict[str, dict[str, Any]]:
    try:
        config = config_for_model(model, include_all=True)
    except Exception:
        return {}
    fields = config.get("fields")
    if not isinstance(fields, list):
        return {}
    return {str(field.get("key")): field for field in fields if isinstance(field, dict) and field.get("key")}


def _is_excluded(key: str, fields: dict[str, dict[str, Any]]) -> bool:
    if key in ALWAYS_EXCLUDED_FIELDS:
        return True
    field = fields.get(key)
    return bool(field and field.get("route_role") == "format_only")


def _media_signature(value: Any) -> dict[str, Any]:
    if value in (None, "", [], {}):
        return {"kind": "empty", "count": 0}
    if isinstance(value, list):
        return {"kind": "list", "count": len(value)}
    if isinstance(value, dict):
        return {"kind": "object", "count": 1}
    return {"kind": "single", "count": 1}
