"""Plan safe generation routes before asking users for model settings."""

from __future__ import annotations

import re
from typing import Any

from lovart_reverse.config import config_for_model
from lovart_reverse.entitlement import free_check
from lovart_reverse.errors import InputError
from lovart_reverse.pricing.estimator import estimate
from lovart_reverse.pricing.table import PriceRow, fetch_pricing_rows
from lovart_reverse.pricing.traits import bucket_rank, size_bucket
from lovart_reverse.registry import load_ref_registry, validate_body
from lovart_reverse.setup import setup_status

QUALITY_ORDER = ("high", "medium", "low", "auto")
MODE_QUALITY_ORDER = ("4k", "pro", "master", "std", "fast", "auto")
SMALL_SIZE_PREFERENCES = ("1024*1024", "512", "1K", "480p", "720p")
ROUTE_IDS = ("quality_best", "cost_best", "speed_best")


def plan_for_model(
    model: str,
    intent: str = "general",
    count: int = 1,
    partial_body: dict[str, Any] | None = None,
    live: bool = True,
) -> dict[str, Any]:
    """Return three non-submitting route options for an agent to present."""

    if count < 1:
        raise InputError("--count must be at least 1", {"count": count})
    config = config_for_model(model)
    fields = {field["key"]: field for field in config["fields"]}
    body = dict(partial_body or {})
    _validate_count(count, fields)
    rows = fetch_pricing_rows(live=live)
    readiness = setup_status(offline=not live)
    routes = [
        _route(
            model,
            "quality_best",
            "质量最高路线",
            "auto",
            body,
            _quality_patch(fields, count),
            rows,
            live,
            readiness,
            ["选择 schema 中最高质量/最大尺寸；如需扣积分，必须由用户确认预算。"],
            "质量最高，但可能消耗积分或遇到未知价格。",
        ),
        _cost_route(model, fields, body, count, rows, live, readiness),
        _speed_route(model, fields, body, count, rows, live, readiness),
    ]
    return {
        "model": config["model"],
        "display_name": config.get("display_name", ""),
        "intent": intent,
        "real_generation_enabled": bool(readiness.get("real_generation_enabled")),
        "readiness": {
            "ready": bool(readiness.get("ready")),
            "real_generation_enabled": bool(readiness.get("real_generation_enabled")),
            "recommended_actions": list(readiness.get("recommended_actions") or []),
            "update_status": (readiness.get("update") or {}).get("status"),
            "signer_maybe_stale": bool((readiness.get("signer") or {}).get("maybe_stale")),
        },
        "routes": routes,
        "fields": _agent_fields(fields),
        "partial_body": body,
        "partial_schema_errors": validate_body(load_ref_registry(), model, body) if body else [],
        "recommended_next_questions": _next_questions(intent, fields, body, routes),
    }


def _validate_count(count: int, fields: dict[str, dict[str, Any]]) -> None:
    for key in ("n", "count"):
        field = fields.get(key)
        if not field:
            continue
        minimum = field.get("minimum")
        maximum = field.get("maximum")
        if isinstance(minimum, int | float) and count < minimum:
            raise InputError("--count is below schema minimum", {"count": count, "field": key, "minimum": minimum})
        if isinstance(maximum, int | float) and count > maximum:
            raise InputError("--count exceeds schema maximum", {"count": count, "field": key, "maximum": maximum})


def _quality_patch(fields: dict[str, dict[str, Any]], count: int) -> dict[str, Any]:
    patch: dict[str, Any] = {}
    _put_if_legal(patch, fields, "quality", _first_by_order(_values(fields, "quality"), QUALITY_ORDER))
    _put_if_legal(patch, fields, "mode", _first_by_order(_values(fields, "mode"), MODE_QUALITY_ORDER))
    _put_if_legal(patch, fields, "size", _largest_dimension(_values(fields, "size")))
    _put_if_legal(patch, fields, "resolution", _largest_dimension(_values(fields, "resolution")))
    _put_count(patch, fields, count)
    return patch


def _cost_route(
    model: str,
    fields: dict[str, dict[str, Any]],
    base_body: dict[str, Any],
    count: int,
    rows: list[PriceRow],
    live: bool,
    readiness: dict[str, Any],
) -> dict[str, Any]:
    constraints = ["优先零积分；不使用参考图；在可确认免费约束内尽量保质量。"]
    for patch in _cost_candidates(fields, count):
        route = _route(
            model,
            "cost_best",
            "花钱最少路线",
            "auto",
            base_body,
            patch,
            rows,
            live,
            readiness,
            constraints,
            "优先零积分，在约束内尽量保质量。",
        )
        if route["zero_credit"]:
            return route
    return _route(
        model,
        "cost_best",
        "花钱最少路线",
        "auto",
        base_body,
        _low_cost_patch(fields, count),
        rows,
        live,
        readiness,
        constraints + ["未能确认零积分，真实提交需要预算确认。"],
        "未能确认零积分；这是最低成本候选路线，提交前需要预算确认。",
    )


def _speed_route(
    model: str,
    fields: dict[str, dict[str, Any]],
    base_body: dict[str, Any],
    count: int,
    rows: list[PriceRow],
    live: bool,
    readiness: dict[str, Any],
) -> dict[str, Any]:
    constraints = ["优先 fast mode；如果 fast 权益有质量/尺寸限制，会反映在 body_patch 中。"]
    for patch in _speed_candidates(fields, count):
        route = _route(
            model,
            "speed_best",
            "速度最快路线",
            "fast",
            base_body,
            patch,
            rows,
            live,
            readiness,
            constraints,
            "优先快速处理。",
        )
        if route["zero_credit"]:
            return route
    route = _route(
        model,
        "speed_best",
        "速度最快路线",
        "fast",
        base_body,
        _low_cost_patch(fields, count),
        rows,
        live,
        readiness,
        constraints + ["fast 零积分未命中；提交前需要预算确认或改用 dry-run 检查。"],
        "优先快速处理，但当前未能确认 fast 零积分。",
    )
    route["warnings"].append("fast entitlement was not confirmed for the selected route")
    return route


def _cost_candidates(fields: dict[str, dict[str, Any]], count: int) -> list[dict[str, Any]]:
    qualities = _ordered_present(_values(fields, "quality"), ("medium", "low", "auto", "high"))
    if not qualities:
        qualities = [None]
    sizes = [_smallest_dimension(_values(fields, "size"))]
    resolutions = [_smallest_dimension(_values(fields, "resolution"))]
    return [_candidate_patch(fields, count, quality=quality, size=size, resolution=resolution) for quality in qualities for size in sizes for resolution in resolutions]


def _speed_candidates(fields: dict[str, dict[str, Any]], count: int) -> list[dict[str, Any]]:
    qualities = _ordered_present(_values(fields, "quality"), ("medium", "low", "auto", "high"))
    if not qualities:
        qualities = [None]
    return [
        _candidate_patch(
            fields,
            count,
            quality=quality,
            mode=_first_by_order(_values(fields, "mode"), ("fast", "std", "pro", "4k")),
            size=_smallest_dimension(_values(fields, "size")),
            resolution=_smallest_dimension(_values(fields, "resolution")),
            duration=_smallest_duration(_values(fields, "duration")),
        )
        for quality in qualities
    ]


def _low_cost_patch(fields: dict[str, dict[str, Any]], count: int) -> dict[str, Any]:
    return _candidate_patch(
        fields,
        count,
        quality=_first_by_order(_values(fields, "quality"), ("low", "medium", "auto", "high")),
        mode=_first_by_order(_values(fields, "mode"), ("fast", "std", "pro", "4k")),
        size=_smallest_dimension(_values(fields, "size")),
        resolution=_smallest_dimension(_values(fields, "resolution")),
        duration=_smallest_duration(_values(fields, "duration")),
    )


def _candidate_patch(
    fields: dict[str, dict[str, Any]],
    count: int,
    quality: Any = None,
    mode: Any = None,
    size: Any = None,
    resolution: Any = None,
    duration: Any = None,
) -> dict[str, Any]:
    patch: dict[str, Any] = {}
    _put_if_legal(patch, fields, "quality", quality)
    _put_if_legal(patch, fields, "mode", mode)
    _put_if_legal(patch, fields, "size", size)
    _put_if_legal(patch, fields, "resolution", resolution)
    _put_if_legal(patch, fields, "duration", duration)
    _put_count(patch, fields, count)
    return patch


def _route(
    model: str,
    route_id: str,
    label: str,
    mode: str,
    base_body: dict[str, Any],
    desired_patch: dict[str, Any],
    rows: list[PriceRow],
    live: bool,
    readiness: dict[str, Any],
    constraints: list[str],
    user_message: str,
) -> dict[str, Any]:
    body_patch = {key: value for key, value in desired_patch.items() if key not in base_body}
    body = {**base_body, **body_patch}
    pricing = estimate(model, body, rows)
    entitlement = _safe_free_check(model, body, mode, live)
    zero_credit = bool(entitlement.get("zero_credit"))
    estimated = bool(pricing.get("estimated"))
    estimated_credits = 0 if zero_credit else pricing.get("credits")
    warnings = list(pricing.get("warnings") or [])
    route_constraints = list(constraints)
    if zero_credit:
        route_constraints.append(f"zero-credit entitlement confirmed via {entitlement.get('selected_mode') or mode}")
    elif estimated:
        route_constraints.append(f"需要 --allow-paid --max-credits {estimated_credits}")
    else:
        route_constraints.append("pricing unknown; real generation will require explicit budget review")
    if not readiness.get("real_generation_enabled"):
        route_constraints.extend(str(action) for action in readiness.get("recommended_actions", []))
    return {
        "id": route_id,
        "label": label,
        "mode": mode,
        "body_patch": body_patch,
        "request_body": body,
        "estimated": estimated or zero_credit,
        "estimated_credits": estimated_credits if estimated_credits is not None else None,
        "zero_credit": zero_credit,
        "requires_paid_confirmation": not zero_credit,
        "real_generation_enabled": bool(readiness.get("real_generation_enabled")),
        "can_submit_without_paid_flags": bool(readiness.get("real_generation_enabled") and zero_credit),
        "constraints": _dedupe(route_constraints),
        "user_message": user_message,
        "pricing": pricing,
        "entitlement": entitlement,
        "warnings": warnings,
    }


def _safe_free_check(model: str, body: dict[str, Any], mode: str, live: bool) -> dict[str, Any]:
    try:
        return free_check(model, body, mode=mode, live=live)
    except Exception as exc:
        return {
            "model": model,
            "requested_mode": mode,
            "selected_mode": mode,
            "zero_credit": False,
            "checks": [],
            "error": {"type": exc.__class__.__name__, "message": str(exc)},
        }


def _agent_fields(fields: dict[str, dict[str, Any]]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for key, field in fields.items():
        if field.get("visible") or field.get("cost_affecting") or field.get("batch_relevant") or field.get("media_input"):
            result[key] = {
                item: field[item]
                for item in (
                    "key",
                    "type",
                    "required",
                    "visible",
                    "default",
                    "enumerable",
                    "values",
                    "minimum",
                    "maximum",
                    "minItems",
                    "maxItems",
                    "source",
                )
                if item in field
            }
    return result


def _next_questions(intent: str, fields: dict[str, dict[str, Any]], body: dict[str, Any], routes: list[dict[str, Any]]) -> list[str]:
    questions: list[str] = []
    if "prompt" in fields and "prompt" not in body:
        questions.append("请提供概念设计主题或 prompt。" if intent == "image-concept" else "请提供 prompt。")
    media_fields = [key for key, field in fields.items() if field.get("media_input") and key not in body and field.get("visible")]
    if media_fields:
        questions.append(f"是否提供参考素材？可用字段：{', '.join(media_fields)}。")
    if any(route["requires_paid_confirmation"] for route in routes):
        questions.append("如果选择需要积分的路线，请提供明确预算上限。")
    return questions


def _put_count(patch: dict[str, Any], fields: dict[str, dict[str, Any]], count: int) -> None:
    for key in ("n", "count"):
        if key in fields:
            patch[key] = count
            return


def _put_if_legal(patch: dict[str, Any], fields: dict[str, dict[str, Any]], key: str, value: Any) -> None:
    if value is None or key not in fields:
        return
    values = fields[key].get("values")
    if isinstance(values, list) and value not in values:
        return
    patch[key] = value


def _values(fields: dict[str, dict[str, Any]], key: str) -> list[Any]:
    values = fields.get(key, {}).get("values")
    return values if isinstance(values, list) else []


def _ordered_present(values: list[Any], order: tuple[str, ...]) -> list[Any]:
    lower = {str(value).lower(): value for value in values}
    result = [lower[item] for item in order if item in lower]
    result.extend(value for value in values if value not in result)
    return result


def _first_by_order(values: list[Any], order: tuple[str, ...]) -> Any:
    ordered = _ordered_present(values, order)
    return ordered[0] if ordered else None


def _largest_dimension(values: list[Any]) -> Any:
    concrete = [value for value in values if str(value).lower() != "auto"]
    return max(concrete, key=_dimension_rank) if concrete else (values[0] if values else None)


def _smallest_dimension(values: list[Any]) -> Any:
    concrete = [value for value in values if str(value).lower() != "auto"]
    for preferred in SMALL_SIZE_PREFERENCES:
        for value in concrete:
            if str(value).lower() == preferred.lower():
                return value
    return min(concrete, key=_dimension_rank) if concrete else (values[0] if values else None)


def _dimension_rank(value: Any) -> tuple[int, int]:
    text = str(value)
    bucket = size_bucket(text, {})
    rank = bucket_rank(bucket) if bucket else -1
    numbers = [int(match) for match in re.findall(r"\d+", text)]
    area = numbers[0] * numbers[1] if len(numbers) >= 2 else (numbers[0] if numbers else 0)
    return (rank if rank is not None else -1, area)


def _smallest_duration(values: list[Any]) -> Any:
    if not values:
        return None
    return min(values, key=lambda value: int(value) if str(value).isdigit() else 999999)


def _dedupe(values: list[str]) -> list[str]:
    result: list[str] = []
    for value in values:
        if value and value not in result:
            result.append(value)
    return result
