"""Local batch job orchestration for Lovart generation."""

from __future__ import annotations

import time
from pathlib import Path
from typing import Any

from lovart_reverse.downloads import download_artifacts
from lovart_reverse.errors import CreditRiskError, InputError, LovartError, SchemaInvalidError, UnknownPricingError
from lovart_reverse.generation import dry_run_request, find_task_id, generation_preflight, submit_model
from lovart_reverse.io_json import write_json
from lovart_reverse.jobs.records import default_run_dir, load_job_records, quote_path
from lovart_reverse.jobs.state import (
    existing_state_has_remote_tasks,
    load_state,
    new_state,
    save_state,
    state_path,
    summarize_state,
)
from lovart_reverse.pricing.quote import quote
from lovart_reverse.registry import load_ref_registry, validate_body
from lovart_reverse.task import task_info

COMPLETED_REMOTE_STATUSES = {"complete", "completed", "done", "finished", "success", "succeeded"}
FAILED_REMOTE_STATUSES = {"cancelled", "canceled", "error", "failed", "failure", "rejected"}


def quote_jobs(jobs_file: Path, out_dir: Path | None = None, language: str = "en") -> dict[str, Any]:
    jobs = load_job_records(jobs_file)
    run_dir = default_run_dir(jobs_file, out_dir)
    registry = load_ref_registry()
    quoted_jobs: list[dict[str, Any]] = []
    for job in jobs:
        schema_errors = validate_body(registry, job["model"], job["body"])
        if schema_errors:
            quoted = {
                "model": job["model"],
                "quoted": False,
                "credits": None,
                "warnings": ["schema validation failed; quote skipped"],
            }
        else:
            quoted = _safe_quote(job["model"], job["body"], language=language)
        quoted_jobs.append({**job, "schema_errors": schema_errors, "quote": quoted})
    report = {
        "jobs_file": str(jobs_file),
        "run_dir": str(run_dir),
        "quote_file": str(quote_path(run_dir)),
        "summary": _quote_summary(quoted_jobs),
        "jobs": quoted_jobs,
    }
    write_json(quote_path(run_dir), report)
    return report


def dry_run_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
) -> dict[str, Any]:
    jobs = load_job_records(jobs_file)
    state = new_state(jobs_file, jobs, out_dir)
    _preflight_entries(state, allow_paid=allow_paid, max_total_credits=max_total_credits, language=language)
    state["batch_gate"] = _batch_gate_payload(state, allow_paid=allow_paid, max_total_credits=max_total_credits)
    save_state(state)
    return _state_result(state, "dry_run")


def run_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
    wait: bool = False,
    download: bool = False,
    timeout_seconds: float = 3600,
    poll_interval: float = 5,
) -> dict[str, Any]:
    jobs = load_job_records(jobs_file)
    run_dir = default_run_dir(jobs_file, out_dir)
    if existing_state_has_remote_tasks(run_dir):
        raise InputError(
            "existing batch state has submitted tasks; use lovart jobs resume",
            {"state_file": str(state_path(run_dir)), "recommended_actions": ["run lovart jobs resume <jobs.jsonl>"]},
        )
    state = new_state(jobs_file, jobs, out_dir)
    _preflight_entries(state, allow_paid=allow_paid, max_total_credits=max_total_credits, language=language)
    _ensure_batch_allowed(state, allow_paid=allow_paid, max_total_credits=max_total_credits)
    save_state(state)
    _submit_pending(state, language=language)
    if wait:
        _wait_for_submitted(state, download=download, timeout_seconds=timeout_seconds, poll_interval=poll_interval)
    return _state_result(state, "run")


def resume_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
    wait: bool = False,
    download: bool = False,
    retry_failed: bool = False,
    timeout_seconds: float = 3600,
    poll_interval: float = 5,
) -> dict[str, Any]:
    run_dir = default_run_dir(jobs_file, out_dir)
    if state_path(run_dir).exists():
        state = load_state(run_dir)
    else:
        state = new_state(jobs_file, load_job_records(jobs_file), out_dir)
    if retry_failed:
        for entry in state["jobs"]:
            if entry.get("status") == "failed" and not entry.get("task_id"):
                entry["status"] = "pending"
                entry["errors"] = []
    _preflight_entries(
        state,
        allow_paid=allow_paid,
        max_total_credits=max_total_credits,
        language=language,
        statuses={"pending"},
    )
    _ensure_batch_allowed(state, allow_paid=allow_paid, max_total_credits=max_total_credits, statuses={"pending"})
    save_state(state)
    _submit_pending(state, language=language)
    if wait:
        _wait_for_submitted(state, download=download, timeout_seconds=timeout_seconds, poll_interval=poll_interval)
    return _state_result(state, "resume")


def status_jobs(run_dir: Path) -> dict[str, Any]:
    state = load_state(run_dir)
    return _state_result(state, "status")


def _safe_quote(model: str, body: dict[str, Any], language: str) -> dict[str, Any]:
    try:
        return quote(model, body, language=language)
    except Exception as exc:
        return {
            "model": model,
            "quoted": False,
            "credits": None,
            "quote_error": {"type": exc.__class__.__name__, "message": str(exc)},
            "warnings": ["live quote failed; credit spend is unknown"],
        }


def _preflight_entries(
    state: dict[str, Any],
    *,
    allow_paid: bool,
    max_total_credits: float | None,
    language: str,
    statuses: set[str] | None = None,
) -> None:
    for entry in state["jobs"]:
        if statuses is not None and entry.get("status") not in statuses:
            continue
        entry["errors"] = []
        preflight, blocking_error = generation_preflight(
            entry["model"],
            entry["body"],
            mode=entry["mode"],
            allow_paid=allow_paid,
            max_credits=max_total_credits,
            live=True,
        )
        entry["preflight"] = preflight
        entry["request"] = dry_run_request(entry["model"], entry["body"], language=language)
        entry["quote"] = _quote_from_preflight(preflight)
        if blocking_error:
            _add_error(entry, blocking_error.code, blocking_error.message, blocking_error.details)
        if _quote_is_unknown(entry):
            _add_error(entry, "unknown_pricing", "live quote did not return an exact credit cost", {"quote": entry.get("quote")})


def _quote_from_preflight(preflight: dict[str, Any]) -> dict[str, Any] | None:
    gate = preflight.get("gate")
    if isinstance(gate, dict) and isinstance(gate.get("pricing"), dict):
        return gate["pricing"]
    return None


def _quote_is_unknown(entry: dict[str, Any]) -> bool:
    quote_result = entry.get("quote")
    return not (isinstance(quote_result, dict) and quote_result.get("quoted"))


def _batch_gate_payload(
    state: dict[str, Any],
    *,
    allow_paid: bool,
    max_total_credits: float | None,
    statuses: set[str] | None = None,
) -> dict[str, Any]:
    summary = summarize_state(state)
    selected = _selected_entries(state, statuses)
    unknown = [entry["job_id"] for entry in selected if _quote_is_unknown(entry)]
    paid = [entry["job_id"] for entry in selected if _quoted_credits(entry) > 0]
    schema_invalid = [entry["job_id"] for entry in selected if _schema_errors(entry)]
    non_credit_blockers = [
        {"job_id": entry["job_id"], "errors": [error for error in entry.get("errors", []) if error.get("code") not in {"credit_risk", "unknown_pricing"}]}
        for entry in selected
    ]
    non_credit_blockers = [item for item in non_credit_blockers if item["errors"]]
    total_credits = sum(_quoted_credits(entry) for entry in selected if not _quote_is_unknown(entry))
    allowed = not schema_invalid and not non_credit_blockers and not unknown
    if paid and not allow_paid:
        allowed = False
    if paid and allow_paid and max_total_credits is None:
        allowed = False
    if paid and max_total_credits is not None and total_credits > max_total_credits:
        allowed = False
    return {
        "allowed": allowed,
        "allow_paid": allow_paid,
        "max_total_credits": max_total_credits,
        "selected_jobs": len(selected),
        "total_credits": total_credits,
        "paid_job_ids": paid,
        "unknown_pricing_job_ids": unknown,
        "schema_invalid_job_ids": schema_invalid,
        "blocking_jobs": non_credit_blockers,
        "summary": summary,
    }


def _ensure_batch_allowed(
    state: dict[str, Any],
    *,
    allow_paid: bool,
    max_total_credits: float | None,
    statuses: set[str] | None = None,
) -> None:
    gate = _batch_gate_payload(state, allow_paid=allow_paid, max_total_credits=max_total_credits, statuses=statuses)
    state["batch_gate"] = gate
    selected = _selected_entries(state, statuses)
    if gate["schema_invalid_job_ids"]:
        raise SchemaInvalidError("one or more batch jobs do not match their model schema", {"batch_gate": gate})
    if gate["blocking_jobs"]:
        first = gate["blocking_jobs"][0]["errors"][0]
        raise LovartError(first["code"], first["message"], {"batch_gate": gate}, 2)
    if gate["unknown_pricing_job_ids"]:
        raise UnknownPricingError("one or more batch jobs have unknown pricing", {"batch_gate": gate})
    paid = gate["paid_job_ids"]
    if paid and not allow_paid:
        raise CreditRiskError(
            "batch generation may spend credits; pass --allow-paid --max-total-credits N to allow it",
            {"batch_gate": gate},
        )
    if paid and allow_paid and max_total_credits is None:
        raise CreditRiskError("--max-total-credits is required with --allow-paid", {"batch_gate": gate})
    if paid and max_total_credits is not None and float(gate["total_credits"]) > max_total_credits:
        raise CreditRiskError("quoted batch credits exceed --max-total-credits", {"batch_gate": gate})
    if not selected:
        return


def _submit_pending(state: dict[str, Any], *, language: str) -> None:
    for entry in state["jobs"]:
        if entry.get("status") != "pending":
            continue
        try:
            response = submit_model(entry["model"], entry["body"], language=language)
            task_id = find_task_id(response)
            entry["response"] = response
            entry["task_id"] = task_id
            entry["status"] = "submitted" if task_id else "failed"
            if not task_id:
                _add_error(entry, "task_failed", "submit response did not include a task_id", {"response": response})
        except Exception as exc:
            entry["status"] = "failed"
            _add_error(entry, "remote_error", "batch job submission failed", {"type": exc.__class__.__name__, "message": str(exc)})
        save_state(state)


def _wait_for_submitted(
    state: dict[str, Any],
    *,
    download: bool,
    timeout_seconds: float,
    poll_interval: float,
) -> None:
    deadline = time.monotonic() + timeout_seconds
    timed_out = False
    while True:
        active = [entry for entry in state["jobs"] if entry.get("task_id") and entry.get("status") in {"submitted", "running"}]
        if not active:
            break
        for entry in active:
            try:
                current = task_info(str(entry["task_id"]))
                entry["task"] = current
                remote_status = str(current.get("status") or "unknown").lower()
                entry["artifacts"] = current.get("artifacts") or []
                if remote_status in COMPLETED_REMOTE_STATUSES:
                    entry["status"] = "completed"
                    if download and entry["artifacts"]:
                        entry["downloads"] = download_artifacts(entry["artifacts"], task_id=str(entry["task_id"]))
                        entry["status"] = "downloaded"
                elif remote_status in FAILED_REMOTE_STATUSES:
                    entry["status"] = "failed"
                    _add_error(entry, "task_failed", "remote Lovart task failed", {"task": current})
                else:
                    entry["status"] = "running"
            except Exception as exc:
                entry["status"] = "running"
                _add_error(entry, "remote_error", "task polling failed", {"type": exc.__class__.__name__, "message": str(exc)})
        save_state(state)
        active = [entry for entry in state["jobs"] if entry.get("task_id") and entry.get("status") in {"submitted", "running"}]
        if not active:
            break
        if time.monotonic() >= deadline:
            timed_out = True
            break
        time.sleep(poll_interval)
    state["timed_out"] = timed_out
    save_state(state)


def _state_result(state: dict[str, Any], operation: str) -> dict[str, Any]:
    failed = [entry for entry in state["jobs"] if entry.get("status") == "failed"]
    submitted = [entry for entry in state["jobs"] if entry.get("task_id")]
    downloads = []
    for entry in state["jobs"]:
        downloads.extend(entry.get("downloads") or [])
    return {
        "operation": operation,
        "jobs_file": state.get("jobs_file"),
        "run_dir": state.get("run_dir"),
        "state_file": state.get("state_file"),
        "summary": summarize_state(state),
        "batch_gate": state.get("batch_gate"),
        "submitted": submitted,
        "tasks": [
            {"job_id": entry.get("job_id"), "task_id": entry.get("task_id"), "status": entry.get("status"), "task": entry.get("task")}
            for entry in submitted
        ],
        "downloads": downloads,
        "failed": failed,
        "timed_out": bool(state.get("timed_out")),
        "jobs": state.get("jobs", []),
    }


def _quote_summary(jobs: list[dict[str, Any]]) -> dict[str, Any]:
    total = 0.0
    quoted = 0
    unknown = 0
    paid = 0
    zero = 0
    schema_invalid = 0
    for job in jobs:
        if job.get("schema_errors"):
            schema_invalid += 1
        quote_result = job.get("quote")
        if isinstance(quote_result, dict) and quote_result.get("quoted"):
            quoted += 1
            credits = float(quote_result.get("credits") or 0)
            total += credits
            if credits == 0:
                zero += 1
            else:
                paid += 1
        else:
            unknown += 1
    return {
        "total_jobs": len(jobs),
        "quoted_jobs": quoted,
        "unknown_pricing_jobs": unknown,
        "schema_invalid_jobs": schema_invalid,
        "total_credits": total,
        "zero_credit_jobs": zero,
        "paid_jobs": paid,
    }


def _selected_entries(state: dict[str, Any], statuses: set[str] | None) -> list[dict[str, Any]]:
    entries = [entry for entry in state.get("jobs", []) if isinstance(entry, dict)]
    if statuses is None:
        return entries
    return [entry for entry in entries if entry.get("status") in statuses]


def _quoted_credits(entry: dict[str, Any]) -> float:
    quote_result = entry.get("quote")
    if not isinstance(quote_result, dict) or not quote_result.get("quoted"):
        return 0.0
    return float(quote_result.get("credits") or 0)


def _schema_errors(entry: dict[str, Any]) -> list[Any]:
    preflight = entry.get("preflight")
    if not isinstance(preflight, dict):
        return []
    errors = preflight.get("schema_errors")
    return errors if isinstance(errors, list) else []


def _add_error(entry: dict[str, Any], code: str, message: str, details: dict[str, Any] | None = None) -> None:
    errors = entry.setdefault("errors", [])
    if not isinstance(errors, list):
        errors = []
        entry["errors"] = errors
    payload = {"code": code, "message": message, "details": details or {}}
    if payload not in errors:
        errors.append(payload)
