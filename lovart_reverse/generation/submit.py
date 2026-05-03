"""Submit Lovart LGW model requests."""

from __future__ import annotations

from typing import Any

from lovart_reverse.http.client import lgw_request


def find_task_id(payload: Any) -> str | None:
    if isinstance(payload, dict):
        for key in ("task_id", "taskId", "id"):
            value = payload.get(key)
            if isinstance(value, str):
                return value
        for value in payload.values():
            found = find_task_id(value)
            if found:
                return found
    elif isinstance(payload, list):
        for value in payload:
            found = find_task_id(value)
            if found:
                return found
    return None


def dry_run_request(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    path = "/" + model.strip("/")
    return {
        "method": "POST",
        "path": path,
        "language": language,
        "body": body,
        "signature_required": True,
    }


def submit_model(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    response = lgw_request("POST", "/" + model.strip("/"), body=body, language=language)
    return response.json()
