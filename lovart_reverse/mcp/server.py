"""Minimal MCP stdio server for safe Lovart generation commands."""

from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any, Callable

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
    plan_command,
    quote_command,
    setup_command,
    version_command,
)
from lovart_reverse.errors import LovartError

PROTOCOL_VERSION = "2024-11-05"


def _string_schema(description: str = "") -> dict[str, Any]:
    schema: dict[str, Any] = {"type": "string"}
    if description:
        schema["description"] = description
    return schema


def _number_schema(description: str = "") -> dict[str, Any]:
    schema: dict[str, Any] = {"type": "number"}
    if description:
        schema["description"] = description
    return schema


def _boolean_schema(description: str = "") -> dict[str, Any]:
    schema: dict[str, Any] = {"type": "boolean"}
    if description:
        schema["description"] = description
    return schema


def _tool(name: str, description: str, properties: dict[str, Any], required: list[str] | None = None) -> dict[str, Any]:
    return {
        "name": name,
        "description": description,
        "inputSchema": {
            "type": "object",
            "properties": properties,
            "required": required or [],
            "additionalProperties": False,
        },
    }


TOOLS: list[dict[str, Any]] = [
    _tool("lovart_setup", "Check Lovart CLI readiness without exposing secrets.", {"offline": _boolean_schema()}),
    _tool("lovart_models", "List known Lovart generator models.", {"live": _boolean_schema()}),
    _tool(
        "lovart_config",
        "Return exhaustive legal config values for one model.",
        {"model": _string_schema(), "include_all": _boolean_schema(), "example": _string_schema()},
        ["model"],
    ),
    _tool(
        "lovart_plan",
        "Plan quality, cost, and speed routes without submitting generation.",
        {
            "model": _string_schema(),
            "intent": _string_schema(),
            "count": {"type": "integer"},
            "body": {"type": "object"},
            "quote_mode": {"type": "string", "enum": ["live", "auto", "offline"]},
            "candidate_limit": {"type": "integer"},
            "offline": _boolean_schema(),
        },
    ),
    _tool(
        "lovart_quote",
        "Fetch exact Lovart credit quote for a model request.",
        {"model": _string_schema(), "body": {"type": "object"}, "language": _string_schema()},
        ["model", "body"],
    ),
    _tool(
        "lovart_generate_dry_run",
        "Run generation preflight without submitting.",
        {
            "model": _string_schema(),
            "body": {"type": "object"},
            "mode": {"type": "string", "enum": ["auto", "fast", "relax"]},
            "allow_paid": _boolean_schema(),
            "max_credits": _number_schema(),
            "language": _string_schema(),
            "offline": _boolean_schema(),
        },
        ["model", "body"],
    ),
    _tool(
        "lovart_generate",
        "Submit a single generation request after the normal paid/zero-credit gate.",
        {
            "model": _string_schema(),
            "body": {"type": "object"},
            "mode": {"type": "string", "enum": ["auto", "fast", "relax"]},
            "allow_paid": _boolean_schema(),
            "max_credits": _number_schema(),
            "language": _string_schema(),
            "wait": _boolean_schema(),
            "download": _boolean_schema(),
            "offline": _boolean_schema(),
        },
        ["model", "body"],
    ),
    _tool(
        "lovart_jobs_quote",
        "Quote a user-level batch jobs JSONL file.",
        {
            "jobs_file": _string_schema(),
            "out_dir": _string_schema(),
            "language": _string_schema(),
            "detail": {"type": "string", "enum": ["summary", "requests", "full"]},
            "concurrency": {"type": "integer"},
            "limit": {"oneOf": [{"type": "integer"}, {"type": "string", "enum": ["auto"]}]},
            "all": _boolean_schema(),
            "refresh": _boolean_schema(),
            "progress": _boolean_schema(),
        },
        ["jobs_file"],
    ),
    _tool(
        "lovart_jobs_quote_status",
        "Read local batch quote progress.",
        {"run_dir": _string_schema(), "jobs_file": _string_schema()},
        ["run_dir"],
    ),
    _tool(
        "lovart_jobs_dry_run",
        "Run whole-batch preflight without submitting.",
        {
            "jobs_file": _string_schema(),
            "out_dir": _string_schema(),
            "allow_paid": _boolean_schema(),
            "max_total_credits": _number_schema(),
            "language": _string_schema(),
        },
        ["jobs_file"],
    ),
    _tool(
        "lovart_jobs_run",
        "Submit all pending batch remote requests after whole-batch gate passes.",
        {
            "jobs_file": _string_schema(),
            "out_dir": _string_schema(),
            "allow_paid": _boolean_schema(),
            "max_total_credits": _number_schema(),
            "language": _string_schema(),
            "wait": _boolean_schema(),
            "download": _boolean_schema(),
            "timeout_seconds": _number_schema(),
            "poll_interval": _number_schema(),
        },
        ["jobs_file"],
    ),
    _tool("lovart_jobs_status", "Read local batch run state.", {"run_dir": _string_schema()}, ["run_dir"]),
    _tool(
        "lovart_jobs_resume",
        "Resume an interrupted batch without resubmitting existing task IDs.",
        {
            "jobs_file": _string_schema(),
            "out_dir": _string_schema(),
            "allow_paid": _boolean_schema(),
            "max_total_credits": _number_schema(),
            "language": _string_schema(),
            "wait": _boolean_schema(),
            "download": _boolean_schema(),
            "retry_failed": _boolean_schema(),
            "timeout_seconds": _number_schema(),
            "poll_interval": _number_schema(),
        },
        ["jobs_file"],
    ),
]

UNSAFE_TOOL_NAMES = {
    "auth_extract",
    "lovart_auth_extract",
    "reverse_capture",
    "lovart_reverse_capture",
    "update_sync",
    "lovart_update_sync",
}


def list_tools() -> list[dict[str, Any]]:
    return TOOLS


def _path(value: Any) -> Path | None:
    return Path(str(value)) if value else None


def _body(args: dict[str, Any]) -> dict[str, Any]:
    body = args.get("body") or {}
    if not isinstance(body, dict):
        raise LovartError("input_error", "body must be an object", {"body_type": type(body).__name__})
    return body


def call_tool_data(name: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
    args = arguments or {}
    handlers: dict[str, Callable[[], dict[str, Any]]] = {
        "lovart_setup": lambda: setup_command(offline=bool(args.get("offline", False))),
        "lovart_models": lambda: models_command(live=bool(args.get("live", False))),
        "lovart_config": lambda: config_command(
            str(args["model"]),
            include_all=bool(args.get("include_all", False)),
            example=args.get("example"),
        ),
        "lovart_plan": lambda: plan_command(
            str(args["model"]) if args.get("model") else None,
            intent=str(args.get("intent") or "general"),
            count=int(args.get("count") or 1),
            body=_body(args),
            quote_mode=str(args.get("quote_mode") or "live"),
            candidate_limit=int(args.get("candidate_limit") or 12),
            offline=bool(args.get("offline", False)),
        ),
        "lovart_quote": lambda: quote_command(str(args["model"]), _body(args), language=str(args.get("language") or "en")),
        "lovart_generate_dry_run": lambda: generate_command(
            str(args["model"]),
            _body(args),
            mode=str(args.get("mode") or "auto"),
            dry_run=True,
            allow_paid=bool(args.get("allow_paid", False)),
            max_credits=args.get("max_credits"),
            language=str(args.get("language") or "en"),
            offline=bool(args.get("offline", False)),
        ),
        "lovart_generate": lambda: generate_command(
            str(args["model"]),
            _body(args),
            mode=str(args.get("mode") or "auto"),
            allow_paid=bool(args.get("allow_paid", False)),
            max_credits=args.get("max_credits"),
            language=str(args.get("language") or "en"),
            wait=bool(args.get("wait", False)),
            download=bool(args.get("download", False)),
            offline=bool(args.get("offline", False)),
        ),
        "lovart_jobs_quote": lambda: jobs_quote_command(
            Path(str(args["jobs_file"])),
            out_dir=_path(args.get("out_dir")),
            language=str(args.get("language") or "en"),
            detail=str(args.get("detail") or "summary"),
            concurrency=int(args.get("concurrency") or 2),
            limit=args.get("limit", "auto"),
            all_requests=bool(args.get("all", False)),
            refresh=bool(args.get("refresh", False)),
            progress=bool(args.get("progress", False)),
        ),
        "lovart_jobs_quote_status": lambda: jobs_quote_status_command(Path(str(args["run_dir"])), jobs_file=_path(args.get("jobs_file"))),
        "lovart_jobs_dry_run": lambda: jobs_dry_run_command(
            Path(str(args["jobs_file"])),
            out_dir=_path(args.get("out_dir")),
            allow_paid=bool(args.get("allow_paid", False)),
            max_total_credits=args.get("max_total_credits"),
            language=str(args.get("language") or "en"),
        ),
        "lovart_jobs_run": lambda: jobs_run_command(
            Path(str(args["jobs_file"])),
            out_dir=_path(args.get("out_dir")),
            allow_paid=bool(args.get("allow_paid", False)),
            max_total_credits=args.get("max_total_credits"),
            language=str(args.get("language") or "en"),
            wait=bool(args.get("wait", False)),
            download=bool(args.get("download", False)),
            timeout_seconds=float(args.get("timeout_seconds") or 3600),
            poll_interval=float(args.get("poll_interval") or 5),
        ),
        "lovart_jobs_status": lambda: jobs_status_command(Path(str(args["run_dir"]))),
        "lovart_jobs_resume": lambda: jobs_resume_command(
            Path(str(args["jobs_file"])),
            out_dir=_path(args.get("out_dir")),
            allow_paid=bool(args.get("allow_paid", False)),
            max_total_credits=args.get("max_total_credits"),
            language=str(args.get("language") or "en"),
            wait=bool(args.get("wait", False)),
            download=bool(args.get("download", False)),
            retry_failed=bool(args.get("retry_failed", False)),
            timeout_seconds=float(args.get("timeout_seconds") or 3600),
            poll_interval=float(args.get("poll_interval") or 5),
        ),
    }
    if name not in handlers:
        raise LovartError("input_error", "unknown MCP tool", {"tool": name})
    return handlers[name]()


def call_tool_envelope(name: str, arguments: dict[str, Any] | None = None) -> dict[str, Any]:
    try:
        return ok(call_tool_data(name, arguments))
    except LovartError as exc:
        return fail(exc)
    except Exception as exc:
        return fail(LovartError("internal_error", str(exc), {"type": exc.__class__.__name__}))


def _tool_result(envelope: dict[str, Any]) -> dict[str, Any]:
    return {
        "content": [{"type": "text", "text": json.dumps(envelope, ensure_ascii=False, separators=(",", ":"))}],
        "isError": not bool(envelope.get("ok")),
    }


def _response(message_id: Any, result: Any = None, error: dict[str, Any] | None = None) -> dict[str, Any]:
    response: dict[str, Any] = {"jsonrpc": "2.0", "id": message_id}
    if error is not None:
        response["error"] = error
    else:
        response["result"] = result or {}
    return response


def handle_message(message: dict[str, Any]) -> dict[str, Any] | None:
    method = message.get("method")
    message_id = message.get("id")
    if message_id is None:
        return None
    if method == "initialize":
        return _response(
            message_id,
            {
                "protocolVersion": PROTOCOL_VERSION,
                "capabilities": {"tools": {"listChanged": False}},
                "serverInfo": {"name": "lovart-reverse", "version": str(version_command().get("version") or "unknown")},
            },
        )
    if method == "ping":
        return _response(message_id, {})
    if method == "tools/list":
        return _response(message_id, {"tools": list_tools()})
    if method == "tools/call":
        params = message.get("params") or {}
        name = str(params.get("name") or "")
        arguments = params.get("arguments") or {}
        if not isinstance(arguments, dict):
            return _response(message_id, error={"code": -32602, "message": "arguments must be an object"})
        return _response(message_id, _tool_result(call_tool_envelope(name, arguments)))
    return _response(message_id, error={"code": -32601, "message": f"unknown method: {method}"})


def main() -> int:
    for line in sys.stdin:
        if not line.strip():
            continue
        try:
            message = json.loads(line)
            response = handle_message(message)
        except Exception as exc:
            response = _response(None, error={"code": -32603, "message": str(exc)})
        if response is not None:
            sys.stdout.write(json.dumps(response, ensure_ascii=False, separators=(",", ":")) + "\n")
            sys.stdout.flush()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
