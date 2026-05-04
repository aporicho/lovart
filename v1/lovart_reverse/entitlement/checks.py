"""Evaluate fast and relaxed zero-credit generation entitlements."""

from __future__ import annotations

import json
import sys
from dataclasses import asdict, dataclass
from typing import Any

from lovart_reverse.http.client import CANVA_BASE, www_session
from lovart_reverse.paths import CAPTURES_DIR
from lovart_reverse.pricing.traits import bucket_rank, has_reference, quality, size_bucket


@dataclass(frozen=True)
class EntitlementResult:
    model: str
    mode: str
    zero_credit: bool
    source: str
    reasons: list[str]
    matched_items: list[dict[str, Any]]
    raw_shape: dict[str, Any] | None = None

    def to_dict(self) -> dict[str, Any]:
        return asdict(self)


def _load_capture(kind: str) -> dict[str, Any] | None:
    marker = "query_fast_unlimited" if kind == "fast" else "query_unlimited"
    for path in sorted(CAPTURES_DIR.glob(f"*{marker}*.json"), reverse=True):
        try:
            capture = json.loads(path.read_text())
            body = capture.get("response_body")
            if isinstance(body, dict):
                return body
        except Exception:
            continue
    return None


def fetch_unlimited(kind: str, live: bool = True) -> tuple[dict[str, Any] | None, str]:
    """Fetch Lovart unlimited-generation entitlement metadata."""

    endpoint = "task/query/fast/unlimited" if kind == "fast" else "task/query/unlimited"
    if live:
        try:
            response = www_session().get(f"{CANVA_BASE}/agent-cashier/{endpoint}", timeout=30)
            response.raise_for_status()
            return response.json(), "live"
        except Exception as exc:
            print(f"warning: live {kind} entitlement fetch failed, falling back to capture: {exc}", file=sys.stderr)
    captured = _load_capture(kind)
    return captured, "capture" if captured else "none"


def _iter_items(payload: Any) -> list[dict[str, Any]]:
    items: list[dict[str, Any]] = []

    def walk(node: Any) -> None:
        if isinstance(node, dict):
            if any(key in node for key in ("alias", "model", "modelName", "extraItem", "supportModels")):
                items.append(node)
            for value in node.values():
                walk(value)
        elif isinstance(node, list):
            for value in node:
                walk(value)

    walk(payload)
    return items


def _aliases(item: dict[str, Any]) -> list[str]:
    values: list[str] = []
    for key in ("alias", "model", "name", "modelName"):
        value = item.get(key)
        if isinstance(value, str):
            values.append(value)
    for key in ("aliases", "alias_list", "supportModels", "models"):
        value = item.get(key)
        if isinstance(value, list):
            values.extend(str(entry) for entry in value if isinstance(entry, str))
    return values


def _item_matches_model(item: dict[str, Any], model: str) -> bool:
    normalized = model.strip("/").lower()
    aliases = [alias.strip("/").lower() for alias in _aliases(item)]
    return normalized in aliases or any(normalized.endswith(alias) or alias.endswith(normalized) for alias in aliases)


def _extra_constraints(item: dict[str, Any]) -> dict[str, Any]:
    extra = item.get("extraItem") or item.get("extra") or item.get("constraints") or {}
    if isinstance(extra, dict):
        return extra
    if isinstance(extra, str):
        tokens = extra.lower().split()
        parsed: dict[str, Any] = {"raw": extra}
        for token in tokens:
            if token in {"low", "medium", "high"}:
                parsed["quality"] = token
            elif token.endswith("k") and token[:-1].isdigit():
                parsed["maxSize"] = token.upper()
        return parsed
    return {}


def _constraint_reasons(item: dict[str, Any], body: dict[str, Any]) -> list[str]:
    reasons: list[str] = []
    extra = _extra_constraints(item)
    q = quality(body)
    bucket = size_bucket(body.get("size") or body.get("resolution"), body)
    if q:
        allowed_quality = extra.get("quality") or extra.get("qualities") or item.get("quality")
        if isinstance(allowed_quality, str) and q != allowed_quality.lower():
            reasons.append(f"quality={q} not covered by entitlement quality={allowed_quality}")
        elif isinstance(allowed_quality, list) and q not in [str(v).lower() for v in allowed_quality]:
            reasons.append(f"quality={q} not covered by entitlement qualities={allowed_quality}")
    max_size = extra.get("maxSize") or extra.get("max_size") or extra.get("resolution")
    if isinstance(max_size, str) and bucket and bucket_rank(bucket) > bucket_rank(max_size):
        reasons.append(f"size_bucket={bucket} exceeds entitlement max={max_size}")
    if has_reference(body) and (extra.get("noReference") or extra.get("no_ref") or "no ref" in json.dumps(item).lower()):
        reasons.append("reference image is not covered by this entitlement item")
    return reasons


def _evaluate_kind(model: str, body: dict[str, Any], kind: str, live: bool) -> EntitlementResult:
    payload, source = fetch_unlimited(kind, live=live)
    reasons: list[str] = []
    if not payload:
        return EntitlementResult(model, kind, False, source, ["no entitlement payload available"], [], None)
    candidates = [item for item in _iter_items(payload) if _item_matches_model(item, model)]
    matched: list[dict[str, Any]] = []
    for item in candidates:
        if item.get("status") not in (None, 1, "1", True):
            reasons.append("entitlement item is not active")
            continue
        item_reasons = _constraint_reasons(item, body)
        if not item_reasons:
            matched.append({key: item.get(key) for key in ("alias", "alias_list", "model", "modelName", "model_display_name", "extraItem", "supportModels") if key in item})
        reasons.extend(item_reasons)
    if matched:
        return EntitlementResult(model, kind, True, source, [], matched, _shape(payload))
    if not candidates:
        reasons.append(f"model {model} is not present in {kind} entitlement aliases")
    return EntitlementResult(model, kind, False, source, reasons, [], _shape(payload))


def _shape(payload: Any) -> dict[str, Any]:
    items = _iter_items(payload)
    aliases = sorted({alias for item in items for alias in _aliases(item)})
    extra_keys = sorted({key for item in items for key in _extra_constraints(item).keys()})
    return {"item_count": len(items), "aliases": aliases, "extra_keys": extra_keys}


def free_check(model: str, body: dict[str, Any], mode: str = "auto", live: bool = True) -> dict[str, Any]:
    """Return zero-credit eligibility for fast, relaxed, or automatic mode."""

    checks: list[EntitlementResult]
    if mode == "auto":
        checks = [_evaluate_kind(model, body, "fast", live), _evaluate_kind(model, body, "relax", live)]
    elif mode in {"fast", "relax"}:
        checks = [_evaluate_kind(model, body, mode, live)]
    else:
        raise ValueError(f"unsupported mode: {mode}")
    selected = next((check for check in checks if check.zero_credit), checks[0])
    return {
        "model": model,
        "requested_mode": mode,
        "selected_mode": selected.mode,
        "zero_credit": selected.zero_credit,
        "checks": [check.to_dict() for check in checks],
    }
