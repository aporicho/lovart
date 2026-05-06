"""Derive exhaustive user-configurable fields from Lovart OpenAPI schemas."""

from __future__ import annotations

from typing import Any

from lovart_reverse.errors import SchemaInvalidError
from lovart_reverse.paths import CAPTURES_DIR, DOWNLOADS_DIR, PACKAGE_REF_DIR, REF_DIR, ROOT, RUNS_DIR
from lovart_reverse.config.field_roles import (
    BATCH_RELEVANT_FIELDS,
    COST_AFFECTING_FIELDS,
    MEDIA_INPUT_FIELDS,
    classify_field,
)
from lovart_reverse.registry import load_ref_registry, model_records, request_schema
from lovart_reverse.registry.snapshot import RegistrySnapshot

FREE_BATCH_DEFAULTS = {
    "quality": "low",
    "size": "1024*1024",
    "resolution": "1K",
    "n": 1,
    "count": 1,
    "max_images": 1,
}


def global_config() -> dict[str, Any]:
    return {
        "paths": {
            "root": str(ROOT),
            "ref": str(REF_DIR),
            "package_ref": str(PACKAGE_REF_DIR),
            "captures": str(CAPTURES_DIR),
            "downloads": str(DOWNLOADS_DIR),
            "runs": str(RUNS_DIR),
            "root_env": "LOVART_REVERSE_ROOT",
            "home_env": "LOVART_REVERSE_HOME",
        },
        "generation_flags": [
            "--body-file",
            "--body",
            "--mode",
            "--dry-run",
            "--wait",
            "--download",
            "--allow-paid",
            "--max-credits",
        ],
        "update_flags": ["update check", "update diff", "update sync --metadata-only"],
        "paid_policy": {
            "default": "zero_credit_only",
            "paid_requires": ["--allow-paid", "--max-credits"],
            "unknown_pricing": "refuse",
        },
        "agent_rules": [
            "Call lovart config <model> before presenting model-specific choices.",
            "Call lovart quote <model> before claiming exact credit cost.",
            "Use only values returned in field.values for enumerable fields.",
            "Do not infer legal values from descriptions.",
            "Ask the user for enumerable=false fields unless the value is already in context.",
        ],
    }


def config_for_model(model: str, include_all: bool = False, example: str | None = None) -> dict[str, Any]:
    snapshot = load_ref_registry()
    schema = request_schema(snapshot, model)
    if not schema:
        raise SchemaInvalidError("model config not found", {"model": model})
    records = {record.model: record for record in model_records(snapshot)}
    record = records.get(model.strip("/"))
    fields = [_field_config(snapshot, schema, key, spec) for key, spec in _properties(schema).items()]
    if not include_all:
        fields = sorted(fields, key=lambda field: (not field["visible"], field["key"]))
    examples = _examples(fields)
    data: dict[str, Any] = {
        "model": model.strip("/"),
        "display_name": record.display_name if record else "",
        "type": record.type if record else _capabilities(schema).get("model_type", ""),
        "request_schema": record.request_schema if record else "",
        "modes": _modes(schema),
        "fields": fields,
        "field_count": len(fields),
        "visible_fields": [field["key"] for field in fields if field["visible"]],
        "examples": examples,
        "no_guess_policy": {
            "enumerable_fields": "values is the complete legal set from schema",
            "free_input_fields": "enumerable=false values must be supplied by the user/context",
            "description_values": "never treat description-only values as legal values",
        },
    }
    if example:
        if example not in examples:
            raise SchemaInvalidError("unknown config example", {"model": model, "example": example, "available": sorted(examples)})
        data["example"] = {"name": example, "body": examples[example]}
    return data


def _components(snapshot: RegistrySnapshot) -> dict[str, Any]:
    components = snapshot.schema.get("components", {})
    schemas = components.get("schemas", {}) if isinstance(components, dict) else {}
    return schemas if isinstance(schemas, dict) else {}


def _properties(schema: dict[str, Any]) -> dict[str, Any]:
    properties = schema.get("properties", {})
    return properties if isinstance(properties, dict) else {}


def _capabilities(schema: dict[str, Any]) -> dict[str, Any]:
    caps = schema.get("x-capabilities", {})
    return caps if isinstance(caps, dict) else {}


def _modes(schema: dict[str, Any]) -> list[dict[str, Any]]:
    modes = _capabilities(schema).get("modes", [])
    return modes if isinstance(modes, list) else []


def _required_fields(schema: dict[str, Any]) -> set[str]:
    required = schema.get("required", [])
    fields = {field for field in required if isinstance(field, str)} if isinstance(required, list) else set()
    modes = _modes(schema)
    if modes:
        mode_required: list[set[str]] = []
        for mode in modes:
            if isinstance(mode, dict):
                values = mode.get("required_fields", [])
                mode_required.append({field for field in values if isinstance(field, str)} if isinstance(values, list) else set())
        if mode_required:
            fields.update(set.intersection(*mode_required))
    return fields


def _required_in_modes(schema: dict[str, Any], key: str) -> list[str]:
    result: list[str] = []
    for mode in _modes(schema):
        if not isinstance(mode, dict):
            continue
        required = mode.get("required_fields", [])
        if isinstance(required, list) and key in required:
            result.append(str(mode.get("mode") or ""))
    return result


def _field_config(snapshot: RegistrySnapshot, schema: dict[str, Any], key: str, spec: Any) -> dict[str, Any]:
    spec = spec if isinstance(spec, dict) else {}
    resolved, ref_name = _resolved_schema(snapshot, spec)
    values, source = _values_and_source(spec, resolved, ref_name)
    field_type = _field_type(spec, resolved)
    visible_fields = schema.get("x-ui-visible-fields", [])
    visible = key in visible_fields if isinstance(visible_fields, list) else False
    required = key in _required_fields(schema)
    field = {
        "key": key,
        "type": field_type,
        "required": required,
        "required_in_modes": _required_in_modes(schema, key),
        "visible": visible,
        "advanced": not visible,
        "default": spec.get("default", resolved.get("default")),
        "description": str(spec.get("description") or resolved.get("description") or ""),
        "source": source,
        "enumerable": values is not None,
        "values": values,
        "cost_affecting": key in COST_AFFECTING_FIELDS,
        "batch_relevant": key in BATCH_RELEVANT_FIELDS,
        "media_input": key in MEDIA_INPUT_FIELDS,
    }
    if isinstance(spec.get("x-resolution-mapping"), dict):
        field["resolution_mapping"] = dict(spec["x-resolution-mapping"])
    elif isinstance(resolved.get("x-resolution-mapping"), dict):
        field["resolution_mapping"] = dict(resolved["x-resolution-mapping"])
    field.update(classify_field(key, field))
    _copy_if_present(field, spec, resolved, "minimum")
    _copy_if_present(field, spec, resolved, "maximum")
    _copy_if_present(field, spec, resolved, "minLength")
    _copy_if_present(field, spec, resolved, "maxLength")
    _copy_if_present(field, spec, resolved, "minItems")
    _copy_if_present(field, spec, resolved, "maxItems")
    if field_type == "array":
        items = spec.get("items") if isinstance(spec.get("items"), dict) else resolved.get("items")
        field["items_type"] = _array_item_type(snapshot, items if isinstance(items, dict) else {})
    if key in {"quality", "size", "resolution"}:
        hint = _zero_credit_hint(key, field)
        if hint is not None:
            field["zero_credit_hint"] = hint
    return field


def _resolved_schema(snapshot: RegistrySnapshot, spec: dict[str, Any]) -> tuple[dict[str, Any], str | None]:
    ref = spec.get("$ref")
    if not isinstance(ref, str):
        return {}, None
    name = ref.rsplit("/", 1)[-1]
    resolved = _components(snapshot).get(name, {})
    return (resolved if isinstance(resolved, dict) else {}), name


def _values_and_source(spec: dict[str, Any], resolved: dict[str, Any], ref_name: str | None) -> tuple[list[Any] | None, str]:
    if isinstance(spec.get("enum"), list):
        return list(spec["enum"]), "schema.inline_enum"
    if isinstance(resolved.get("enum"), list) and ref_name:
        return list(resolved["enum"]), f"schema.ref_enum:{ref_name}"
    field_type = spec.get("type") or resolved.get("type")
    if field_type == "boolean":
        return [True, False], "schema.boolean"
    if field_type in {"integer", "number"} and ("minimum" in spec or "maximum" in spec or "minimum" in resolved or "maximum" in resolved):
        return None, "schema.range"
    if field_type == "array":
        return None, "schema.array_limits"
    return None, "schema.free_input"


def _field_type(spec: dict[str, Any], resolved: dict[str, Any]) -> str:
    value = spec.get("type") or resolved.get("type")
    if isinstance(value, str):
        return value
    if "$ref" in spec:
        if isinstance(resolved.get("enum"), list):
            return str(resolved.get("type") or "string")
        if resolved:
            return str(resolved.get("type") or "object")
    return "unknown"


def _copy_if_present(field: dict[str, Any], spec: dict[str, Any], resolved: dict[str, Any], key: str) -> None:
    if key in spec:
        field[key] = spec[key]
    elif key in resolved:
        field[key] = resolved[key]


def _array_item_type(snapshot: RegistrySnapshot, items: dict[str, Any]) -> str:
    if not items:
        return "unknown"
    if isinstance(items.get("type"), str):
        return str(items["type"])
    resolved, _ = _resolved_schema(snapshot, items)
    return str(resolved.get("type") or "object") if resolved else "unknown"


def _zero_credit_hint(key: str, field: dict[str, Any]) -> str | None:
    values = field.get("values")
    if not isinstance(values, list):
        return None
    if key == "quality" and "low" in values:
        return "Prefer low for fast zero-credit eligibility when available."
    if key in {"size", "resolution"}:
        for candidate in ("1024*1024", "1K", "512"):
            if candidate in values:
                return f"Prefer {candidate} for batch zero-credit checks when acceptable."
    return None


def _examples(fields: list[dict[str, Any]]) -> dict[str, dict[str, Any]]:
    defaults: dict[str, Any] = {}
    zero_credit: dict[str, Any] = {}
    for field in fields:
        key = field["key"]
        if field.get("default") is not None:
            defaults[key] = field["default"]
        elif field.get("required") and not field.get("enumerable"):
            defaults[key] = _placeholder_for(field)
        if key in FREE_BATCH_DEFAULTS:
            value = FREE_BATCH_DEFAULTS[key]
            values = field.get("values")
            if not isinstance(values, list) or value in values:
                zero_credit[key] = value
        elif field.get("default") is not None and key in {"prompt"}:
            zero_credit[key] = field["default"]
        elif key == "prompt":
            zero_credit[key] = "Describe the image to generate."
    return {
        "defaults": defaults,
        "zero_credit": {**defaults, **zero_credit},
    }


def _placeholder_for(field: dict[str, Any]) -> str:
    key = str(field.get("key"))
    if key in MEDIA_INPUT_FIELDS:
        return f"<{key}_url>"
    return f"<{key}>"
