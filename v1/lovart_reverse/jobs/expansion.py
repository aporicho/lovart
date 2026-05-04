"""Expand user-level batch jobs into concrete Lovart remote requests."""

from __future__ import annotations

from typing import Any

from lovart_reverse.config import config_for_model

QUANTITY_FIELD_PRIORITY = ("n", "max_images", "count")


def expand_jobs(jobs: list[dict[str, Any]]) -> list[dict[str, Any]]:
    return [_expand_job(job) for job in jobs]


def _expand_job(job: dict[str, Any]) -> dict[str, Any]:
    outputs = int(job.get("outputs") or 1)
    outputs_explicit = bool(job.get("outputs_explicit", "outputs" in job))
    base_body = dict(job["body"])
    quantity = _quantity_field(job["model"])
    if outputs_explicit:
        remote_requests = _requests_from_outputs(job, base_body, outputs, quantity)
    else:
        outputs = _legacy_output_count(base_body)
        remote_requests = [
            _remote_request(
                job,
                index=1,
                output_count=outputs,
                body=base_body,
            )
        ]
    expanded = {
        "job_id": job["job_id"],
        "title": job.get("title"),
        "model": job["model"],
        "mode": job["mode"],
        "outputs": outputs,
        "outputs_explicit": outputs_explicit,
        "body": base_body,
        "status": "pending",
        "remote_requests": remote_requests,
        "errors": [],
    }
    return expanded


def _requests_from_outputs(
    job: dict[str, Any],
    base_body: dict[str, Any],
    outputs: int,
    quantity: dict[str, Any] | None,
) -> list[dict[str, Any]]:
    if quantity is None:
        return [
            _remote_request(job, index=index, output_count=1, body=dict(base_body))
            for index in range(1, outputs + 1)
        ]
    key = str(quantity["key"])
    maximum = int(quantity["maximum"])
    requests: list[dict[str, Any]] = []
    remaining = outputs
    index = 1
    while remaining > 0:
        count = min(maximum, remaining)
        body = dict(base_body)
        body[key] = count
        requests.append(_remote_request(job, index=index, output_count=count, body=body))
        remaining -= count
        index += 1
    return requests


def _quantity_field(model: str) -> dict[str, Any] | None:
    fields = config_for_model(model, include_all=True)["fields"]
    by_key = {field["key"]: field for field in fields}
    for key in QUANTITY_FIELD_PRIORITY:
        field = by_key.get(key)
        if _usable_quantity_field(field):
            return {"key": key, "maximum": int(field["maximum"])}
    for field in fields:
        if field.get("route_role") == "quantity" and _usable_quantity_field(field):
            return {"key": field["key"], "maximum": int(field["maximum"])}
    return None


def _usable_quantity_field(field: Any) -> bool:
    if not isinstance(field, dict):
        return False
    if field.get("type") != "integer":
        return False
    maximum = field.get("maximum")
    return isinstance(maximum, int) and not isinstance(maximum, bool) and maximum >= 1


def _legacy_output_count(body: dict[str, Any]) -> int:
    for key in QUANTITY_FIELD_PRIORITY:
        value = body.get(key)
        if isinstance(value, int) and not isinstance(value, bool) and value >= 1:
            return value
    return 1


def _remote_request(
    job: dict[str, Any],
    *,
    index: int,
    output_count: int,
    body: dict[str, Any],
) -> dict[str, Any]:
    request_id = f"{job['job_id']}-{index:03d}"
    return {
        "request_id": request_id,
        "job_id": job["job_id"],
        "model": job["model"],
        "mode": job["mode"],
        "output_count": output_count,
        "body": body,
        "status": "pending",
        "quote": None,
        "preflight": None,
        "request": None,
        "task_id": None,
        "response": None,
        "task": None,
        "artifacts": [],
        "downloads": [],
        "errors": [],
    }
