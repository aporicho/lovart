"""Plan safe generation routes from schema-derived candidates and quotes."""

from __future__ import annotations

import itertools
import json
import re
from dataclasses import dataclass
from typing import Any

from lovart_reverse.config import config_for_model
from lovart_reverse.entitlement import free_check
from lovart_reverse.errors import InputError, SchemaInvalidError
from lovart_reverse.planning.field_roles import MEDIA_INPUT_FIELDS
from lovart_reverse.pricing.quote import quote_or_unknown
from lovart_reverse.pricing.traits import bucket_rank, size_bucket
from lovart_reverse.registry import ModelRecord, load_ref_registry, model_records, validate_body
from lovart_reverse.setup import setup_status

ROUTE_IDS = ("quality_best", "cost_best", "speed_best")
QUALITY_ORDER = ("high", "medium", "standard", "std", "auto", "low")
MODE_QUALITY_ORDER = ("4k", "pro", "master", "quality", "std", "standard", "fast", "auto")
SPEED_MODE_ORDER = ("fast", "turbo", "quick", "std", "standard", "auto", "pro", "4k")
FIDELITY_ORDER = ("high", "auto", "low")
ROUTE_FIELDS = {
    "quality",
    "size",
    "resolution",
    "aspect_ratio",
    "ratio",
    "mode",
    "input_fidelity",
    "duration",
}
QUOTE_ONLY_FREE_DEFAULTS = {"prompt": ""}


@dataclass(frozen=True)
class Candidate:
    model: str
    display_name: str
    mode: str
    body_patch: dict[str, Any]
    request_body: dict[str, Any]
    quote_request_body: dict[str, Any]
    quote_only_defaults: dict[str, Any]
    quality_score: float
    speed_score: float
    constraints: tuple[str, ...]


def plan_for_model(
    model: str | None = None,
    intent: str = "general",
    count: int = 1,
    partial_body: dict[str, Any] | None = None,
    live: bool = True,
    quote_mode: str | None = None,
    candidate_limit: int = 12,
) -> dict[str, Any]:
    """Return three non-submitting route options for an agent to present."""

    if count < 1:
        raise InputError("--count must be at least 1", {"count": count})
    quote_mode = quote_mode or ("live" if live else "offline")
    if quote_mode not in {"live", "auto", "offline"}:
        raise InputError("--quote must be live, auto, or offline", {"quote": quote_mode})
    if candidate_limit < 1:
        raise InputError("--candidate-limit must be at least 1", {"candidate_limit": candidate_limit})

    base_body = dict(partial_body or {})
    snapshot = load_ref_registry()
    records, rejected = _select_records(snapshot, model, intent, base_body, candidate_limit)
    if model and not records:
        raise SchemaInvalidError("model config not found", {"model": model})

    readiness = setup_status(offline=quote_mode == "offline")
    effective_live = _effective_live(quote_mode, readiness)

    candidates: list[Candidate] = []
    model_summaries: list[dict[str, Any]] = []
    for record in records:
        try:
            config = config_for_model(record.model)
        except SchemaInvalidError as exc:
            rejected.append({"model": record.model, "reason": "schema_invalid", "details": exc.details})
            continue
        fields = {field["key"]: field for field in config["fields"]}
        errors = _validate_count(count, fields)
        if errors:
            rejected.append({"model": record.model, "reason": "count_invalid", "details": {"errors": errors}})
            continue
        built = _build_candidates(record, fields, base_body, count)
        candidates.extend(built)
        model_summaries.append(
            {
                "model": record.model,
                "display_name": record.display_name,
                "type": record.type,
                "candidate_count": len(built),
            }
        )
    if not candidates:
        raise SchemaInvalidError("no plannable model candidates", {"model": model, "intent": intent, "rejected": rejected})

    quote_cache: dict[str, dict[str, Any]] = {}
    quality = _select_quality_route(candidates, effective_live, quote_cache)
    cost = _select_cost_route(candidates, effective_live, quote_cache)
    speed = _select_speed_route(candidates, effective_live, quote_cache)
    routes = [
        _route_payload("quality_best", "质量最高路线", quality, quality, effective_live, quote_cache, readiness),
        _route_payload("cost_best", "花钱最少路线", cost, quality, effective_live, quote_cache, readiness),
        _route_payload("speed_best", "速度最快路线", speed, quality, effective_live, quote_cache, readiness),
    ]

    fields = _agent_fields(config_for_model(records[0].model)["fields"]) if len(records) == 1 else {}
    return {
        "model": records[0].model if len(records) == 1 else None,
        "display_name": records[0].display_name if len(records) == 1 else "",
        "intent": intent,
        "planning_scope": "fixed_model" if len(records) == 1 else "all_models",
        "quote_mode": quote_mode,
        "quote_live": effective_live,
        "candidate_limit": candidate_limit,
        "real_generation_enabled": bool(readiness.get("real_generation_enabled")),
        "readiness": _readiness_payload(readiness),
        "candidate_models": {"selected": model_summaries, "rejected": rejected},
        "routes": routes,
        "fields": fields,
        "partial_body": base_body,
        "partial_schema_errors": validate_body(snapshot, records[0].model, base_body) if model and base_body else [],
        "recommended_next_questions": _next_questions(intent, routes, base_body),
    }


def _select_records(
    snapshot: Any,
    model: str | None,
    intent: str,
    body: dict[str, Any],
    candidate_limit: int,
) -> tuple[list[ModelRecord], list[dict[str, Any]]]:
    records = model_records(snapshot)
    rejected: list[dict[str, Any]] = []
    if model:
        chosen = [record for record in records if record.model == model.strip("/")]
        return chosen, rejected

    has_media = any(key in body for key in MEDIA_INPUT_FIELDS)
    selected: list[ModelRecord] = []
    for record in records:
        if intent == "image-concept" and record.type != "image":
            rejected.append({"model": record.model, "reason": "intent_requires_image_model", "type": record.type})
            continue
        if record.type == "image-modify" and not has_media:
            rejected.append({"model": record.model, "reason": "requires_user_media", "type": record.type})
            continue
        if record.type not in {"image", "image-modify"}:
            rejected.append({"model": record.model, "reason": "not_image_generation", "type": record.type})
            continue
        selected.append(record)
        if len(selected) >= candidate_limit:
            break
    return selected, rejected


def _effective_live(quote_mode: str, readiness: dict[str, Any]) -> bool:
    if quote_mode == "offline":
        return False
    if quote_mode == "live":
        return True
    signer_stale = bool((readiness.get("signer") or {}).get("maybe_stale"))
    return bool(readiness.get("real_generation_enabled")) and not signer_stale


def _validate_count(count: int, fields: dict[str, dict[str, Any]]) -> list[str]:
    errors: list[str] = []
    for key in ("n", "count"):
        field = fields.get(key)
        if not field:
            continue
        minimum = field.get("minimum")
        maximum = field.get("maximum")
        if isinstance(minimum, int | float) and count < minimum:
            errors.append(f"{key} below minimum {minimum}")
        if isinstance(maximum, int | float) and count > maximum:
            errors.append(f"{key} above maximum {maximum}")
    return errors


def _build_candidates(
    record: ModelRecord,
    fields: dict[str, dict[str, Any]],
    base_body: dict[str, Any],
    count: int,
) -> list[Candidate]:
    options = _route_options(fields, base_body, count)
    keys = list(options)
    combinations = itertools.product(*(options[key] for key in keys)) if keys else [()]
    candidates: list[Candidate] = []
    seen: set[str] = set()
    for values in combinations:
        desired = dict(zip(keys, values, strict=True)) if keys else {}
        _put_count(desired, fields, count)
        patch = {
            key: value
            for key, value in desired.items()
            if key not in base_body and not fields.get(key, {}).get("free_input") and not fields.get(key, {}).get("media_input")
        }
        body = {**base_body, **patch}
        quote_body, quote_defaults = _quote_body(fields, body)
        key = _canonical_body(record.model, quote_body)
        if key in seen:
            continue
        seen.add(key)
        mode = _mode_for_candidate(record.model, quote_body)
        candidates.append(
            Candidate(
                model=record.model,
                display_name=record.display_name,
                mode=mode,
                body_patch=patch,
                request_body=body,
                quote_request_body=quote_body,
                quote_only_defaults=quote_defaults,
                quality_score=_quality_score(fields, quote_body, record),
                speed_score=_speed_score(fields, quote_body, record),
                constraints=tuple(_candidate_constraints(fields, quote_body)),
            )
        )
    return sorted(candidates, key=lambda item: (-item.quality_score, -item.speed_score, item.model))[:160]


def _route_options(fields: dict[str, dict[str, Any]], body: dict[str, Any], count: int) -> dict[str, list[Any]]:
    options: dict[str, list[Any]] = {}
    for key, field in fields.items():
        if key in body or key in {"n", "count"}:
            continue
        if key not in ROUTE_FIELDS:
            continue
        if field.get("free_input") or field.get("media_input"):
            continue
        values = field.get("values")
        if not isinstance(values, list) or not values:
            continue
        ranked = _ranked_values(key, values, field)
        if key in {"size", "resolution", "aspect_ratio", "ratio"}:
            ranked = [value for value in ranked if str(value).lower() != "auto"] or ranked
        if key == "duration":
            ranked = ranked[:4]
        elif key in {"aspect_ratio", "ratio"}:
            ranked = ranked[:20]
        elif key in {"size", "resolution"}:
            ranked = ranked[:20]
        else:
            ranked = ranked[:6]
        options[key] = ranked
    for key in ("n", "count"):
        if key in fields and key not in body:
            field = fields[key]
            minimum = field.get("minimum")
            maximum = field.get("maximum")
            if isinstance(minimum, int | float) and count < minimum:
                continue
            if isinstance(maximum, int | float) and count > maximum:
                continue
            options[key] = [count]
            break
    return options


def _ranked_values(key: str, values: list[Any], field: dict[str, Any]) -> list[Any]:
    if key == "quality":
        return _ordered_present(values, QUALITY_ORDER)
    if key == "mode":
        return _ordered_present(values, MODE_QUALITY_ORDER)
    if key == "input_fidelity":
        return _ordered_present(values, FIDELITY_ORDER)
    if key in {"size", "resolution", "aspect_ratio", "ratio"}:
        return sorted(values, key=lambda value: _dimension_rank(value, field), reverse=True)
    if key == "duration":
        return sorted(values, key=_duration_rank)
    return list(values)


def _select_quality_route(
    candidates: list[Candidate],
    live: bool,
    quote_cache: dict[str, dict[str, Any]],
) -> Candidate:
    for candidate in sorted(candidates, key=lambda item: (-item.quality_score, _credit_sort(item, live, quote_cache), item.model)):
        if _quote_success(candidate, live, quote_cache) or not live:
            return candidate
    return sorted(candidates, key=lambda item: (-item.quality_score, item.model))[0]


def _select_cost_route(
    candidates: list[Candidate],
    live: bool,
    quote_cache: dict[str, dict[str, Any]],
) -> Candidate:
    ranked = sorted(candidates, key=lambda item: (-item.quality_score, item.model))
    best_known: Candidate | None = None
    best_key: tuple[float, float, str] | None = None
    for candidate in ranked:
        pricing = _pricing(candidate, live, quote_cache)
        if live and not _quote_success(candidate, live, quote_cache):
            continue
        credits = _credits(pricing)
        if _candidate_zero_credit(candidate, pricing, live):
            return candidate
        if credits is None:
            continue
        key = (credits, -candidate.quality_score, candidate.model)
        if best_key is None or key < best_key:
            best_key = key
            best_known = candidate
    return best_known or ranked[0]


def _select_speed_route(
    candidates: list[Candidate],
    live: bool,
    quote_cache: dict[str, dict[str, Any]],
) -> Candidate:
    ranked = sorted(candidates, key=lambda item: (-item.speed_score, _credit_sort(item, live, quote_cache), -item.quality_score, item.model))
    for candidate in ranked:
        if _quote_success(candidate, live, quote_cache) or not live:
            return candidate
    return ranked[0]


def _route_payload(
    route_id: str,
    label: str,
    candidate: Candidate,
    quality_candidate: Candidate,
    live: bool,
    quote_cache: dict[str, dict[str, Any]],
    readiness: dict[str, Any],
) -> dict[str, Any]:
    pricing = _pricing(candidate, live, quote_cache)
    route_mode = "fast" if route_id == "speed_best" else candidate.mode
    entitlement = _safe_free_check(candidate.model, candidate.quote_request_body, route_mode, live)
    credits = _credits(pricing)
    exact = bool(pricing.get("quoted"))
    zero_credit = credits == 0 if exact else (bool(entitlement.get("zero_credit")) or credits == 0)
    route_constraints = list(candidate.constraints)
    if route_id == "quality_best":
        user_message = "质量最高路线：选择 schema 中质量、尺寸、分辨率等字段的最高合法组合。"
    elif route_id == "cost_best":
        user_message = "花钱最少路线：按质量从高到低实时询价，优先选择 0 积分组合。"
    else:
        user_message = "速度最快路线：优先 fast mode、fast variant 或 fast entitlement；不代表实测耗时。"
    if zero_credit:
        route_constraints.append("live quote confirmed zero credits" if exact else "zero-credit entitlement confirmed")
    elif credits is not None:
        route_constraints.append(f"需要 --allow-paid --max-credits {credits}")
    else:
        route_constraints.append("pricing unknown; real generation requires explicit budget review")
    if not readiness.get("real_generation_enabled"):
        route_constraints.extend(str(action) for action in readiness.get("recommended_actions", []))
    warnings = list(pricing.get("warnings") or [])
    if live and not exact:
        warnings.append("live quote failed; route was not selected from exact pricing")
    return {
        "id": route_id,
        "label": label,
        "model": candidate.model,
        "display_name": candidate.display_name,
        "mode": route_mode,
        "body_patch": candidate.body_patch,
        "request_body": candidate.request_body,
        "quote_request_body": candidate.quote_request_body,
        "quote_only_defaults": candidate.quote_only_defaults,
        "quote": {
            "exact": exact,
            "credits": credits,
            "balance": pricing.get("balance"),
            "price_detail": pricing.get("price_detail"),
            "quote_error": pricing.get("quote_error"),
        },
        "zero_credit": zero_credit,
        "requires_paid_confirmation": not zero_credit,
        "real_generation_enabled": bool(readiness.get("real_generation_enabled")),
        "can_submit_without_paid_flags": bool(readiness.get("real_generation_enabled") and zero_credit),
        "constraints": _dedupe(route_constraints),
        "degraded_steps": _degraded_steps(quality_candidate, candidate),
        "quality_score": candidate.quality_score,
        "cost_score": _cost_score(credits),
        "speed_score": candidate.speed_score,
        "user_message": user_message,
        "recommended_next_questions": _route_questions(candidate, not zero_credit),
        "pricing": pricing,
        "entitlement": entitlement,
        "warnings": _dedupe(warnings),
    }


def _pricing(candidate: Candidate, live: bool, quote_cache: dict[str, dict[str, Any]]) -> dict[str, Any]:
    key = _canonical_body(candidate.model, candidate.quote_request_body)
    if key not in quote_cache:
        quote_cache[key] = quote_or_unknown(candidate.model, candidate.quote_request_body, live=live)
    return quote_cache[key]


def _quote_success(candidate: Candidate, live: bool, quote_cache: dict[str, dict[str, Any]]) -> bool:
    if not live:
        return True
    return bool(_pricing(candidate, live, quote_cache).get("quoted"))


def _credit_sort(candidate: Candidate, live: bool, quote_cache: dict[str, dict[str, Any]]) -> float:
    credits = _credits(_pricing(candidate, live, quote_cache))
    return credits if credits is not None else 999999.0


def _credits(pricing: dict[str, Any]) -> float | None:
    value = pricing.get("credits")
    if value is None:
        return None
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _candidate_zero_credit(candidate: Candidate, pricing: dict[str, Any], live: bool) -> bool:
    credits = _credits(pricing)
    if pricing.get("quoted"):
        return credits == 0
    entitlement = _safe_free_check(candidate.model, candidate.quote_request_body, candidate.mode, live)
    return bool(entitlement.get("zero_credit")) or credits == 0


def _cost_score(credits: float | None) -> float:
    return 0 - credits if credits is not None else -999999.0


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


def _agent_fields(fields: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    result: dict[str, dict[str, Any]] = {}
    for field in fields:
        if field.get("visible") or field.get("cost_affecting") or field.get("batch_relevant") or field.get("media_input"):
            result[field["key"]] = {
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
                    "resolution_mapping",
                    "quality_affecting",
                    "cost_affecting",
                    "speed_affecting",
                    "route_role",
                )
                if item in field
            }
    return result


def _next_questions(intent: str, routes: list[dict[str, Any]], body: dict[str, Any]) -> list[str]:
    questions: list[str] = []
    if "prompt" not in body:
        questions.append("请提供概念设计主题或 prompt。" if intent == "image-concept" else "请提供 prompt。")
    media_fields = sorted(
        {
            str(item).split(" ", 1)[0]
            for route in routes
            for item in route.get("constraints", [])
            if isinstance(item, str) and item.endswith(" not provided")
        }
    )
    if media_fields:
        questions.append(f"是否提供参考素材？可用字段：{', '.join(media_fields)}。")
    if any(route["requires_paid_confirmation"] for route in routes):
        questions.append("如果选择需要积分的路线，请提供明确预算上限。")
    return questions


def _route_questions(candidate: Candidate, needs_budget: bool) -> list[str]:
    questions = []
    if "prompt" not in candidate.request_body:
        questions.append("提供 prompt 或概念描述。")
    if needs_budget:
        questions.append("确认预算上限后才能使用付费路线。")
    return questions


def _readiness_payload(readiness: dict[str, Any]) -> dict[str, Any]:
    return {
        "ready": bool(readiness.get("ready")),
        "real_generation_enabled": bool(readiness.get("real_generation_enabled")),
        "recommended_actions": list(readiness.get("recommended_actions") or []),
        "update_status": (readiness.get("update") or {}).get("status"),
        "signer_maybe_stale": bool((readiness.get("signer") or {}).get("maybe_stale")),
    }


def _put_count(patch: dict[str, Any], fields: dict[str, dict[str, Any]], count: int) -> None:
    for key in ("n", "count"):
        if key in fields and key not in patch:
            patch[key] = count
            return


def _quote_body(fields: dict[str, dict[str, Any]], body: dict[str, Any]) -> tuple[dict[str, Any], dict[str, Any]]:
    quote_body = dict(body)
    defaults: dict[str, Any] = {}
    for key, default in QUOTE_ONLY_FREE_DEFAULTS.items():
        if key in fields and key not in quote_body:
            quote_body[key] = default
            defaults[key] = "empty_for_quote_only"
    return quote_body, defaults


def _mode_for_candidate(model: str, body: dict[str, Any]) -> str:
    mode = str(body.get("mode") or "").lower()
    text = f"{model} {mode}".lower()
    if any(token in text for token in ("fast", "turbo", "quick")):
        return "fast"
    return "auto"


def _candidate_constraints(fields: dict[str, dict[str, Any]], body: dict[str, Any]) -> list[str]:
    constraints: list[str] = []
    for key in ("quality", "size", "resolution", "aspect_ratio", "ratio", "mode", "input_fidelity", "duration", "n", "count"):
        if key in body:
            constraints.append(f"{key}={body[key]}")
    for key, field in fields.items():
        if field.get("media_input") and key not in body and field.get("visible"):
            constraints.append(f"{key} not provided")
    return constraints


def _quality_score(fields: dict[str, dict[str, Any]], body: dict[str, Any], record: ModelRecord) -> float:
    score = max(0, 100 - min(record.index, 100)) / 100
    for key, value in body.items():
        field = fields.get(key, {})
        if key == "quality":
            score += 40 - 6 * _ordered_index(value, QUALITY_ORDER)
        elif key == "mode":
            score += 25 - 4 * _ordered_index(value, MODE_QUALITY_ORDER)
        elif key == "input_fidelity":
            score += 16 - 4 * _ordered_index(value, FIDELITY_ORDER)
        elif key in {"size", "resolution", "aspect_ratio", "ratio"}:
            rank, area = _dimension_rank(value, field)
            score += rank * 12 + min(area / 1_000_000, 20)
        elif key == "duration":
            score += min(_duration_rank(value), 20) / 10
    return round(score, 4)


def _speed_score(fields: dict[str, dict[str, Any]], body: dict[str, Any], record: ModelRecord) -> float:
    score = max(0, 100 - min(record.index, 100)) / 100
    text = f"{record.model} {record.display_name}".lower()
    if any(token in text for token in ("fast", "turbo", "quick")):
        score += 50
    mode = str(body.get("mode") or "").lower()
    if mode:
        score += 40 - 5 * _ordered_index(mode, SPEED_MODE_ORDER)
    if "duration" in body:
        score += max(0, 20 - _duration_rank(body["duration"]))
    for key in ("size", "resolution", "aspect_ratio", "ratio"):
        if key in body:
            rank, area = _dimension_rank(body[key], fields.get(key, {}))
            score += max(0, 12 - rank * 2) + max(0, 8 - area / 1_000_000)
    return round(score, 4)


def _degraded_steps(quality: Candidate, candidate: Candidate) -> list[str]:
    if quality.model != candidate.model:
        return [f"model: {quality.model} -> {candidate.model}"]
    steps: list[str] = []
    keys = sorted(set(quality.request_body) | set(candidate.request_body))
    for key in keys:
        if key in QUOTE_ONLY_FREE_DEFAULTS:
            continue
        before = quality.request_body.get(key)
        after = candidate.request_body.get(key)
        if before != after:
            steps.append(f"{key}: {before} -> {after}")
    return steps


def _canonical_body(model: str, body: dict[str, Any]) -> str:
    return model + "\n" + json.dumps(body, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def _ordered_present(values: list[Any], order: tuple[str, ...]) -> list[Any]:
    lower = {str(value).lower(): value for value in values}
    result = [lower[item] for item in order if item in lower]
    result.extend(value for value in values if value not in result)
    return result


def _ordered_index(value: Any, order: tuple[str, ...]) -> int:
    text = str(value).lower()
    for index, item in enumerate(order):
        if text == item:
            return index
    return len(order)


def _dimension_rank(value: Any, field: dict[str, Any]) -> tuple[int, int]:
    text = str(value)
    mapping = field.get("resolution_mapping")
    mapped = mapping.get(text) if isinstance(mapping, dict) else None
    search = f"{text} {mapped or ''}"
    bucket = size_bucket(search, {})
    rank = bucket_rank(bucket) if bucket else -1
    numbers = [int(match) for match in re.findall(r"\d+", search)]
    if len(numbers) >= 2:
        area = max(numbers[index] * numbers[index + 1] for index in range(len(numbers) - 1))
    elif numbers:
        area = numbers[0]
    else:
        area = 0
    return (rank if rank is not None else -1, area)


def _duration_rank(value: Any) -> int:
    text = str(value)
    return int(text) if text.isdigit() else 999999


def _dedupe(values: list[str]) -> list[str]:
    result: list[str] = []
    for value in values:
        if value and value not in result:
            result.append(value)
    return result
