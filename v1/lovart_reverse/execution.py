"""Execution metadata helpers for CLI and MCP command results."""

from __future__ import annotations

from typing import Any

LOCAL = "local"
PREFLIGHT = "preflight"
SUBMIT = "submit"


def annotate(
    data: dict[str, Any],
    execution_class: str,
    *,
    network_required: bool,
    remote_write: bool,
    submitted: bool | None = None,
    cache_used: bool | None = None,
) -> dict[str, Any]:
    result = dict(data)
    result["execution_class"] = execution_class
    result["network_required"] = network_required
    result["remote_write"] = remote_write
    if submitted is not None:
        result["submitted"] = submitted
    if cache_used is not None:
        result["cache_used"] = cache_used
    return result
