"""JSON CLI for Lovart reverse tooling."""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path
from typing import Any

from lovart_reverse.auth.extract import extract_from_capture
from lovart_reverse.auth.store import status as auth_status
from lovart_reverse.capture.runtime import capture_command
from lovart_reverse.capture.session import DEFAULT_CAPTURE_URL, run_capture_session
from lovart_reverse.capture import replay_capture
from lovart_reverse.envelope import fail, ok
from lovart_reverse.commands import (
    config_command,
    generate_command,
    jobs_dry_run_command,
    jobs_quote_command,
    jobs_quote_status_command,
    jobs_resume_command,
    jobs_run_command,
    jobs_status_command,
    models_command,
    quote_command,
    self_test_command,
    setup_command,
    version_command,
)
from lovart_reverse.discovery import generator_schema
from lovart_reverse.errors import InputError, LovartError
from lovart_reverse.io_json import load_body
from lovart_reverse.mcp import mcp_install, mcp_status
from lovart_reverse.paths import PACKAGE_DIR
from lovart_reverse.registry import load_ref_registry, request_schema, validate_body
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
    return models_command(live=args.live)


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
    return quote_command(args.model, body, language=args.language)


def cmd_free(args: argparse.Namespace) -> dict[str, Any]:
    from lovart_reverse.entitlement import free_check

    body = _load_body_args(args)
    result = free_check(args.model, body, mode=args.mode, live=not args.offline)
    result["schema_errors"] = _schema_validation(args.model, body)
    return result


def cmd_setup(args: argparse.Namespace) -> dict[str, Any]:
    return setup_command(offline=args.offline)


def cmd_config(args: argparse.Namespace) -> dict[str, Any]:
    return config_command(args.model, include_all=args.include_all, example=args.example, global_=args.global_config)


def cmd_generate(args: argparse.Namespace) -> dict[str, Any]:
    body = _load_body_args(args)
    return generate_command(
        args.model,
        body,
        mode=args.mode,
        dry_run=args.dry_run,
        allow_paid=args.allow_paid,
        max_credits=args.max_credits,
        language=args.language,
        wait=args.wait,
        download=args.download,
        offline=args.offline,
    )


def cmd_task(args: argparse.Namespace) -> dict[str, Any]:
    return task_info(args.task_id)


def cmd_jobs(args: argparse.Namespace) -> dict[str, Any]:
    if args.jobs_cmd == "quote":
        jobs_file = args.jobs_file_option or args.jobs_file
        if jobs_file is None:
            raise InputError("jobs file is required", {"recommended_actions": ["pass lovart jobs quote <jobs.jsonl> or --jobs-file <jobs.jsonl>"]})
        return jobs_quote_command(
            jobs_file,
            out_dir=args.out_dir,
            language=args.language,
            detail=args.detail,
            concurrency=args.concurrency,
            limit=args.limit,
            all_requests=args.all,
            refresh=args.refresh,
            progress=not args.no_progress,
        )
    if args.jobs_cmd == "quote-status":
        return jobs_quote_status_command(args.run_dir, jobs_file=args.jobs_file)
    if args.jobs_cmd == "dry-run":
        return jobs_dry_run_command(
            args.jobs_file,
            out_dir=args.out_dir,
            allow_paid=args.allow_paid,
            max_total_credits=args.max_total_credits,
            language=args.language,
        )
    if args.jobs_cmd == "run":
        return jobs_run_command(
            args.jobs_file,
            out_dir=args.out_dir,
            allow_paid=args.allow_paid,
            max_total_credits=args.max_total_credits,
            language=args.language,
            wait=args.wait,
            download=args.download,
            download_dir=args.download_dir,
            timeout_seconds=args.timeout_seconds,
            poll_interval=args.poll_interval,
            detail=args.detail,
        )
    if args.jobs_cmd == "resume":
        return jobs_resume_command(
            args.jobs_file,
            out_dir=args.out_dir,
            allow_paid=args.allow_paid,
            max_total_credits=args.max_total_credits,
            language=args.language,
            wait=args.wait,
            download=args.download,
            download_dir=args.download_dir,
            retry_failed=args.retry_failed,
            timeout_seconds=args.timeout_seconds,
            poll_interval=args.poll_interval,
            detail=args.detail,
        )
    if args.jobs_cmd == "status":
        return jobs_status_command(args.run_dir, detail=args.detail)
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
        addon = PACKAGE_DIR / "capture" / "mitm_addon.py"
        return capture_command(args.port, addon)
    if args.reverse_cmd == "start":
        return run_capture_session(
            port=args.port,
            url=args.url,
            profile_dir=args.profile_dir,
            browser=args.browser,
            open_browser=not args.no_browser,
            dry_run=args.dry_run,
        )
    raise ValueError("unknown reverse command")


def cmd_doctor(args: argparse.Namespace) -> dict[str, Any]:
    from lovart_reverse.diagnostics.architecture import run_checks

    return run_checks().to_dict()


def cmd_mcp(args: argparse.Namespace) -> dict[str, Any]:
    if args.mcp_cmd == "status":
        return mcp_status(clients=args.clients, lovart_path=args.lovart_path, home=args.home)
    if args.mcp_cmd == "install":
        return mcp_install(
            clients=args.clients,
            lovart_path=args.lovart_path,
            home=args.home,
            dry_run=args.dry_run,
            yes=args.yes,
            force=args.force,
        )
    raise ValueError("unknown mcp command")


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="lovart")
    parser.add_argument("--version", action="store_true", dest="show_version")
    sub = parser.add_subparsers(dest="command")

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
    jobs_quote.add_argument("jobs_file", nargs="?", type=Path)
    jobs_quote.add_argument("--jobs-file", dest="jobs_file_option", type=Path)
    jobs_quote.add_argument("--out-dir", type=Path)
    jobs_quote.add_argument("--language", default="en")
    jobs_quote.add_argument("--detail", choices=["summary", "requests", "full"], default="summary")
    jobs_quote.add_argument("--concurrency", type=int, default=2)
    jobs_quote.add_argument("--limit", default="auto")
    jobs_quote.add_argument("--all", action="store_true")
    jobs_quote.add_argument("--refresh", action="store_true")
    jobs_quote.add_argument("--no-progress", action="store_true")
    jobs_quote_status = jobs_sub.add_parser("quote-status")
    jobs_quote_status.add_argument("run_dir", type=Path)
    jobs_quote_status.add_argument("--jobs-file", type=Path)
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
    jobs_run.add_argument("--download-dir", type=Path)
    jobs_run.add_argument("--timeout-seconds", type=float, default=3600)
    jobs_run.add_argument("--poll-interval", type=float, default=5)
    jobs_run.add_argument("--detail", choices=["summary", "requests", "full"], default="full")
    jobs_status = jobs_sub.add_parser("status")
    jobs_status.add_argument("run_dir", type=Path)
    jobs_status.add_argument("--detail", choices=["summary", "requests", "full"], default="summary")
    jobs_resume = jobs_sub.add_parser("resume")
    jobs_resume.add_argument("jobs_file", type=Path)
    jobs_resume.add_argument("--out-dir", type=Path)
    jobs_resume.add_argument("--allow-paid", action="store_true")
    jobs_resume.add_argument("--max-total-credits", type=float)
    jobs_resume.add_argument("--language", default="en")
    jobs_resume.add_argument("--wait", action="store_true")
    jobs_resume.add_argument("--download", action="store_true")
    jobs_resume.add_argument("--download-dir", type=Path)
    jobs_resume.add_argument("--retry-failed", action="store_true")
    jobs_resume.add_argument("--timeout-seconds", type=float, default=3600)
    jobs_resume.add_argument("--poll-interval", type=float, default=5)
    jobs_resume.add_argument("--detail", choices=["summary", "requests", "full"], default="full")

    reverse = sub.add_parser("reverse")
    reverse_sub = reverse.add_subparsers(dest="reverse_cmd", required=True)
    capture = reverse_sub.add_parser("capture")
    capture.add_argument("--port", type=int, default=8080)
    start = reverse_sub.add_parser("start")
    start.add_argument("--port", type=int, default=8080)
    start.add_argument("--url", default=DEFAULT_CAPTURE_URL)
    start.add_argument("--profile-dir", type=Path)
    start.add_argument("--browser", type=Path)
    start.add_argument("--no-browser", action="store_true")
    start.add_argument("--dry-run", action="store_true")
    replay = reverse_sub.add_parser("replay")
    replay.add_argument("capture", type=Path)
    replay.add_argument("--submit", action="store_true")

    sub.add_parser("self-test")
    mcp = sub.add_parser("mcp")
    mcp_sub = mcp.add_subparsers(dest="mcp_cmd", required=False)
    mcp_sub.add_parser("serve")
    mcp_status_cmd = mcp_sub.add_parser("status")
    mcp_status_cmd.add_argument("--clients", default="auto")
    mcp_status_cmd.add_argument("--lovart-path", type=Path)
    mcp_status_cmd.add_argument("--home", type=Path)
    mcp_install_cmd = mcp_sub.add_parser("install")
    mcp_install_cmd.add_argument("--clients", default="auto")
    mcp_install_cmd.add_argument("--lovart-path", type=Path)
    mcp_install_cmd.add_argument("--home", type=Path)
    mcp_install_cmd.add_argument("--yes", action="store_true")
    mcp_install_cmd.add_argument("--force", action="store_true")
    mcp_install_cmd.add_argument("--dry-run", action="store_true")
    sub.add_parser("doctor")
    return parser


def dispatch(args: argparse.Namespace) -> dict[str, Any]:
    if args.show_version:
        return version_command()
    if not args.command:
        raise InputError("command is required unless --version is used", {"recommended_actions": ["run lovart --help"]})
    if args.command == "setup":
        return cmd_setup(args)
    if args.command == "self-test":
        return self_test_command()
    if args.command == "config":
        return cmd_config(args)
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
    if args.command == "mcp":
        return cmd_mcp(args)
    raise ValueError(f"unknown command: {args.command}")


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.command == "mcp" and getattr(args, "mcp_cmd", None) in (None, "serve"):
        from lovart_reverse.mcp.server import main as mcp_main

        return mcp_main()
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
