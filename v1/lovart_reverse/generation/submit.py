"""Submit Lovart LGW model requests."""

from __future__ import annotations

from typing import Any

from lovart_reverse.auth import saved_ids
from lovart_reverse.errors import RemoteError
from lovart_reverse.http.client import WWW_BASE, lgw_request, www_session
from lovart_reverse.pricing.web_parity import generation_input_args

TASKS_PATH = "/v1/generator/tasks"
SET_UNLIMITED_PATH = "/api/canva/agent-cashier/task/set/unlimited"
TAKE_SLOT_PATH = "/api/canva/agent-cashier/task/take/slot"


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


def task_request_payload(model: str, body: dict[str, Any], context_ids: dict[str, Any] | None = None) -> dict[str, Any]:
    ids = saved_ids() if context_ids is None else context_ids
    payload: dict[str, Any] = {"generator_name": model.strip("/"), "input_args": generation_input_args(model, body)}
    cid = ids.get("cid") or ids.get("webid") or ids.get("webId")
    project_id = ids.get("project_id") or ids.get("projectId")
    if cid:
        payload["cid"] = cid
    if project_id:
        payload["project_id"] = project_id
    return payload


def dry_run_request(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    return {
        "method": "POST",
        "path": TASKS_PATH,
        "language": language,
        "body": task_request_payload(model, body),
        "signature_required": True,
    }


def apply_generation_mode(mode: str, context_ids: dict[str, Any] | None = None, language: str = "en") -> dict[str, Any] | None:
    if mode not in {"fast", "relax"}:
        return None
    ids = saved_ids() if context_ids is None else context_ids
    cid = ids.get("cid") or ids.get("webid") or ids.get("webId")
    if not cid:
        raise RemoteError("Lovart generation mode requires cid captured from the browser", {"mode": mode})
    response = www_session({"accept-language": language}).post(
        f"{WWW_BASE}{SET_UNLIMITED_PATH}",
        json={"unlimited": mode == "relax", "cid": cid},
        timeout=30,
    )
    response.raise_for_status()
    payload = response.json()
    status = ((payload.get("data") or {}).get("status") if isinstance(payload, dict) else None) or ""
    if str(status).upper() != "SUCCESS":
        raise RemoteError("Lovart generation mode switch failed", {"mode": mode, "response": payload})
    return payload


def take_generation_slot(context_ids: dict[str, Any] | None = None, language: str = "en") -> dict[str, Any] | None:
    ids = saved_ids() if context_ids is None else context_ids
    cid = ids.get("cid") or ids.get("webid") or ids.get("webId")
    project_id = ids.get("project_id") or ids.get("projectId")
    if not cid or not project_id:
        return None
    response = www_session({"accept-language": language}).post(
        f"{WWW_BASE}{TAKE_SLOT_PATH}",
        json={"project_id": project_id, "cid": cid},
        timeout=30,
    )
    response.raise_for_status()
    return response.json()


def submit_model(model: str, body: dict[str, Any], language: str = "en", mode: str = "auto") -> dict[str, Any]:
    if mode in {"fast", "relax"}:
        apply_generation_mode(mode, language=language)
    take_generation_slot(language=language)
    response = lgw_request("POST", TASKS_PATH, body=task_request_payload(model, body), language=language)
    return response.json()
