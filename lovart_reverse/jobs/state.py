"""Persistent state for local Lovart batch jobs."""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from lovart_reverse.errors import InputError
from lovart_reverse.io_json import read_json, write_json
from lovart_reverse.jobs.records import default_run_dir, state_path

TERMINAL_STATUSES = {"completed", "downloaded", "failed", "skipped"}


def new_state(
    jobs_file: Path,
    jobs: list[dict[str, Any]],
    out_dir: Path | None = None,
    jobs_file_hash: str | None = None,
) -> dict[str, Any]:
    run_dir = default_run_dir(jobs_file, out_dir)
    now = _now()
    return {
        "jobs_file": str(jobs_file),
        "jobs_file_hash": jobs_file_hash,
        "run_dir": str(run_dir),
        "state_file": str(state_path(run_dir)),
        "created_at": now,
        "updated_at": now,
        "jobs": jobs,
    }


def load_state(run_dir: Path) -> dict[str, Any]:
    path = state_path(run_dir)
    if not path.exists():
        raise InputError("jobs state file does not exist", {"state_file": str(path)})
    data = read_json(path)
    if not isinstance(data, dict) or not isinstance(data.get("jobs"), list):
        raise InputError("jobs state file is invalid", {"state_file": str(path)})
    return data


def save_state(state: dict[str, Any]) -> None:
    state["updated_at"] = _now()
    write_json(Path(str(state["state_file"])), state)


def existing_state_has_remote_tasks(run_dir: Path) -> bool:
    path = state_path(run_dir)
    if not path.exists():
        return False
    data = read_json(path)
    if not isinstance(data, dict):
        return False
    jobs = data.get("jobs")
    if not isinstance(jobs, list):
        return False
    for job in jobs:
        if not isinstance(job, dict):
            continue
        if job.get("task_id"):
            return True
        remote_requests = job.get("remote_requests")
        if isinstance(remote_requests, list) and any(isinstance(request, dict) and request.get("task_id") for request in remote_requests):
            return True
    return False


def summarize_state(state: dict[str, Any]) -> dict[str, Any]:
    jobs = [job for job in state.get("jobs", []) if isinstance(job, dict)]
    job_counts: dict[str, int] = {}
    request_counts: dict[str, int] = {}
    total_credits = 0.0
    unknown_pricing = 0
    paid_requests = 0
    zero_credit_requests = 0
    remote_requests = _remote_requests(jobs)
    for job in jobs:
        status = str(job.get("status") or "unknown")
        job_counts[status] = job_counts.get(status, 0) + 1
    for request in remote_requests:
        status = str(request.get("status") or "unknown")
        request_counts[status] = request_counts.get(status, 0) + 1
        quote = request.get("quote")
        if isinstance(quote, dict) and quote.get("quoted"):
            credits = float(quote.get("credits") or 0)
            total_credits += credits
            if credits == 0:
                zero_credit_requests += 1
            else:
                paid_requests += 1
        else:
            unknown_pricing += 1
    failed = [job for job in jobs if job.get("status") == "failed"]
    requested_outputs = sum(int(job.get("outputs") or 1) for job in jobs)
    return {
        "logical_jobs": len(jobs),
        "total_jobs": len(jobs),
        "remote_requests": len(remote_requests),
        "requested_outputs": requested_outputs,
        "status_counts": job_counts,
        "remote_status_counts": request_counts,
        "total_credits": total_credits,
        "zero_credit_jobs": zero_credit_requests,
        "paid_jobs": paid_requests,
        "zero_credit_remote_requests": zero_credit_requests,
        "paid_remote_requests": paid_requests,
        "unknown_pricing_jobs": unknown_pricing,
        "unknown_pricing_remote_requests": unknown_pricing,
        "failed_jobs": len(failed),
        "complete": len(jobs) > 0 and all(job.get("status") in TERMINAL_STATUSES for job in jobs),
    }


def _remote_requests(jobs: list[dict[str, Any]]) -> list[dict[str, Any]]:
    requests: list[dict[str, Any]] = []
    for job in jobs:
        remote_requests = job.get("remote_requests")
        if isinstance(remote_requests, list):
            requests.extend(request for request in remote_requests if isinstance(request, dict))
        else:
            requests.append(job)
    return requests


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()
