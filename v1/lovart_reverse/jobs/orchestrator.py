"""Local user-level batch job orchestration for Lovart generation."""

from __future__ import annotations

import json
import sys
import time
from copy import deepcopy
from concurrent.futures import ThreadPoolExecutor, as_completed
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from lovart_reverse.downloads import download_artifacts
from lovart_reverse.errors import CreditRiskError, InputError, LovartError, SchemaInvalidError, UnknownPricingError
from lovart_reverse.generation import dry_run_request, find_task_id, generation_preflight, submit_model
from lovart_reverse.io_json import hash_bytes, read_json, write_json
from lovart_reverse.jobs.expansion import expand_jobs
from lovart_reverse.jobs.quote_signature import cost_signature_for_request
from lovart_reverse.jobs.records import (
    default_run_dir,
    load_job_records,
    quote_full_path,
    quote_path,
    quote_root_path,
    quote_state_dir,
    quote_state_path,
)
from lovart_reverse.jobs.state import (
    existing_state_has_remote_tasks,
    load_state,
    new_state,
    save_state,
    state_path,
    summarize_state,
)
from lovart_reverse.pricing.quote import QuoteClient, quote
from lovart_reverse.registry import load_ref_registry, validate_body
from lovart_reverse.task import task_info

COMPLETED_REMOTE_STATUSES = {"complete", "completed", "done", "finished", "success", "succeeded"}
FAILED_REMOTE_STATUSES = {"cancelled", "canceled", "error", "failed", "failure", "rejected"}
NETWORK_ERROR_MARKERS = (
    "Failed to resolve",
    "NameResolutionError",
    "Temporary failure in name resolution",
    "Name or service not known",
    "nodename nor servname",
    "Unknown host",
    "ConnectionError",
    "Max retries exceeded",
    "Network is unreachable",
)
AUTO_QUOTE_LIMIT = 100
COMPACT_TASK_SAMPLE_LIMIT = 20


def quote_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    language: str = "en",
    *,
    detail: str = "summary",
    concurrency: int = 2,
    limit: int | str | None = "auto",
    all_requests: bool = False,
    refresh: bool = False,
    progress: bool = True,
) -> dict[str, Any]:
    run_dir = default_run_dir(jobs_file, out_dir)
    if detail not in {"summary", "requests", "full"}:
        raise InputError("unknown quote detail mode", {"detail": detail, "allowed": ["summary", "requests", "full"]})
    warnings: list[str] = []
    concurrency, concurrency_warnings = _quote_concurrency(concurrency)
    warnings.extend(concurrency_warnings)
    state, state_warnings = _quote_state(jobs_file, run_dir, refresh=refresh)
    warnings.extend(state_warnings)
    registry = load_ref_registry()
    remote_requests = [request for _, request in _iter_remote_requests(state["jobs"])]
    if len(remote_requests) > 50:
        warnings.append("large batch quote detected; quote will auto-limit unless --all is passed")
    pending = _pending_quote_requests(remote_requests, limit=None)
    selected_limit, effective_limit, limit_warnings = _resolve_quote_limit(limit, all_requests=all_requests, pending_count=len(pending))
    warnings.extend(limit_warnings)
    selected = pending[:selected_limit] if selected_limit is not None else pending
    state["last_quote_run"] = {
        "limit": "all" if all_requests else limit,
        "effective_limit": effective_limit,
        "selected_remote_requests": len(selected),
        "pending_before_run": len(pending),
        "concurrency": concurrency,
    }
    if selected:
        state.pop("quote_blocker", None)
    _quote_progress(
        progress,
        "quote_started",
        {
            "selected_remote_requests": len(selected),
            "remote_requests": len(remote_requests),
            "concurrency": concurrency,
            "effective_limit": effective_limit,
        },
    )
    for request in selected:
        _prepare_quote_request(registry, request, state)
    selected = [request for request in selected if request.get("quote_status") == "pending"]
    if selected:
        try:
            with QuoteClient(language=language, persistent_signer=True) as quote_client:
                client_warnings, network_blocked = _quote_selected_requests(
                    state,
                    selected,
                    quote_client=quote_client,
                    language=language,
                    concurrency=concurrency,
                    progress=progress,
                )
                warnings.extend(client_warnings)
        except Exception as exc:
            code = _network_error_code(exc, default="timestamp_network_unavailable")
            state["quote_blocker"] = {
                "code": code,
                "message": "Lovart timestamp sync failed; quote stopped before pricing requests",
                "details": {"type": exc.__class__.__name__, "message": str(exc), "selected_remote_requests": len(selected)},
            }
            _quote_progress(progress, "quote_failed", state["quote_blocker"])
            network_blocked = True
        if network_blocked:
            warnings.append("Lovart network/DNS is unavailable; quote stopped early and remaining retryable requests were left pending")
    _refresh_job_statuses(state["jobs"])
    _save_quote_state(state)
    full_report = _quote_report(state, detail="full", warnings=warnings)
    summary_report = _quote_report(state, detail="summary", warnings=warnings)
    write_json(Path(str(state["full_quote_file"])), full_report)
    write_json(Path(str(state["quote_file"])), summary_report)
    _quote_progress(progress, "quote_completed", {"summary": summary_report["summary"], "quote_file": summary_report["quote_file"]})
    return _quote_report(state, detail=detail, warnings=warnings)


def quote_status(run_dir: Path, jobs_file: Path | None = None) -> dict[str, Any]:
    if jobs_file is not None:
        jobs_hash = _jobs_file_hash(jobs_file)
        state_dir = quote_state_dir(run_dir, jobs_file, jobs_hash)
        state = _load_quote_state(state_dir)
        return _quote_report(state, detail="summary", warnings=[])
    states = []
    seen_hashes: set[str] = set()
    root = quote_root_path(run_dir)
    if root.exists():
        for path in sorted(root.glob("*/jobs_quote_state.json")):
            try:
                state = read_json(path)
            except Exception:
                continue
            if isinstance(state, dict) and isinstance(state.get("jobs"), list):
                states.append(_quote_status_entry(state))
                if state.get("jobs_file_hash"):
                    seen_hashes.add(str(state["jobs_file_hash"]))
    legacy = quote_state_path(run_dir)
    if legacy.exists():
        try:
            state = read_json(legacy)
        except Exception:
            state = None
        if isinstance(state, dict) and isinstance(state.get("jobs"), list) and str(state.get("jobs_file_hash") or "") not in seen_hashes:
            states.append({**_quote_status_entry(state), "legacy": True})
    return {
        "operation": "quote_status",
        "run_dir": str(run_dir),
        "state_count": len(states),
        "states": states,
        "summary": _aggregate_quote_status(states),
    }


def dry_run_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
    detail: str = "full",
) -> dict[str, Any]:
    state = _new_state(jobs_file, out_dir)
    _preflight_remote_requests(state, allow_paid=allow_paid, max_total_credits=max_total_credits, language=language)
    state["batch_gate"] = _batch_gate_payload(state, allow_paid=allow_paid, max_total_credits=max_total_credits)
    save_state(state)
    return _state_result(state, "dry_run", detail=detail)


def run_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
    wait: bool = False,
    download: bool = False,
    download_dir: Path | None = None,
    timeout_seconds: float = 3600,
    poll_interval: float = 5,
    detail: str = "full",
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
    _set_download_dir(state, download_dir)
    save_state(state)
    _submit_pending(state, language=language)
    if wait:
        _wait_for_submitted(
            state,
            language=language,
            download=download,
            download_dir=download_dir,
            timeout_seconds=timeout_seconds,
            poll_interval=poll_interval,
        )
    elif download:
        _download_completed_artifacts(state, download_dir=download_dir)
    return _state_result(state, "run", detail=detail)


def resume_jobs(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
    wait: bool = False,
    download: bool = False,
    download_dir: Path | None = None,
    retry_failed: bool = False,
    timeout_seconds: float = 3600,
    poll_interval: float = 5,
    detail: str = "full",
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
    _set_download_dir(state, download_dir)
    save_state(state)
    _submit_pending(state, language=language)
    if wait:
        _wait_for_submitted(
            state,
            language=language,
            download=download,
            download_dir=download_dir,
            timeout_seconds=timeout_seconds,
            poll_interval=poll_interval,
        )
    elif download:
        _download_completed_artifacts(state, download_dir=download_dir)
    return _state_result(state, "resume", detail=detail)


def status_jobs(run_dir: Path, *, detail: str = "summary") -> dict[str, Any]:
    state = load_state(run_dir)
    return _state_result(state, "status", detail=detail)


def _quote_concurrency(value: int) -> tuple[int, list[str]]:
    if value < 1:
        raise InputError("--concurrency must be at least 1", {"concurrency": value})
    if value > 4:
        return 4, ["--concurrency was capped at 4 to avoid overloading the pricing endpoint"]
    return value, []


def _resolve_quote_limit(value: int | str | None, *, all_requests: bool, pending_count: int) -> tuple[int | None, int, list[str]]:
    if all_requests:
        return None, pending_count, []
    if value is None or value == "auto":
        if pending_count > AUTO_QUOTE_LIMIT:
            return AUTO_QUOTE_LIMIT, AUTO_QUOTE_LIMIT, [f"auto-limited this quote run to {AUTO_QUOTE_LIMIT} pending remote requests"]
        return None, pending_count, []
    try:
        parsed = int(value)
    except (TypeError, ValueError) as exc:
        raise InputError("--limit must be a positive integer or auto", {"limit": value}) from exc
    if parsed < 1:
        raise InputError("--limit must be a positive integer or auto", {"limit": value})
    return parsed, min(parsed, pending_count), []


def _now() -> str:
    return datetime.now(timezone.utc).isoformat()


def _quote_state(jobs_file: Path, run_dir: Path, *, refresh: bool) -> tuple[dict[str, Any], list[str]]:
    current_hash = _jobs_file_hash(jobs_file)
    state_dir = quote_state_dir(run_dir, jobs_file, current_hash)
    path = quote_state_path(state_dir)
    if path.exists() and not refresh:
        state = _load_quote_state(state_dir)
        return _with_quote_paths(state, jobs_file, current_hash, run_dir, state_dir), []
    legacy_path = quote_state_path(run_dir)
    if legacy_path.exists() and not refresh:
        legacy = read_json(legacy_path)
        if isinstance(legacy, dict) and legacy.get("jobs_file_hash") == current_hash:
            return _with_quote_paths(legacy, jobs_file, current_hash, run_dir, state_dir), ["migrated legacy quote state into per-file quote state"]
        return _new_quote_state(jobs_file, run_dir, current_hash, state_dir), [
            "ignored legacy quote state for a different jobs file; using isolated per-file quote state"
        ]
    return _new_quote_state(jobs_file, run_dir, current_hash, state_dir), []


def _new_quote_state(jobs_file: Path, run_dir: Path, jobs_file_hash: str, state_dir: Path) -> dict[str, Any]:
    now = _now()
    return {
        "jobs_file": str(jobs_file),
        "jobs_file_hash": jobs_file_hash,
        "run_dir": str(run_dir),
        "quote_state_file": str(quote_state_path(state_dir)),
        "quote_file": str(quote_path(state_dir)),
        "full_quote_file": str(quote_full_path(state_dir)),
        "quote_cache_file": str(state_dir / "jobs_quote_cache.json"),
        "created_at": now,
        "updated_at": now,
        "quote_cache": {},
        "jobs": _expanded_jobs(jobs_file),
    }


def _with_quote_paths(state: dict[str, Any], jobs_file: Path, jobs_file_hash: str, run_dir: Path, state_dir: Path) -> dict[str, Any]:
    state["jobs_file"] = str(jobs_file)
    state["jobs_file_hash"] = jobs_file_hash
    state["run_dir"] = str(run_dir)
    state["quote_state_file"] = str(quote_state_path(state_dir))
    state["quote_file"] = str(quote_path(state_dir))
    state["full_quote_file"] = str(quote_full_path(state_dir))
    state["quote_cache_file"] = str(state_dir / "jobs_quote_cache.json")
    state.setdefault("quote_cache", {})
    return state


def _load_quote_state(run_dir: Path) -> dict[str, Any]:
    path = quote_state_path(run_dir)
    if not path.exists():
        raise InputError("jobs quote state file does not exist", {"state_file": str(path)})
    data = read_json(path)
    if not isinstance(data, dict) or not isinstance(data.get("jobs"), list):
        raise InputError("jobs quote state file is invalid", {"state_file": str(path)})
    return data


def _save_quote_state(state: dict[str, Any]) -> None:
    state["updated_at"] = _now()
    write_json(Path(str(state["quote_state_file"])), state)
    if state.get("quote_cache_file"):
        write_json(Path(str(state["quote_cache_file"])), state.get("quote_cache", {}))


def _pending_quote_requests(remote_requests: list[dict[str, Any]], *, limit: int | None) -> list[dict[str, Any]]:
    pending = [
        request
        for request in remote_requests
        if request.get("quote_status") != "quoted"
        and (request.get("quote_status") != "failed" or _request_has_network_failure(request))
    ]
    return pending[:limit] if limit is not None else pending


def _prepare_quote_request(registry: Any, request: dict[str, Any], state: dict[str, Any]) -> None:
    request["errors"] = []
    schema_errors = validate_body(registry, request["model"], request["body"])
    request["schema_errors"] = schema_errors
    if schema_errors:
        request["quote_status"] = "failed"
        request["quote"] = {
            "model": request["model"],
            "quoted": False,
            "credits": None,
            "payable_credits": None,
            "listed_credits": None,
            "warnings": ["schema validation failed; quote skipped"],
        }
        _add_error(request, "schema_invalid", "schema validation failed; quote skipped", {"schema_errors": schema_errors})
        return
    signature = cost_signature_for_request(request["model"], request.get("mode") or "auto", request["body"])
    request["cost_signature"] = signature["signature"]
    request["cost_signature_version"] = signature["version"]
    request["cost_signature_basis"] = signature["basis"]
    cached = _quote_cache_entry(state, signature["signature"])
    if cached:
        _apply_cached_quote(request, cached)
        return
    request["quote_status"] = "pending"


def _quote_progress(enabled: bool, event: str, data: dict[str, Any]) -> None:
    if not enabled:
        return
    print(json.dumps({"event": event, **data}, ensure_ascii=False, separators=(",", ":")), file=sys.stderr, flush=True)


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
        return _quote_exception_payload(model, exc)


def _quote_cache_entry(state: dict[str, Any], cost_signature: str) -> dict[str, Any] | None:
    cache = state.setdefault("quote_cache", {})
    entry = cache.get(cost_signature) if isinstance(cache, dict) else None
    if not isinstance(entry, dict):
        return None
    quote_result = entry.get("quote")
    if isinstance(quote_result, dict) and quote_result.get("quoted"):
        return entry
    return None


def _apply_cached_quote(request: dict[str, Any], cached: dict[str, Any]) -> None:
    request["quote"] = deepcopy(cached["quote"])
    request["quote_status"] = "quoted"
    request["quote_source"] = "cost_signature_cache"
    request["representative_request_id"] = cached.get("representative_request_id")
    request["errors"] = []


def _signature_groups(requests: list[dict[str, Any]]) -> list[list[dict[str, Any]]]:
    grouped: dict[str, list[dict[str, Any]]] = {}
    order: list[str] = []
    for request in requests:
        signature = str(request.get("cost_signature") or request.get("request_id"))
        if signature not in grouped:
            grouped[signature] = []
            order.append(signature)
        grouped[signature].append(request)
    return [grouped[signature] for signature in order]


def _apply_remote_quote_to_group(state: dict[str, Any], representative: dict[str, Any], group: list[dict[str, Any]]) -> None:
    representative["quote_source"] = "remote"
    representative["representative_request_id"] = representative.get("request_id")
    signature = representative.get("cost_signature")
    if signature:
        state.setdefault("quote_cache", {})[str(signature)] = {
            "quote": deepcopy(representative["quote"]),
            "representative_request_id": representative.get("request_id"),
            "updated_at": _now(),
        }
    for request in group:
        if request is representative:
            continue
        request["quote"] = deepcopy(representative["quote"])
        request["quote_status"] = "quoted"
        request["quote_source"] = "cost_signature_cache"
        request["representative_request_id"] = representative.get("request_id")
        request["errors"] = []


def _quote_selected_requests(
    state: dict[str, Any],
    selected: list[dict[str, Any]],
    *,
    quote_client: QuoteClient,
    language: str,
    concurrency: int,
    progress: bool,
) -> tuple[list[str], bool]:
    completed_total = 0
    groups = _signature_groups(selected)
    representatives = [group[0] for group in groups]
    group_by_request_id = {str(group[0].get("request_id")): group for group in groups}
    for chunk in _chunks(representatives, concurrency):
        with ThreadPoolExecutor(max_workers=concurrency) as executor:
            futures = {
                executor.submit(_safe_quote_with_client, quote_client, request["model"], request["body"], language): request
                for request in chunk
            }
            completed_chunk: list[dict[str, Any]] = []
            for future in as_completed(futures):
                request = futures[future]
                group = group_by_request_id.get(str(request.get("request_id")), [request])
                try:
                    request["quote"] = future.result()
                    request["quote_status"] = "quoted" if isinstance(request["quote"], dict) and request["quote"].get("quoted") else "failed"
                    if request["quote_status"] == "failed":
                        error_code, message = _quote_failure_code_and_message(request.get("quote"))
                        _add_error(request, error_code, message, {"quote": request.get("quote")})
                        request["quote_source"] = "remote_failed"
                        completed_total += 1
                    else:
                        _apply_remote_quote_to_group(state, request, group)
                        completed_total += len(group)
                except Exception as exc:
                    request["quote_status"] = "failed"
                    request["quote"] = _quote_exception_payload(request.get("model"), exc)
                    error_code, message = _quote_failure_code_and_message(request.get("quote"))
                    _add_error(request, error_code, message, {"type": exc.__class__.__name__, "message": str(exc)})
                    request["quote_source"] = "remote_failed"
                    completed_total += 1
                completed_chunk.append(request)
                _refresh_job_statuses(state["jobs"])
                _save_quote_state(state)
                _quote_progress(
                    progress,
                    "quote_progress",
                    {
                        "completed_selected_remote_requests": completed_total,
                        "selected_remote_requests": len(selected),
                        "request_id": request.get("request_id"),
                        "quote_status": request.get("quote_status"),
                        "cost_signature": request.get("cost_signature"),
                    },
                )
            if completed_chunk and all(_request_has_network_failure(request) for request in completed_chunk):
                state["quote_blocker"] = {
                    "code": "network_unavailable",
                    "message": "Lovart quote endpoint is not reachable; stopped quote early",
                    "details": {
                        "host": "lgw.lovart.ai",
                        "completed_before_stop": completed_total,
                        "selected_remote_requests": len(selected),
                    },
                }
                _quote_progress(progress, "quote_failed", state["quote_blocker"])
                return _quote_client_warnings(quote_client), True
    return _quote_client_warnings(quote_client), False


def _safe_quote_with_client(quote_client: QuoteClient, model: str, body: dict[str, Any], language: str) -> dict[str, Any]:
    last_error: Exception | None = None
    for attempt in range(3):
        try:
            return quote_client.quote(model, body)
        except Exception as exc:
            last_error = exc
            if not _looks_like_network_error(exc) or attempt == 2:
                break
            time.sleep(0.25 * (attempt + 1))
    return _quote_exception_payload(model, last_error or RuntimeError("quote failed"))


def _quote_client_warnings(quote_client: QuoteClient) -> list[str]:
    return list(getattr(quote_client, "warnings", []) or [])


def _chunks(items: list[dict[str, Any]], size: int) -> list[list[dict[str, Any]]]:
    return [items[index : index + size] for index in range(0, len(items), size)]


def _quote_exception_payload(model: Any, exc: Exception) -> dict[str, Any]:
    error_code = _network_error_code(exc) if _looks_like_network_error(exc) else "unknown_pricing"
    warning = "Lovart network/DNS unavailable; credit spend is unknown" if _is_network_error_code(error_code) else "remote quote failed; credit spend is unknown"
    return {
        "model": model,
        "quoted": False,
        "credits": None,
        "payable_credits": None,
        "listed_credits": None,
        "quote_error": {"type": exc.__class__.__name__, "message": str(exc), "code": error_code},
        "warnings": [warning],
    }


def _quote_failure_code_and_message(quote_result: Any) -> tuple[str, str]:
    if _quote_result_has_network_error(quote_result):
        quote_error = quote_result.get("quote_error") if isinstance(quote_result, dict) else None
        if isinstance(quote_error, dict) and quote_error.get("code"):
            return str(quote_error["code"]), "remote quote failed because Lovart is not reachable"
        return "network_unavailable", "remote quote failed because Lovart is not reachable"
    return "unknown_pricing", "remote quote did not return an exact credit cost"


def _request_has_network_failure(request: dict[str, Any]) -> bool:
    errors = request.get("errors")
    if isinstance(errors, list) and any(isinstance(error, dict) and _is_network_error_code(error.get("code")) for error in errors):
        return True
    return _quote_result_has_network_error(request.get("quote"))


def _quote_result_has_network_error(quote_result: Any) -> bool:
    if not isinstance(quote_result, dict):
        return False
    quote_error = quote_result.get("quote_error")
    if isinstance(quote_error, dict) and _is_network_error_code(quote_error.get("code")):
        return True
    return _looks_like_network_error(quote_error)


def _network_error_code(error: Any, default: str = "network_unavailable") -> str:
    phase = getattr(error, "phase", None)
    if phase == "timestamp":
        return "timestamp_network_unavailable"
    if phase == "pricing":
        return "pricing_network_unavailable"
    return default


def _is_network_error_code(code: Any) -> bool:
    return str(code or "") in {"network_unavailable", "timestamp_network_unavailable", "pricing_network_unavailable"}


def _looks_like_network_error(error: Any) -> bool:
    if error is None:
        return False
    text = str(error)
    return any(marker in text for marker in NETWORK_ERROR_MARKERS)


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
        )
        request["preflight"] = preflight
        request["request"] = dry_run_request(request["model"], request["body"], language=language)
        request["quote"] = _quote_from_preflight(preflight)
        if blocking_error:
            _add_error(request, blocking_error.code, blocking_error.message, blocking_error.details)
        if _quote_is_unknown(request):
            _add_error(request, "unknown_pricing", "remote quote did not return an exact credit cost", {"quote": request.get("quote")})
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
            response = submit_model(request["model"], request["body"], language=language, mode=request["mode"])
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
    language: str,
    download: bool,
    download_dir: Path | None,
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
                current = task_info(str(request["task_id"]), language=language)
                request["task"] = current
                remote_status = str(current.get("status") or "unknown").lower()
                request["artifacts"] = current.get("artifacts") or []
                if remote_status in COMPLETED_REMOTE_STATUSES:
                    request["status"] = "completed"
                    if download and request["artifacts"]:
                        _download_request_artifacts(request, state, download_dir=download_dir)
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
    if download:
        _download_completed_artifacts(state, download_dir=download_dir)
    save_state(state)


def _set_download_dir(state: dict[str, Any], download_dir: Path | None) -> None:
    if download_dir is not None:
        state["download_dir"] = str(download_dir)


def _effective_download_dir(state: dict[str, Any], download_dir: Path | None) -> Path | None:
    if download_dir is not None:
        return download_dir
    saved = state.get("download_dir")
    return Path(str(saved)) if saved else None


def _download_completed_artifacts(state: dict[str, Any], *, download_dir: Path | None = None) -> None:
    changed = False
    for _, request in _iter_remote_requests(state.get("jobs", [])):
        if request.get("status") != "completed" or request.get("downloads"):
            continue
        task_id = request.get("task_id")
        artifacts = request.get("artifacts")
        if not task_id or not isinstance(artifacts, list) or not artifacts:
            continue
        if _download_request_artifacts(request, state, download_dir=download_dir):
            changed = True
        else:
            changed = True
    if changed:
        _refresh_job_statuses(state["jobs"])
        save_state(state)


def _download_request_artifacts(request: dict[str, Any], state: dict[str, Any], *, download_dir: Path | None = None) -> bool:
    task_id = request.get("task_id")
    artifacts = request.get("artifacts")
    if not task_id or not isinstance(artifacts, list) or not artifacts:
        return False
    try:
        output_dir = _effective_download_dir(state, download_dir)
        if output_dir is None:
            request["downloads"] = download_artifacts(artifacts, task_id=str(task_id))
        else:
            request["downloads"] = download_artifacts(artifacts, output_dir=output_dir, task_id=str(task_id))
        request["status"] = "downloaded"
        request["download_error"] = None
        return True
    except Exception as exc:
        request["status"] = "completed"
        request["download_error"] = {"type": exc.__class__.__name__, "message": str(exc)}
        _add_error(request, "download_failed", "artifact download failed; task remains completed and can be resumed", request["download_error"])
        return False


def _state_result(state: dict[str, Any], operation: str, *, detail: str = "full") -> dict[str, Any]:
    if detail == "summary":
        return _compact_state_result(state, operation, include_requests=False)
    if detail == "requests":
        return _compact_state_result(state, operation, include_requests=True)
    if detail != "full":
        raise InputError("unknown jobs detail mode", {"detail": detail, "allowed": ["summary", "requests", "full"]})
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
        "download_dir": state.get("download_dir"),
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
        "warnings": _state_warnings(state),
        "recommended_actions": _state_recommended_actions(state),
    }


def _compact_state_result(state: dict[str, Any], operation: str, *, include_requests: bool) -> dict[str, Any]:
    remote_requests = [request for _, request in _iter_remote_requests(state.get("jobs", []))]
    tasks = [_compact_task_request(request) for request in remote_requests if request.get("task_id")]
    task_samples = _task_samples(tasks)
    failed = [_compact_task_request(request) for request in remote_requests if request.get("status") == "failed"]
    downloads = []
    for request in remote_requests:
        downloads.extend(request.get("downloads") or [])
    result: dict[str, Any] = {
        "operation": operation,
        "jobs_file": state.get("jobs_file"),
        "jobs_file_hash": state.get("jobs_file_hash"),
        "run_dir": state.get("run_dir"),
        "state_file": state.get("state_file"),
        "download_dir": state.get("download_dir"),
        "summary": summarize_state(state),
        "batch_gate": _compact_batch_gate(state.get("batch_gate")),
        "task_count": len(tasks),
        "task_sample_limit": COMPACT_TASK_SAMPLE_LIMIT,
        "tasks_truncated": len(tasks) > len(task_samples),
        "tasks": task_samples,
        "download_count": len(downloads),
        "failed": failed,
        "timed_out": bool(state.get("timed_out")),
        "warnings": _state_warnings(state),
        "recommended_actions": _state_recommended_actions(state),
    }
    if include_requests:
        result["remote_requests"] = [_compact_task_request(request) for request in remote_requests]
    return result


def _task_samples(tasks: list[dict[str, Any]]) -> list[dict[str, Any]]:
    if len(tasks) <= COMPACT_TASK_SAMPLE_LIMIT:
        return tasks
    priority_statuses = {"failed", "running", "submitted"}
    prioritized = [task for task in tasks if task.get("status") in priority_statuses]
    remaining = [task for task in tasks if task.get("status") not in priority_statuses]
    return (prioritized + remaining)[:COMPACT_TASK_SAMPLE_LIMIT]


def _compact_batch_gate(batch_gate: Any) -> dict[str, Any] | None:
    if not isinstance(batch_gate, dict):
        return None
    return {
        "allowed": batch_gate.get("allowed"),
        "allow_paid": batch_gate.get("allow_paid"),
        "max_total_credits": batch_gate.get("max_total_credits"),
        "selected_remote_requests": batch_gate.get("selected_remote_requests"),
        "total_credits": batch_gate.get("total_credits"),
        "paid_request_count": len(batch_gate.get("paid_request_ids") or []),
        "unknown_pricing_request_count": len(batch_gate.get("unknown_pricing_request_ids") or []),
        "schema_invalid_request_count": len(batch_gate.get("schema_invalid_request_ids") or []),
        "blocking_request_count": len(batch_gate.get("blocking_requests") or []),
    }


def _compact_task_request(request: dict[str, Any]) -> dict[str, Any]:
    task = request.get("task") if isinstance(request.get("task"), dict) else {}
    artifacts = request.get("artifacts")
    if not isinstance(artifacts, list):
        artifacts = task.get("artifacts") if isinstance(task.get("artifacts"), list) else []
    downloads = request.get("downloads") if isinstance(request.get("downloads"), list) else []
    return {
        "job_id": request.get("job_id"),
        "title": request.get("title"),
        "request_id": request.get("request_id"),
        "model": request.get("model"),
        "mode": request.get("mode"),
        "output_count": request.get("output_count"),
        "status": request.get("status"),
        "task_id": request.get("task_id"),
        "remote_status": task.get("status"),
        "artifact_count": len(artifacts),
        "download_count": len(downloads),
        "quote_summary": _quote_summary_payload(request.get("quote")),
        "schema_errors": request.get("schema_errors") or [],
        "last_error": _last_error_summary(request),
    }


def _last_error_summary(request: dict[str, Any]) -> dict[str, Any] | None:
    errors = request.get("errors")
    if not isinstance(errors, list) or not errors:
        return None
    last = errors[-1]
    if not isinstance(last, dict):
        return {"code": "unknown", "message": str(last)}
    details = last.get("details") if isinstance(last.get("details"), dict) else {}
    return {
        "code": last.get("code"),
        "message": last.get("message"),
        "detail_type": details.get("type"),
    }


def _state_warnings(state: dict[str, Any]) -> list[str]:
    warnings: list[str] = []
    summary = summarize_state(state)
    if state.get("timed_out"):
        warnings.append("batch polling timed out locally; submitted task IDs are saved and resume can continue polling")
    if summary.get("remote_status_counts", {}).get("running") or summary.get("remote_status_counts", {}).get("submitted"):
        warnings.append("some remote requests are still active; use jobs resume or jobs status to continue safely")
    if summary.get("failed_jobs"):
        warnings.append("some jobs failed; inspect detail=requests and retry only when the failure is understood")
    return warnings


def _state_recommended_actions(state: dict[str, Any]) -> list[str]:
    actions: list[str] = []
    jobs_file = state.get("jobs_file") or "<jobs.jsonl>"
    run_dir = state.get("run_dir")
    out_dir_arg = f" --out-dir {run_dir}" if run_dir else ""
    remote_counts = summarize_state(state).get("remote_status_counts", {})
    active = int(remote_counts.get("submitted") or 0) + int(remote_counts.get("running") or 0)
    pending = int(remote_counts.get("pending") or 0)
    failed = int(remote_counts.get("failed") or 0)
    completed = int(remote_counts.get("completed") or 0)
    downloaded = int(remote_counts.get("downloaded") or 0)
    if active:
        actions.append(f"lovart jobs resume {jobs_file}{out_dir_arg} --wait --download --timeout-seconds 90")
        actions.append(f"lovart jobs status {run_dir}" if run_dir else "lovart jobs status <run_dir>")
    if pending:
        actions.append(f"lovart jobs resume {jobs_file}{out_dir_arg}")
    if completed:
        actions.append(f"lovart jobs resume {jobs_file}{out_dir_arg} --download --timeout-seconds 90")
    if failed:
        actions.append(f"lovart jobs status {run_dir} --detail requests" if run_dir else "lovart jobs status <run_dir> --detail requests")
    return list(dict.fromkeys(actions))


def _quote_status_entry(state: dict[str, Any]) -> dict[str, Any]:
    return {
        "jobs_file": state.get("jobs_file"),
        "jobs_file_hash": state.get("jobs_file_hash"),
        "state_file": state.get("quote_state_file"),
        "quote_file": state.get("quote_file"),
        "full_quote_file": state.get("full_quote_file"),
        "summary": _quote_summary(state),
    }


def _aggregate_quote_status(states: list[dict[str, Any]]) -> dict[str, Any]:
    totals = {
        "remote_requests": 0,
        "quoted_remote_requests": 0,
        "pending_quote_remote_requests": 0,
        "failed_quote_remote_requests": 0,
        "total_payable_credits": 0.0,
        "total_listed_credits": 0.0,
    }
    for state in states:
        summary = state.get("summary")
        if not isinstance(summary, dict):
            continue
        for key in ("remote_requests", "quoted_remote_requests", "pending_quote_remote_requests", "failed_quote_remote_requests"):
            totals[key] += int(summary.get(key) or 0)
        totals["total_payable_credits"] += float(summary.get("total_payable_credits") or 0)
        totals["total_listed_credits"] += float(summary.get("total_listed_credits") or 0)
    totals["total_credits"] = totals["total_payable_credits"]
    return totals


def _quote_report(state: dict[str, Any], *, detail: str, warnings: list[str]) -> dict[str, Any]:
    output_dir = Path(str(state["quote_state_file"])).parent
    jobs = state["jobs"]
    summary = _quote_summary(state)
    merged_warnings = list(dict.fromkeys(warnings + _quote_summary_warnings(summary)))
    report: dict[str, Any] = {
        "operation": "quote",
        "jobs_file": state.get("jobs_file"),
        "jobs_file_hash": state.get("jobs_file_hash"),
        "run_dir": state.get("run_dir"),
        "state_file": str(quote_state_path(output_dir)),
        "quote_file": str(quote_path(output_dir)),
        "full_quote_file": str(quote_full_path(output_dir)),
        "quote_cache_file": str(output_dir / "jobs_quote_cache.json"),
        "summary": summary,
        "warnings": merged_warnings,
        "recommended_actions": _quote_recommended_actions(summary, state),
    }
    if isinstance(state.get("quote_blocker"), dict):
        report["quote_blocker"] = state["quote_blocker"]
    if detail == "requests":
        report["remote_requests"] = [_compact_quote_request(request) for _, request in _iter_remote_requests(jobs)]
    elif detail == "full":
        report["jobs"] = jobs
        report["remote_requests"] = [request for _, request in _iter_remote_requests(jobs)]
    elif detail != "summary":
        raise InputError("unknown quote detail mode", {"detail": detail, "allowed": ["summary", "requests", "full"]})
    return report


def _compact_quote_request(request: dict[str, Any]) -> dict[str, Any]:
    return {
        "request_id": request.get("request_id"),
        "job_id": request.get("job_id"),
        "model": request.get("model"),
        "mode": request.get("mode"),
        "output_count": request.get("output_count"),
        "cost_signature": request.get("cost_signature"),
        "quote_source": request.get("quote_source"),
        "representative_request_id": request.get("representative_request_id"),
        "status": request.get("quote_status") or "pending",
        "quote_summary": _quote_summary_payload(request.get("quote")),
        "schema_errors": request.get("schema_errors") or [],
        "errors": request.get("errors") or [],
    }


def _quote_summary_payload(quote_result: Any) -> dict[str, Any] | None:
    if not isinstance(quote_result, dict):
        return None
    return {
        "quoted": bool(quote_result.get("quoted")),
        "credits": quote_result.get("credits"),
        "payable_credits": _quote_payable_credits(quote_result),
        "listed_credits": _quote_listed_credits(quote_result),
        "balance": quote_result.get("balance"),
        "warnings": quote_result.get("warnings") or [],
    }


def _quote_summary_warnings(summary: dict[str, Any]) -> list[str]:
    warnings: list[str] = []
    if summary.get("network_unavailable_remote_requests"):
        warnings.append("Lovart network/DNS is unavailable; remote quote cannot reach www.lovart.ai")
    if summary.get("listed_but_zero_payable_remote_requests"):
        warnings.append("some requests have payable_credits=0 but listed_credits>0; total_credits uses payable_credits")
    if summary.get("remote_requests", 0) > 50:
        warnings.append("large batch quote detected; quote uses automatic limiting unless --all is passed")
    return warnings


def _quote_recommended_actions(summary: dict[str, Any], state: dict[str, Any]) -> list[str]:
    actions: list[str] = []
    if summary.get("network_unavailable_remote_requests"):
        actions.append("fix DNS/network access to www.lovart.ai, then rerun lovart jobs quote <jobs.jsonl>")
    if summary.get("pending_quote_remote_requests"):
        actions.append(f"lovart jobs quote {state.get('jobs_file')}")
    if summary.get("remote_requests", 0) > 50:
        actions.append("use --all only when you intentionally want to quote every pending request in one command")
    if summary.get("failed_quote_remote_requests"):
        actions.append("inspect lovart jobs quote <jobs.jsonl> --detail requests")
    return actions


def _quote_summary(state: dict[str, Any]) -> dict[str, Any]:
    jobs = [job for job in state.get("jobs", []) if isinstance(job, dict)]
    summary = summarize_state({"jobs": jobs})
    quoted = 0
    schema_invalid = 0
    network_unavailable = 0
    cache_hits = 0
    cache_misses = 0
    quoted_representatives = 0
    signatures: set[str] = set()
    error_counts: dict[str, int] = {}
    quote_status_counts: dict[str, int] = {}
    for _, request in _iter_remote_requests(jobs):
        if request.get("cost_signature"):
            signatures.add(str(request["cost_signature"]))
        quote_source = request.get("quote_source")
        if quote_source == "cost_signature_cache":
            cache_hits += 1
        elif quote_source in {"remote", "remote_failed", "live", "live_failed"}:
            cache_misses += 1
        quote_status = str(request.get("quote_status") or "pending")
        quote_status_counts[quote_status] = quote_status_counts.get(quote_status, 0) + 1
        if request.get("schema_errors"):
            schema_invalid += 1
        errors = request.get("errors")
        if isinstance(errors, list):
            for error in errors:
                if isinstance(error, dict) and error.get("code"):
                    code = str(error["code"])
                    error_counts[code] = error_counts.get(code, 0) + 1
        if _request_has_network_failure(request):
            network_unavailable += 1
        quote_result = request.get("quote")
        if isinstance(quote_result, dict) and quote_result.get("quoted"):
            quoted += 1
            if quote_source in {"remote", "live"}:
                quoted_representatives += 1
    return {
        **summary,
        "quoted_remote_requests": quoted,
        "pending_quote_remote_requests": quote_status_counts.get("pending", 0) + quote_status_counts.get("running", 0),
        "failed_quote_remote_requests": quote_status_counts.get("failed", 0),
        "quote_complete": quote_status_counts.get("pending", 0) + quote_status_counts.get("running", 0) == 0,
        "quote_status_counts": quote_status_counts,
        "schema_invalid_remote_requests": schema_invalid,
        "network_unavailable_remote_requests": network_unavailable,
        "error_counts": error_counts,
        "effective_limit": (state.get("last_quote_run") or {}).get("effective_limit"),
        "cache_hits": cache_hits,
        "cache_misses": cache_misses,
        "signature_groups": len(signatures),
        "quoted_representative_requests": quoted_representatives,
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
    return _quote_payable_credits(quote_result)


def _quote_payable_credits(quote_result: dict[str, Any]) -> float:
    value = quote_result.get("payable_credits", quote_result.get("credits"))
    try:
        return float(value or 0)
    except (TypeError, ValueError):
        return 0.0


def _quote_listed_credits(quote_result: dict[str, Any]) -> float:
    value = quote_result.get("listed_credits")
    try:
        return float(value or 0)
    except (TypeError, ValueError):
        return 0.0


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
