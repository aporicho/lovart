"""JSON envelope helpers shared by CLI and MCP wrappers."""

from __future__ import annotations

from typing import Any

from lovart_reverse.errors import LovartError

_EXECUTION_KEYS = (
    "execution_class",
    "network_required",
    "remote_write",
    "submitted",
    "cache_used",
)


def ok(data: Any = None, warnings: list[str] | None = None) -> dict[str, Any]:
    payload = data or {}
    envelope = {"ok": True, "data": payload, "warnings": warnings or []}
    if isinstance(payload, dict):
        for key in _EXECUTION_KEYS:
            if key in payload:
                envelope[key] = payload[key]
    return envelope


def fail(error: LovartError) -> dict[str, Any]:
    return {
        "ok": False,
        "error": {
            "code": error.code,
            "message": error.message,
            "details": error.details,
        },
    }
