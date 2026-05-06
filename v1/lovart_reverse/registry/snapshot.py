"""Build a model registry from Lovart generator list and schema."""

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path
from typing import Any

from lovart_reverse.io_json import read_json
from lovart_reverse.paths import GENERATOR_LIST_FILE, GENERATOR_SCHEMA_FILE
from lovart_reverse.registry.models import ModelRecord


@dataclass(frozen=True)
class RegistrySnapshot:
    schema: dict[str, Any]
    listing: dict[str, Any]


def _ref_name(ref: str) -> str:
    return ref.rsplit("/", 1)[-1]


def _components(schema: dict[str, Any]) -> dict[str, Any]:
    components = schema.get("components", {})
    schemas = components.get("schemas", {}) if isinstance(components, dict) else {}
    return schemas if isinstance(schemas, dict) else {}


def _request_schema_name(operation: dict[str, Any]) -> str:
    request_body = operation.get("requestBody", {})
    content = request_body.get("content", {}) if isinstance(request_body, dict) else {}
    json_content = content.get("application/json", {}) if isinstance(content, dict) else {}
    schema = json_content.get("schema", {}) if isinstance(json_content, dict) else {}
    return _ref_name(str(schema["$ref"])) if isinstance(schema, dict) and "$ref" in schema else ""


def _response_schema_name(operation: dict[str, Any]) -> str:
    response = operation.get("responses", {}).get("200", {})
    content = response.get("content", {}) if isinstance(response, dict) else {}
    json_content = content.get("application/json", {}) if isinstance(content, dict) else {}
    schema = json_content.get("schema", {}) if isinstance(json_content, dict) else {}
    return _ref_name(str(schema["$ref"])) if isinstance(schema, dict) and "$ref" in schema else ""


def load_ref_registry(
    schema_path: Path = GENERATOR_SCHEMA_FILE,
    list_path: Path = GENERATOR_LIST_FILE,
) -> RegistrySnapshot:
    schema = read_json(schema_path)
    listing = read_json(list_path)
    if isinstance(schema, dict) and isinstance(schema.get("data"), dict):
        schema = schema["data"]
    if isinstance(listing, dict) and isinstance(listing.get("data"), dict):
        listing = listing["data"]
    return RegistrySnapshot(schema=schema, listing=listing)


def model_records(snapshot: RegistrySnapshot) -> list[ModelRecord]:
    items = snapshot.listing.get("items", [])
    listing = {item.get("name"): item for item in items if isinstance(item, dict) and item.get("name")}
    records: list[ModelRecord] = []
    paths = snapshot.schema.get("paths", {})
    if not isinstance(paths, dict):
        return records
    for path, methods in paths.items():
        if not isinstance(methods, dict):
            continue
        operation = methods.get("post")
        if not isinstance(operation, dict):
            continue
        model = path.strip("/")
        item = listing.get(model, {})
        request_name = _request_schema_name(operation)
        request_schema = _components(snapshot.schema).get(request_name, {})
        caps = request_schema.get("x-capabilities", {}) if isinstance(request_schema, dict) else {}
        model_type = item.get("type") or (caps.get("model_type") if isinstance(caps, dict) else "") or ""
        records.append(
            ModelRecord(
                model=model,
                display_name=str(item.get("display_name") or ""),
                type=str(model_type),
                vip=bool(item.get("is_vip")),
                path=path,
                summary=str(operation.get("summary") or ""),
                request_schema=request_name,
                response_schema=_response_schema_name(operation),
                index=int(item.get("index") or 9999),
            )
        )
    return sorted(records, key=lambda row: (row.type, row.index, row.model))


def request_schema(snapshot: RegistrySnapshot, model: str) -> dict[str, Any] | None:
    path = "/" + model.strip("/")
    methods = snapshot.schema.get("paths", {}).get(path, {})
    operation = methods.get("post") if isinstance(methods, dict) else None
    if not isinstance(operation, dict):
        return None
    name = _request_schema_name(operation)
    schema = _components(snapshot.schema).get(name)
    return schema if isinstance(schema, dict) else None


def validate_body(snapshot: RegistrySnapshot, model: str, body: dict[str, Any]) -> list[str]:
    """Run a conservative local request-schema validation.

    This does not implement full JSON Schema. It catches missing required
    fields and obvious type mismatches without becoming another dependency.
    """

    schema = request_schema(snapshot, model)
    if not schema:
        return [f"unknown model or request schema: {model}"]
    errors: list[str] = []
    required = schema.get("required", [])
    if isinstance(required, list):
        for field in required:
            if isinstance(field, str) and field not in body:
                errors.append(f"missing required field: {field}")
    properties = schema.get("properties", {})
    if isinstance(properties, dict):
        for field, spec in properties.items():
            if field not in body or not isinstance(spec, dict):
                continue
            expected = spec.get("type")
            value = body[field]
            if expected == "string" and not isinstance(value, str):
                errors.append(f"{field} must be string")
            elif expected == "integer" and not isinstance(value, int):
                errors.append(f"{field} must be integer")
            elif expected == "number" and not isinstance(value, (int, float)):
                errors.append(f"{field} must be number")
            elif expected == "boolean" and not isinstance(value, bool):
                errors.append(f"{field} must be boolean")
            elif expected == "array" and not isinstance(value, list):
                errors.append(f"{field} must be array")
            elif expected == "object" and not isinstance(value, dict):
                errors.append(f"{field} must be object")
    return errors
