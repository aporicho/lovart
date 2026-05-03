"""Local user-level batch job orchestration for Lovart generation."""

from __future__ import annotations

import time
from pathlib import Path
from typing import Any

from lovart_reverse.downloads import download_artifacts
from lovart_reverse.errors import CreditRiskError, InputError, LovartError, SchemaInvalidError, UnknownPricingError
from lovart_reverse.generation import dry_run_request, find_task_id, generation_preflight, submit_model
from lovart_reverse.io_json import hash_bytes, write_json
from lovart_reverse.jobs.expansion import expand_jobs
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
    jobs = _expanded_jobs(jobs_file)
    run_dir = default_run_dir(jobs_file, out_dir)
    registry = load_ref_registry()
    for _, request in _iter_remote_requests(jobs):
        schema_errors = validate_body(registry, request["model"], request["body"])
        request["schema_errors"] = schema_errors
        if schema_errors:
            request["quote"] = {
                "model": request["model"],
                "quoted": False,
                "credits": None,
                "warnings": ["schema validation failed; quote skipped"],
            }
        else:
            request["quote"] = _safe_quote(request["model"], request["body"], language=language)
    _refresh_job_statuses(jobs)
    report = {
        "jobs_file": str(jobs_file),
        "jobs_file_hash": _jobs_file_hash(jobs_file),
        "run_dir": str(run_dir),
        "quote_file": str(quote_path(run_dir)),
        "summary": _quote_summary(jobs),
        "jobs": jobs,
        "remote_requests": [request for _, request in _iter_remote_requests(jobs)],
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
    state = _new_state(jobs_file, out_dir)
    _preflight_remote_requests(state, allow_paid=allow_paid, max_total_credits=max_total_credits, language=language)
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
    run_dir = default_run_dir(jobs_file, out_dir)
    if existing_state_has_remote_tasks(run_dir):
        raise InputError(
            "existing batch state has submitted tasks; use lovart jobs resume",
            {"state_file": str(state_path(run_dir)), "recommended_actions": ["run lovart jobs resume <jobs.jsonl>"]},
        )
    state = _new_state(jobs_file, out_dir)
    _preflight_remote_requests(state, allow_paid=allow_paid, max_total_credits=max_total_credits, language=language)
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
        _assert_same_jobs_file(state, jobs_file)
    else:
        state = _new_state(jobs_file, out_dir)
    if retry_failed:
        for _, request in _iter_remote_requests(state["jobs"]):
            if request.get("status") == "failed" and not request.get("task_id"):
                request["status"] = "pending"
                request["errors"] = []
    _preflight_remote_requests(
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


def _expanded_jobs(jobs_file: Path) -> list[dict[str, Any]]:
    return expand_jobs(load_job_records(jobs_file))


def _new_state(jobs_file: Path, out_dir: Path | None) -> dict[str, Any]:
    return new_state(jobs_file, _expanded_jobs(jobs_file), out_dir, jobs_file_hash=_jobs_file_hash(jobs_file))


def _jobs_file_hash(jobs_file: Path) -> str:
    return hash_bytes(jobs_file.read_bytes())


def _assert_same_jobs_file(state: dict[str, Any], jobs_file: Path) -> None:
    current_hash = _jobs_file_hash(jobs_file)
    saved_hash = state.get("jobs_file_hash")
    if saved_hash != current_hash:
        raise InputError(
            "jobs file changed since state was created; refusing to resume",
            {"state_file": state.get("state_file"), "saved_hash": saved_hash, "current_hash": current_hash},
        )


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


def _preflight_remote_requests(
    state: dict[str, Any],
    *,
    allow_paid: bool,
    max_total_credits: float | None,
    language: str,
    statuses: set[str] | None = None,
) -> None:
    for _, request in _iter_remote_requests(state["jobs"], statuses=statuses):
        request["errors"] = []
        preflight, blocking_error = generation_preflight(
            request["model"],
            request["body"],
            mode=request["mode"],
            allow_paid=allow_paid,
            max_credits=max_total_credits,
            live=True,
        )
        request["preflight"] = preflight
        request["request"] = dry_run_request(request["model"], request["body"], language=language)
        request["quote"] = _quote_from_preflight(preflight)
        if blocking_error:
            _add_error(request, blocking_error.code, blocking_error.message, blocking_error.details)
        if _quote_is_unknown(request):
            _add_error(request, "unknown_pricing", "live quote did not return an exact credit cost", {"quote": request.get("quote")})
    _refresh_job_statuses(state["jobs"])


def _quote_from_preflight(preflight: dict[str, Any]) -> dict[str, Any] | None:
    gate = preflight.get("gate")
    if isinstance(gate, dict) and isinstance(gate.get("pricing"), dict):
        return gate["pricing"]
    return None


def _quote_is_unknown(request: dict[str, Any]) -> bool:
    quote_result = request.get("quote")
    return not (isinstance(quote_result, dict) and quote_result.get("quoted"))


def _batch_gate_payload(
    state: dict[str, Any],
    *,
    allow_paid: bool,
    max_total_credits: float | None,
    statuses: set[str] | None = None,
) -> dict[str, Any]:
    selected = [request for _, request in _iter_remote_requests(state["jobs"], statuses=statuses)]
    unknown = [request["request_id"] for request in selected if _quote_is_unknown(request)]
    paid = [request["request_id"] for request in selected if _quoted_credits(request) > 0]
    schema_invalid = [request["request_id"] for request in selected if _schema_errors(request)]
    non_credit_blockers = [
        {
            "request_id": request["request_id"],
            "job_id": request["job_id"],
            "errors": [error for error in request.get("errors", []) if error.get("code") not in {"credit_risk", "unknown_pricing"}],
        }
        for request in selected
    ]
    non_credit_blockers = [item for item in non_credit_blockers if item["errors"]]
    total_credits = sum(_quoted_credits(request) for request in selected if not _quote_is_unknown(request))
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
        "selected_remote_requests": len(selected),
        "total_credits": total_credits,
        "paid_request_ids": paid,
        "unknown_pricing_request_ids": unknown,
        "schema_invalid_request_ids": schema_invalid,
        "blocking_requests": non_credit_blockers,
        "summary": summarize_state(state),
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
    if gate["schema_invalid_request_ids"]:
        raise SchemaInvalidError("one or more batch requests do not match their model schema", {"batch_gate": gate})
    if gate["blocking_requests"]:
        first = gate["blocking_requests"][0]["errors"][0]
        raise LovartError(first["code"], first["message"], {"batch_gate": gate}, 2)
    if gate["unknown_pricing_request_ids"]:
        raise UnknownPricingError("one or more batch requests have unknown pricing", {"batch_gate": gate})
    paid = gate["paid_request_ids"]
    if paid and not allow_paid:
        raise CreditRiskError(
            "batch generation may spend credits; pass --allow-paid --max-total-credits N to allow it",
            {"batch_gate": gate},
        )
    if paid and allow_paid and max_total_credits is None:
        raise CreditRiskError("--max-total-credits is required with --allow-paid", {"batch_gate": gate})
    if paid and max_total_credits is not None and float(gate["total_credits"]) > max_total_credits:
        raise CreditRiskError("quoted batch credits exceed --max-total-credits", {"batch_gate": gate})


def _submit_pending(state: dict[str, Any], *, language: str) -> None:
    for _, request in _iter_remote_requests(state["jobs"], statuses={"pending"}):
        try:
            response = submit_model(request["model"], request["body"], language=language)
            task_id = find_task_id(response)
            request["response"] = response
            request["task_id"] = task_id
            request["status"] = "submitted" if task_id else "failed"
            if not task_id:
                _add_error(request, "task_failed", "submit response did not include a task_id", {"response": response})
        except Exception as exc:
            request["status"] = "failed"
            _add_error(request, "remote_error", "batch request submission failed", {"type": exc.__class__.__name__, "message": str(exc)})
        _refresh_job_statuses(state["jobs"])
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
        active = [
            request
            for _, request in _iter_remote_requests(state["jobs"])
            if request.get("task_id") and request.get("status") in {"submitted", "running"}
        ]
        if not active:
            break
        for request in active:
            try:
                current = task_info(str(request["task_id"]))
                request["task"] = current
                remote_status = str(current.get("status") or "unknown").lower()
                request["artifacts"] = current.get("artifacts") or []
                if remote_status in COMPLETED_REMOTE_STATUSES:
                    request["status"] = "completed"
                    if download and request["artifacts"]:
                        request["downloads"] = download_artifacts(request["artifacts"], task_id=str(request["task_id"]))
                        request["status"] = "downloaded"
                elif remote_status in FAILED_REMOTE_STATUSES:
                    request["status"] = "failed"
                    _add_error(request, "task_failed", "remote Lovart task failed", {"task": current})
                else:
                    request["status"] = "running"
            except Exception as exc:
                request["status"] = "running"
                _add_error(request, "remote_error", "task polling failed", {"type": exc.__class__.__name__, "message": str(exc)})
        _refresh_job_statuses(state["jobs"])
        save_state(state)
        active = [
            request
            for _, request in _iter_remote_requests(state["jobs"])
            if request.get("task_id") and request.get("status") in {"submitted", "running"}
        ]
        if not active:
            break
        if time.monotonic() >= deadline:
            timed_out = True
            break
        time.sleep(poll_interval)
    state["timed_out"] = timed_out
    save_state(state)


def _state_result(state: dict[str, Any], operation: str) -> dict[str, Any]:
    failed_jobs = [job for job in state["jobs"] if job.get("status") == "failed"]
    remote_requests = [request for _, request in _iter_remote_requests(state["jobs"])]
    submitted = [request for request in remote_requests if request.get("task_id")]
    downloads = []
    for request in remote_requests:
        downloads.extend(request.get("downloads") or [])
    return {
        "operation": operation,
        "jobs_file": state.get("jobs_file"),
        "jobs_file_hash": state.get("jobs_file_hash"),
        "run_dir": state.get("run_dir"),
        "state_file": state.get("state_file"),
        "summary": summarize_state(state),
        "batch_gate": state.get("batch_gate"),
        "submitted": submitted,
        "remote_requests": remote_requests,
        "tasks": [
            {
                "job_id": request.get("job_id"),
                "request_id": request.get("request_id"),
                "task_id": request.get("task_id"),
                "status": request.get("status"),
                "task": request.get("task"),
            }
            for request in submitted
        ],
        "downloads": downloads,
        "failed": failed_jobs,
        "timed_out": bool(state.get("timed_out")),
        "jobs": state.get("jobs", []),
    }


def _quote_summary(jobs: list[dict[str, Any]]) -> dict[str, Any]:
    state = {"jobs": jobs}
    summary = summarize_state(state)
    quoted = 0
    schema_invalid = 0
    for _, request in _iter_remote_requests(jobs):
        if request.get("schema_errors"):
            schema_invalid += 1
        quote_result = request.get("quote")
        if isinstance(quote_result, dict) and quote_result.get("quoted"):
            quoted += 1
    return {
        **summary,
        "quoted_remote_requests": quoted,
        "schema_invalid_remote_requests": schema_invalid,
    }


def _iter_remote_requests(
    jobs: list[dict[str, Any]],
    statuses: set[str] | None = None,
) -> list[tuple[dict[str, Any], dict[str, Any]]]:
    result: list[tuple[dict[str, Any], dict[str, Any]]] = []
    for job in jobs:
        remote_requests = job.get("remote_requests")
        if not isinstance(remote_requests, list):
            continue
        for request in remote_requests:
            if not isinstance(request, dict):
                continue
            if statuses is not None and request.get("status") not in statuses:
                continue
            result.append((job, request))
    return result


def _refresh_job_statuses(jobs: list[dict[str, Any]]) -> None:
    for job in jobs:
        remote_requests = [request for request in job.get("remote_requests", []) if isinstance(request, dict)]
        statuses = [str(request.get("status") or "unknown") for request in remote_requests]
        if not statuses:
            job["status"] = "pending"
        elif any(status == "failed" for status in statuses):
            job["status"] = "failed"
        elif all(status == "downloaded" for status in statuses):
            job["status"] = "downloaded"
        elif all(status in {"completed", "downloaded"} for status in statuses):
            job["status"] = "completed"
        elif any(status in {"submitted", "running"} for status in statuses):
            job["status"] = "running"
        elif all(status == "skipped" for status in statuses):
            job["status"] = "skipped"
        else:
            job["status"] = "pending"
        job["errors"] = [error for request in remote_requests for error in request.get("errors", [])]


def _quoted_credits(request: dict[str, Any]) -> float:
    quote_result = request.get("quote")
    if not isinstance(quote_result, dict) or not quote_result.get("quoted"):
        return 0.0
    return float(quote_result.get("credits") or 0)


def _schema_errors(request: dict[str, Any]) -> list[Any]:
    preflight = request.get("preflight")
    if isinstance(preflight, dict):
        errors = preflight.get("schema_errors")
        return errors if isinstance(errors, list) else []
    errors = request.get("schema_errors")
    return errors if isinstance(errors, list) else []


def _add_error(request: dict[str, Any], code: str, message: str, details: dict[str, Any] | None = None) -> None:
    errors = request.setdefault("errors", [])
    if not isinstance(errors, list):
        errors = []
        request["errors"] = errors
    payload = {"code": code, "message": message, "details": details or {}}
    if payload not in errors:
        errors.append(payload)
