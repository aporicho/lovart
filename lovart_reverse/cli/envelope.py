"""JSON envelope rendering for CLI commands."""

from __future__ import annotations

from typing import Any

from lovart_reverse.errors import LovartError


def ok(data: Any = None, warnings: list[str] | None = None) -> dict[str, Any]:
    return {"ok": True, "data": data or {}, "warnings": warnings or []}


def fail(error: LovartError) -> dict[str, Any]:
    return {
        "ok": False,
        "error": {
            "code": error.code,
            "message": error.message,
            "details": error.details,
        },
    }
