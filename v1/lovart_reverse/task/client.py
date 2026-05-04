"""Task-status client for Lovart LGW generator tasks."""

from __future__ import annotations

from typing import Any

from lovart_reverse.http.client import lgw_request

TASKS_PATH = "/v1/generator/tasks"


def normalize_task_response(payload: dict[str, Any]) -> dict[str, Any]:
    data = payload.get("data") if isinstance(payload.get("data"), dict) else payload
    status = data.get("status") or data.get("state") or data.get("taskStatus")
    artifacts = data.get("artifacts") or data.get("result") or data.get("results") or []
    task_id = data.get("task_id") or data.get("taskId") or data.get("generator_task_id") or data.get("generatorTaskId")
    return {
        "task_id": task_id,
        "raw_status": status,
        "status": str(status or "unknown").lower(),
        "artifacts": artifacts,
        "raw": payload,
    }


def task_info(task_id: str, language: str = "en") -> dict[str, Any]:
    response = lgw_request("GET", TASKS_PATH, params={"task_id": task_id}, language=language, timeout=30)
    return normalize_task_response(response.json())
