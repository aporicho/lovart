"""Task-status clients for reverse-confirmed and candidate Lovart endpoints."""

from __future__ import annotations

from typing import Any

from lovart_reverse.http.client import CANVA_BASE, www_session


def normalize_task_response(payload: dict[str, Any]) -> dict[str, Any]:
    data = payload.get("data") if isinstance(payload.get("data"), dict) else payload
    status = data.get("status") or data.get("state") or data.get("taskStatus")
    artifacts = data.get("artifacts") or data.get("result") or data.get("results") or []
    return {"raw_status": status, "status": str(status or "unknown").lower(), "artifacts": artifacts, "raw": payload}


def task_info(task_id: str) -> dict[str, Any]:
    response = www_session().get(
        f"{CANVA_BASE}/agent/v1/generators/taskInfo",
        params={"task_id": task_id, "taskId": task_id},
        timeout=30,
    )
    response.raise_for_status()
    return normalize_task_response(response.json())
