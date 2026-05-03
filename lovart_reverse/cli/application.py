"""JSON CLI for Lovart reverse tooling."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from lovart_reverse.auth.extract import extract_from_capture
from lovart_reverse.auth.store import status as auth_status
from lovart_reverse.capture import replay_capture
from lovart_reverse.cli.envelope import fail, ok
from lovart_reverse.config import config_for_model, global_config
from lovart_reverse.discovery import generator_list, generator_schema
from lovart_reverse.downloads import download_artifacts
from lovart_reverse.errors import InputError, LovartError
from lovart_reverse.generation import dry_run_request, find_task_id, generation_preflight, submit_model
from lovart_reverse.io_json import load_body
from lovart_reverse.jobs import dry_run_jobs, quote_jobs, resume_jobs, run_jobs, status_jobs
from lovart_reverse.paths import ROOT
from lovart_reverse.planning.planner import plan_for_model
from lovart_reverse.pricing.quote import quote
from lovart_reverse.registry import load_ref_registry, model_records, request_schema, validate_body
from lovart_reverse.setup import setup_status
from lovart_reverse.task import task_info
from lovart_reverse.update import check_update, diff_update, sync_metadata


def _add_body_args(parser: argparse.ArgumentParser) -> None:
    parser.add_argument("--body-file", type=Path)
    parser.add_argument("--body", help="inline JSON body")


def _load_body_args(args: argparse.Namespace) -> dict[str, Any]:
    return load_body(args.body, str(args.body_file) if args.body_file else None)


def _schema_validation(model: str, body: dict[str, Any]) -> list[str]:
    return validate_body(load_ref_registry(), model, body)


def cmd_auth(args: argparse.Namespace) -> dict[str, Any]:
    if args.auth_cmd == "status":
        return auth_status()
    if args.auth_cmd == "extract":
        return extract_from_capture(args.capture)
    raise ValueError("unknown auth command")


def cmd_models(args: argparse.Namespace) -> dict[str, Any]:
    if args.live:
        listing = generator_list(live=True)
        return {"source": "live", "raw": listing}
    snapshot = load_ref_registry()
    records = [record.to_dict() for record in model_records(snapshot)]
    return {"source": "ref", "count": len(records), "models": records}


def cmd_schema(args: argparse.Namespace) -> dict[str, Any]:
    if args.live:
        schema = generator_schema(live=True)
        return {"source": "live", "raw": schema}
    schema = request_schema(load_ref_registry(), args.model)
    if not schema:
        raise InputError("model schema not found", {"model": args.model})
    return {"source": "ref", "model": args.model, "schema": schema}


def cmd_quote(args: argparse.Namespace) -> dict[str, Any]:
    body = _load_body_args(args)
    result = quote(args.model, body, language=args.language)
    result["schema_errors"] = _schema_validation(args.model, body)
    return result


def cmd_free(args: argparse.Namespace) -> dict[str, Any]:
    from lovart_reverse.entitlement import free_check

    body = _load_body_args(args)
    result = free_check(args.model, body, mode=args.mode, live=not args.offline)
    result["schema_errors"] = _schema_validation(args.model, body)
    return result


def cmd_setup(args: argparse.Namespace) -> dict[str, Any]:
    return setup_status(offline=args.offline)


def cmd_config(args: argparse.Namespace) -> dict[str, Any]:
    if args.global_config:
        return global_config()
    if not args.model:
        raise InputError("model is required unless --global is used")
    return config_for_model(args.model, include_all=args.include_all, example=args.example)


def cmd_plan(args: argparse.Namespace) -> dict[str, Any]:
    body = _load_body_args(args)
    quote_mode = "offline" if args.offline else args.quote
    return plan_for_model(
        args.model,
        intent=args.intent,
        count=args.count,
        partial_body=body,
        quote_mode=quote_mode,
        candidate_limit=args.candidate_limit,
    )


def cmd_generate(args: argparse.Namespace) -> dict[str, Any]:
    body = _load_body_args(args)
    preflight, blocking_error = generation_preflight(
        args.model,
        body,
        mode=args.mode,
        allow_paid=args.allow_paid,
        max_credits=args.max_credits,
        live=not args.offline,
    )
    request = dry_run_request(args.model, body, language=args.language)
    if args.dry_run:
        return {"submitted": False, "preflight": preflight, "request": request}
    if blocking_error:
        raise blocking_error
    response = submit_model(args.model, body, language=args.language)
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
    if args.wait and task_id:
        current = task_info(task_id)
        artifacts = current.get("artifacts") or []
        data.update({"status": current.get("status"), "task": current, "artifacts": artifacts})
        if args.download:
            data["downloads"] = download_artifacts(artifacts, task_id=task_id)
    return data


def cmd_task(args: argparse.Namespace) -> dict[str, Any]:
    return task_info(args.task_id)


def cmd_jobs(args: argparse.Namespace) -> dict[str, Any]:
    if args.jobs_cmd == "quote":
        return quote_jobs(args.jobs_file, out_dir=args.out_dir, language=args.language)
    if args.jobs_cmd == "dry-run":
        return dry_run_jobs(
            args.jobs_file,
            out_dir=args.out_dir,
            allow_paid=args.allow_paid,
            max_total_credits=args.max_total_credits,
            language=args.language,
        )
    if args.jobs_cmd == "run":
        return run_jobs(
            args.jobs_file,
            out_dir=args.out_dir,
            allow_paid=args.allow_paid,
            max_total_credits=args.max_total_credits,
            language=args.language,
            wait=args.wait,
            download=args.download,
            timeout_seconds=args.timeout_seconds,
            poll_interval=args.poll_interval,
        )
    if args.jobs_cmd == "resume":
        return resume_jobs(
            args.jobs_file,
            out_dir=args.out_dir,
            allow_paid=args.allow_paid,
            max_total_credits=args.max_total_credits,
            language=args.language,
            wait=args.wait,
            download=args.download,
            retry_failed=args.retry_failed,
            timeout_seconds=args.timeout_seconds,
            poll_interval=args.poll_interval,
        )
    if args.jobs_cmd == "status":
        return status_jobs(args.run_dir)
    raise ValueError("unknown jobs command")


def cmd_update(args: argparse.Namespace) -> dict[str, Any]:
    if args.update_cmd == "check":
        return check_update()
    if args.update_cmd == "diff":
        return diff_update()
    if args.update_cmd == "sync":
        if not args.metadata_only:
            raise InputError("only --metadata-only sync is supported")
        return sync_metadata()
    raise ValueError("unknown update command")


def cmd_reverse(args: argparse.Namespace) -> dict[str, Any]:
    if args.reverse_cmd == "replay":
        return replay_capture(args.capture, submit=args.submit)
    if args.reverse_cmd == "capture":
        addon = ROOT / "lovart_reverse" / "capture" / "mitm_addon.py"
        return {
            "command": ["uv", "run", "mitmdump", "-s", str(addon), "--listen-port", str(args.port)],
            "proxy": f"http://127.0.0.1:{args.port}",
            "note": "run this command in a separate shell and browse Lovart through the proxy",
        }
    raise ValueError("unknown reverse command")


def cmd_doctor(args: argparse.Namespace) -> dict[str, Any]:
    from lovart_reverse.diagnostics.architecture import run_checks

    return run_checks().to_dict()


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="lovart")
    sub = parser.add_subparsers(dest="command", required=True)

    setup = sub.add_parser("setup")
    setup.add_argument("--offline", action="store_true")

    auth = sub.add_parser("auth")
    auth_sub = auth.add_subparsers(dest="auth_cmd", required=True)
    auth_sub.add_parser("status")
    auth_extract = auth_sub.add_parser("extract")
    auth_extract.add_argument("capture", type=Path)

    models = sub.add_parser("models")
    models.add_argument("--live", action="store_true")

    schema = sub.add_parser("schema")
    schema.add_argument("model")
    schema.add_argument("--live", action="store_true")

    config = sub.add_parser("config")
    config.add_argument("model", nargs="?")
    config.add_argument("--all", action="store_true", dest="include_all")
    config.add_argument("--example", choices=["defaults", "zero_credit"])
    config.add_argument("--global", action="store_true", dest="global_config")

    plan = sub.add_parser("plan")
    plan.add_argument("model", nargs="?")
    plan.add_argument("--intent", default="general")
    plan.add_argument("--count", type=int, default=1)
    _add_body_args(plan)
    plan.add_argument("--quote", choices=["live", "auto", "offline"], default="live")
    plan.add_argument("--candidate-limit", type=int, default=12)
    plan.add_argument("--offline", action="store_true")

    quote_cmd = sub.add_parser("quote")
    quote_cmd.add_argument("model")
    _add_body_args(quote_cmd)
    quote_cmd.add_argument("--language", default="en")

    free = sub.add_parser("free")
    free.add_argument("model")
    _add_body_args(free)
    free.add_argument("--mode", choices=["fast", "relax", "auto"], default="auto")
    free.add_argument("--offline", action="store_true")

    update = sub.add_parser("update")
    update_sub = update.add_subparsers(dest="update_cmd", required=True)
    update_sub.add_parser("check")
    update_sub.add_parser("diff")
    sync = update_sub.add_parser("sync")
    sync.add_argument("--metadata-only", action="store_true", required=True)

    generate = sub.add_parser("generate")
    generate.add_argument("model")
    _add_body_args(generate)
    generate.add_argument("--mode", choices=["fast", "relax", "auto"], default="auto")
    generate.add_argument("--dry-run", action="store_true")
    generate.add_argument("--submit", action="store_true", help=argparse.SUPPRESS)
    generate.add_argument("--allow-paid", action="store_true")
    generate.add_argument("--max-credits", type=float)
    generate.add_argument("--language", default="en")
    generate.add_argument("--wait", action="store_true")
    generate.add_argument("--download", action="store_true")
    generate.add_argument("--offline", action="store_true")

    task = sub.add_parser("task")
    task.add_argument("task_id")

    jobs = sub.add_parser("jobs")
    jobs_sub = jobs.add_subparsers(dest="jobs_cmd", required=True)
    jobs_quote = jobs_sub.add_parser("quote")
    jobs_quote.add_argument("jobs_file", type=Path)
    jobs_quote.add_argument("--out-dir", type=Path)
    jobs_quote.add_argument("--language", default="en")
    jobs_dry_run = jobs_sub.add_parser("dry-run")
    jobs_dry_run.add_argument("jobs_file", type=Path)
    jobs_dry_run.add_argument("--out-dir", type=Path)
    jobs_dry_run.add_argument("--allow-paid", action="store_true")
    jobs_dry_run.add_argument("--max-total-credits", type=float)
    jobs_dry_run.add_argument("--language", default="en")
    jobs_run = jobs_sub.add_parser("run")
    jobs_run.add_argument("jobs_file", type=Path)
    jobs_run.add_argument("--out-dir", type=Path)
    jobs_run.add_argument("--allow-paid", action="store_true")
    jobs_run.add_argument("--max-total-credits", type=float)
    jobs_run.add_argument("--language", default="en")
    jobs_run.add_argument("--wait", action="store_true")
    jobs_run.add_argument("--download", action="store_true")
    jobs_run.add_argument("--timeout-seconds", type=float, default=3600)
    jobs_run.add_argument("--poll-interval", type=float, default=5)
    jobs_status = jobs_sub.add_parser("status")
    jobs_status.add_argument("run_dir", type=Path)
    jobs_resume = jobs_sub.add_parser("resume")
    jobs_resume.add_argument("jobs_file", type=Path)
    jobs_resume.add_argument("--out-dir", type=Path)
    jobs_resume.add_argument("--allow-paid", action="store_true")
    jobs_resume.add_argument("--max-total-credits", type=float)
    jobs_resume.add_argument("--language", default="en")
    jobs_resume.add_argument("--wait", action="store_true")
    jobs_resume.add_argument("--download", action="store_true")
    jobs_resume.add_argument("--retry-failed", action="store_true")
    jobs_resume.add_argument("--timeout-seconds", type=float, default=3600)
    jobs_resume.add_argument("--poll-interval", type=float, default=5)

    reverse = sub.add_parser("reverse")
    reverse_sub = reverse.add_subparsers(dest="reverse_cmd", required=True)
    capture = reverse_sub.add_parser("capture")
    capture.add_argument("--port", type=int, default=8080)
    replay = reverse_sub.add_parser("replay")
    replay.add_argument("capture", type=Path)
    replay.add_argument("--submit", action="store_true")

    sub.add_parser("doctor")
    return parser


def dispatch(args: argparse.Namespace) -> dict[str, Any]:
    if args.command == "setup":
        return cmd_setup(args)
    if args.command == "config":
        return cmd_config(args)
    if args.command == "plan":
        return cmd_plan(args)
    if args.command == "auth":
        return cmd_auth(args)
    if args.command == "models":
        return cmd_models(args)
    if args.command == "schema":
        return cmd_schema(args)
    if args.command == "quote":
        return cmd_quote(args)
    if args.command == "free":
        return cmd_free(args)
    if args.command == "generate":
        return cmd_generate(args)
    if args.command == "task":
        return cmd_task(args)
    if args.command == "jobs":
        return cmd_jobs(args)
    if args.command == "update":
        return cmd_update(args)
    if args.command == "reverse":
        return cmd_reverse(args)
    if args.command == "doctor":
        return cmd_doctor(args)
    raise ValueError(f"unknown command: {args.command}")


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    try:
        print(json.dumps(ok(dispatch(args)), ensure_ascii=False, separators=(",", ":")))
        return 0
    except LovartError as exc:
        print(json.dumps(fail(exc), ensure_ascii=False, separators=(",", ":")))
        return 2
    except Exception as exc:
        print(
            json.dumps(
                fail(LovartError("internal_error", str(exc), {"type": exc.__class__.__name__})),
                ensure_ascii=False,
                separators=(",", ":"),
            )
        )
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
