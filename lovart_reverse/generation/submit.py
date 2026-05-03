"""Submit Lovart LGW model requests."""

from __future__ import annotations

from typing import Any

from lovart_reverse.http.client import lgw_request

TASKS_PATH = "/v1/generator/tasks"


def find_task_id(payload: Any) -> str | None:
    if isinstance(payload, dict):
        for key in ("task_id", "taskId", "generator_task_id", "generatorTaskId", "id"):
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


def task_request_payload(model: str, body: dict[str, Any]) -> dict[str, Any]:
    return {"generator_name": model.strip("/"), "input_args": dict(body)}


def dry_run_request(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    return {
        "method": "POST",
        "path": TASKS_PATH,
        "language": language,
        "body": task_request_payload(model, body),
        "signature_required": True,
    }


def submit_model(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    response = lgw_request("POST", TASKS_PATH, body=task_request_payload(model, body), language=language)
    return response.json()
