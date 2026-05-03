"""Batch job file parsing and path resolution."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from lovart_reverse.errors import InputError

VALID_MODES = {"auto", "fast", "relax"}
QUANTITY_BODY_FIELDS = {"n", "max_images", "count"}


def load_job_records(jobs_file: Path) -> list[dict[str, Any]]:
    if not jobs_file.exists():
        raise InputError("jobs file does not exist", {"jobs_file": str(jobs_file)})
    if jobs_file.suffix == ".jsonl":
        records = _load_jsonl(jobs_file)
    else:
        records = _load_json(jobs_file)
    return _validate_records(records, jobs_file)


def default_run_dir(jobs_file: Path, out_dir: Path | None = None) -> Path:
    if out_dir:
        return out_dir
    return jobs_file.parent


def state_path(run_dir: Path) -> Path:
    return run_dir / "jobs_state.json"


def quote_path(run_dir: Path) -> Path:
    return run_dir / "jobs_quote.json"


def _load_jsonl(path: Path) -> list[Any]:
    records: list[Any] = []
    for line_number, line in enumerate(path.read_text().splitlines(), start=1):
        stripped = line.strip()
        if not stripped:
            continue
        try:
            records.append(json.loads(stripped))
        except json.JSONDecodeError as exc:
            raise InputError(
                "jobs JSONL contains invalid JSON",
                {"jobs_file": str(path), "line": line_number, "message": str(exc)},
            ) from exc
    return records


def _load_json(path: Path) -> list[Any]:
    try:
        data = json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        raise InputError("jobs JSON contains invalid JSON", {"jobs_file": str(path), "message": str(exc)}) from exc
    if isinstance(data, dict) and isinstance(data.get("jobs"), list):
        return data["jobs"]
    if isinstance(data, list):
        return data
    raise InputError("jobs JSON must be an array or an object with a jobs array", {"jobs_file": str(path)})


def _validate_records(records: list[Any], path: Path) -> list[dict[str, Any]]:
    if not records:
        raise InputError("jobs file is empty", {"jobs_file": str(path)})
    seen: set[str] = set()
    validated: list[dict[str, Any]] = []
    for index, raw in enumerate(records, start=1):
        if not isinstance(raw, dict):
            raise InputError("job must be a JSON object", {"jobs_file": str(path), "index": index})
        job_id = raw.get("job_id")
        model = raw.get("model")
        body = raw.get("body")
        mode = raw.get("mode", "auto")
        outputs_supplied = "outputs" in raw
        outputs = raw.get("outputs", 1)
        if not isinstance(job_id, str) or not job_id.strip():
            raise InputError("job_id is required for every job", {"jobs_file": str(path), "index": index})
        if job_id in seen:
            raise InputError("duplicate job_id in jobs file", {"jobs_file": str(path), "job_id": job_id})
        if not isinstance(model, str) or not model.strip():
            raise InputError("model is required for every job", {"jobs_file": str(path), "job_id": job_id})
        if mode not in VALID_MODES:
            raise InputError(
                "job mode must be auto, fast, or relax",
                {"jobs_file": str(path), "job_id": job_id, "mode": mode},
            )
        if not isinstance(body, dict):
            raise InputError("body must be a JSON object for every job", {"jobs_file": str(path), "job_id": job_id})
        if not isinstance(outputs, int) or isinstance(outputs, bool) or outputs < 1:
            raise InputError(
                "outputs must be a positive integer",
                {"jobs_file": str(path), "job_id": job_id, "outputs": outputs},
            )
        if outputs_supplied:
            conflicting = sorted(QUANTITY_BODY_FIELDS & set(body))
            if conflicting:
                raise InputError(
                    "outputs cannot be combined with quantity fields inside body",
                    {"jobs_file": str(path), "job_id": job_id, "conflicting_fields": conflicting},
                )
        seen.add(job_id)
        validated.append(
            {
                "job_id": job_id,
                "title": raw.get("title"),
                "model": model,
                "mode": mode,
                "outputs": outputs,
                "outputs_explicit": outputs_supplied,
                "body": body,
            }
        )
    return validated
