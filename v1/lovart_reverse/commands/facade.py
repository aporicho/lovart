"""Safe command facade shared by CLI and MCP wrappers."""

from __future__ import annotations

import json
import sys
import subprocess
from importlib import metadata
from pathlib import Path
from typing import Any

from lovart_reverse.auth.store import status as auth_status
from lovart_reverse.capture.runtime import reverse_extra_status
from lovart_reverse.config import config_for_model, global_config
from lovart_reverse.discovery import generator_list
from lovart_reverse.downloads import download_artifacts
from lovart_reverse.errors import InputError
from lovart_reverse.execution import LOCAL, PREFLIGHT, SUBMIT, annotate
from lovart_reverse.generation import dry_run_request, find_task_id, generation_preflight, submit_model
from lovart_reverse.io_json import hash_bytes
from lovart_reverse.jobs import dry_run_jobs, quote_jobs, quote_status, resume_jobs, run_jobs, status_jobs
from lovart_reverse.paths import (
    PACKAGE_DIR,
    GENERATOR_SCHEMA_FILE,
    MANIFEST_FILE,
    PACKAGE_REF_DIR,
    REF_DIR,
    ROOT,
    SIGNATURE_JS,
    SIGNER_WASM,
)
from lovart_reverse.pricing.quote import quote
from lovart_reverse.registry import load_ref_registry, model_records, validate_body
from lovart_reverse.setup import local_cache_status, setup_status
from lovart_reverse.task import task_info


def _package_version() -> str:
    try:
        return metadata.version("lovart-reverse")
    except metadata.PackageNotFoundError:
        return "0.1.0"


def _git_commit() -> str | None:
    repo_root = PACKAGE_DIR.parent
    if not (repo_root / ".git").exists():
        return _build_info_commit() or _direct_url_commit()
    try:
        result = subprocess.run(
            ["git", "-C", str(repo_root), "rev-parse", "--short", "HEAD"],
            capture_output=True,
            text=True,
            check=True,
        )
    except (subprocess.CalledProcessError, FileNotFoundError):
        return _direct_url_commit()
    return result.stdout.strip() or _build_info_commit() or _direct_url_commit()


def _build_info() -> dict[str, Any]:
    path = PACKAGE_DIR / "data" / "build_info.json"
    if not path.exists():
        return {}
    try:
        data = json.loads(path.read_text())
    except (OSError, json.JSONDecodeError):
        return {}
    return data if isinstance(data, dict) else {}


def _build_info_commit() -> str | None:
    commit = _build_info().get("git_commit")
    return str(commit)[:12] if commit else None


def _direct_url_commit() -> str | None:
    try:
        text = metadata.distribution("lovart-reverse").read_text("direct_url.json")
    except metadata.PackageNotFoundError:
        return None
    if not text:
        return None
    try:
        data = json.loads(text)
    except json.JSONDecodeError:
        return None
    vcs_info = data.get("vcs_info") if isinstance(data, dict) else None
    commit = vcs_info.get("commit_id") if isinstance(vcs_info, dict) else None
    return str(commit)[:12] if commit else None


def _file_hash(path: Path) -> str | None:
    return hash_bytes(path.read_bytes()) if path.exists() else None


def version_command() -> dict[str, Any]:
    return annotate({
        "package": "lovart-reverse",
        "version": _package_version(),
        "git_commit": _git_commit(),
        "binary_mode": bool(getattr(sys, "frozen", False)),
        "build": _build_info(),
        "runtime_root": str(ROOT),
        "ref_dir": str(REF_DIR),
        "package_ref_dir": str(PACKAGE_REF_DIR),
        "manifest": {"path": str(MANIFEST_FILE), "hash": _file_hash(MANIFEST_FILE)},
        "generator_schema": {"path": str(GENERATOR_SCHEMA_FILE), "hash": _file_hash(GENERATOR_SCHEMA_FILE)},
    }, LOCAL, network_required=False, remote_write=False, cache_used=True)


def self_test_command() -> dict[str, Any]:
    refs = {
        "manifest": {"path": str(MANIFEST_FILE), "exists": MANIFEST_FILE.exists()},
        "generator_schema": {"path": str(GENERATOR_SCHEMA_FILE), "exists": GENERATOR_SCHEMA_FILE.exists()},
        "signer_wasm": {"path": str(SIGNER_WASM), "exists": SIGNER_WASM.exists()},
        "signature_js": {"path": str(SIGNATURE_JS), "exists": SIGNATURE_JS.exists()},
    }
    setup = {"update": local_cache_status()}
    registry = load_ref_registry()
    models = model_records(registry)
    doctor = _doctor_payload()
    checks = {
        "json_stdout_envelope": True,
        "refs_available": all(item["exists"] for item in refs.values()),
        "models_available": bool(models),
        "doctor_ok": bool(doctor.get("ok")),
        "mcp_command_supported": True,
    }
    return annotate({
        "ok": all(checks.values()),
        "version": version_command(),
        "checks": checks,
        "runtime": {
            "binary_mode": bool(getattr(sys, "frozen", False)),
            "mcp_command_supported": True,
            "reverse_extra_available": reverse_extra_status()["available"],
            "reverse_extra": reverse_extra_status(),
        },
        "refs": refs,
        "auth": auth_status(),
        "setup": setup,
        "doctor": doctor,
    }, LOCAL, network_required=False, remote_write=False, cache_used=True)


def _doctor_payload() -> dict[str, Any]:
    from lovart_reverse.diagnostics.architecture import run_checks

    return run_checks().to_dict()


def setup_command() -> dict[str, Any]:
    return annotate(setup_status(), PREFLIGHT, network_required=True, remote_write=False, cache_used=True)


def models_command(refresh: bool = False) -> dict[str, Any]:
    if refresh:
        listing = generator_list(live=True)
        return annotate({"source": "remote", "raw": listing}, PREFLIGHT, network_required=True, remote_write=False, cache_used=False)
    snapshot = load_ref_registry()
    records = [record.to_dict() for record in model_records(snapshot)]
    return annotate({"source": "ref", "count": len(records), "models": records}, LOCAL, network_required=False, remote_write=False, cache_used=True)


def config_command(model: str | None = None, include_all: bool = False, example: str | None = None, global_: bool = False) -> dict[str, Any]:
    if global_:
        return annotate(global_config(), LOCAL, network_required=False, remote_write=False, cache_used=True)
    if not model:
        raise InputError("model is required unless --global is used")
    return annotate(config_for_model(model, include_all=include_all, example=example), LOCAL, network_required=False, remote_write=False, cache_used=True)


def quote_command(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    result = quote(model, body, language=language)
    result["schema_errors"] = validate_body(load_ref_registry(), model, body)
    return annotate(result, PREFLIGHT, network_required=True, remote_write=False, cache_used=True)


def generate_command(
    model: str,
    body: dict[str, Any],
    *,
    mode: str = "auto",
    dry_run: bool = False,
    allow_paid: bool = False,
    max_credits: float | None = None,
    language: str = "en",
    wait: bool = False,
    download: bool = False,
) -> dict[str, Any]:
    preflight, blocking_error = generation_preflight(
        model,
        body,
        mode=mode,
        allow_paid=allow_paid,
        max_credits=max_credits,
    )
    request = dry_run_request(model, body, language=language)
    if dry_run:
        return annotate({"submitted": False, "preflight": preflight, "request": request}, PREFLIGHT, network_required=True, remote_write=False, submitted=False, cache_used=True)
    if blocking_error:
        raise blocking_error
    response = submit_model(model, body, language=language, mode=mode)
    task_id = find_task_id(response)
    data: dict[str, Any] = {
        "preflight": preflight,
        "submitted": True,
        "task_id": task_id,
        "status": "submitted",
        "artifacts": [],
        "downloads": [],
        "response": response,
    }
    if wait and task_id:
        current = task_info(task_id, language=language)
        artifacts = current.get("artifacts") or []
        data.update({"status": current.get("status"), "task": current, "artifacts": artifacts})
        if download:
            data["downloads"] = download_artifacts(artifacts, task_id=task_id)
    return annotate(data, SUBMIT, network_required=True, remote_write=True, submitted=True, cache_used=True)


def jobs_quote_command(
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
    return annotate(quote_jobs(
        jobs_file,
        out_dir=out_dir,
        language=language,
        detail=detail,
        concurrency=concurrency,
        limit=limit,
        all_requests=all_requests,
        refresh=refresh,
        progress=progress,
    ), PREFLIGHT, network_required=True, remote_write=False, cache_used=True)


def jobs_dry_run_command(
    jobs_file: Path,
    out_dir: Path | None = None,
    *,
    allow_paid: bool = False,
    max_total_credits: float | None = None,
    language: str = "en",
    detail: str = "full",
) -> dict[str, Any]:
    result = dry_run_jobs(
        jobs_file,
        out_dir=out_dir,
        allow_paid=allow_paid,
        max_total_credits=max_total_credits,
        language=language,
        detail=detail,
    )
    return annotate(result, PREFLIGHT, network_required=True, remote_write=False, submitted=False, cache_used=True)


def jobs_run_command(
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
    result = run_jobs(
        jobs_file,
        out_dir=out_dir,
        allow_paid=allow_paid,
        max_total_credits=max_total_credits,
        language=language,
        wait=wait,
        download=download,
        download_dir=download_dir,
        timeout_seconds=timeout_seconds,
        poll_interval=poll_interval,
        detail=detail,
    )
    submitted = _has_submitted_jobs(result)
    return annotate(result, SUBMIT, network_required=True, remote_write=submitted, submitted=submitted, cache_used=True)


def jobs_status_command(run_dir: Path, *, detail: str = "summary") -> dict[str, Any]:
    return annotate(status_jobs(run_dir, detail=detail), LOCAL, network_required=False, remote_write=False)


def jobs_quote_status_command(run_dir: Path, jobs_file: Path | None = None) -> dict[str, Any]:
    return annotate(quote_status(run_dir, jobs_file=jobs_file), LOCAL, network_required=False, remote_write=False)


def jobs_resume_command(
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
    result = resume_jobs(
        jobs_file,
        out_dir=out_dir,
        allow_paid=allow_paid,
        max_total_credits=max_total_credits,
        language=language,
        wait=wait,
        download=download,
        download_dir=download_dir,
        retry_failed=retry_failed,
        timeout_seconds=timeout_seconds,
        poll_interval=poll_interval,
        detail=detail,
    )
    submitted = _has_submitted_jobs(result)
    return annotate(result, SUBMIT, network_required=True, remote_write=submitted, submitted=submitted, cache_used=True)


def _has_submitted_jobs(result: dict[str, Any]) -> bool:
    submitted = result.get("submitted")
    if isinstance(submitted, list) and submitted:
        return True
    summary = result.get("summary")
    counts = summary.get("remote_status_counts") if isinstance(summary, dict) else None
    if not isinstance(counts, dict):
        return False
    return any(int(counts.get(status) or 0) > 0 for status in ("submitted", "running", "completed", "downloaded"))
