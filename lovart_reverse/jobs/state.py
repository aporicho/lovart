"""Persistent state for local Lovart batch jobs."""

from __future__ import annotations

from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from lovart_reverse.errors import InputError
from lovart_reverse.io_json import read_json, write_json
from lovart_reverse.jobs.records import default_run_dir, state_path

TERMINAL_STATUSES = {"completed", "downloaded", "failed", "skipped"}


def new_state(jobs_file: Path, jobs: list[dict[str, Any]], out_dir: Path | None = None) -> dict[str, Any]:
    run_dir = default_run_dir(jobs_file, out_dir)
    now = _now()
    entries = []
    for job in jobs:
        entries.append(
            {
                "job_id": job["job_id"],
                "title": job.get("title"),
                "model": job["model"],
                "mode": job["mode"],
                "body": job["body"],
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
        )
    return {
        "jobs_file": str(jobs_file),
        "run_dir": str(run_dir),
        "state_file": str(state_path(run_dir)),
        "created_at": now,
        "updated_at": now,
        "jobs": entries,
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
    return any(isinstance(job, dict) and job.get("task_id") for job in jobs)


def summarize_state(state: dict[str, Any]) -> dict[str, Any]:
    jobs = [job for job in state.get("jobs", []) if isinstance(job, dict)]
    counts: dict[str, int] = {}
    total_credits = 0.0
    unknown_pricing = 0
    paid_jobs = 0
    zero_credit_jobs = 0
    for job in jobs:
        status = str(job.get("status") or "unknown")
        counts[status] = counts.get(status, 0) + 1
        quote = job.get("quote")
        if isinstance(quote, dict) and quote.get("quoted"):
            credits = float(quote.get("credits") or 0)
            total_credits += credits
            if credits == 0:
                zero_credit_jobs += 1
            else:
                paid_jobs += 1
        else:
            unknown_pricing += 1
    failed = [job for job in jobs if job.get("status") == "failed"]
    return {
        "total_jobs": len(jobs),
        "status_counts": counts,
        "total_credits": total_credits,
        "zero_credit_jobs": zero_credit_jobs,
        "paid_jobs": paid_jobs,
        "unknown_pricing_jobs": unknown_pricing,
        "failed_jobs": len(failed),
        "complete": len(jobs) > 0 and all(job.get("status") in TERMINAL_STATUSES for job in jobs),
    }


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()
