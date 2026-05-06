"""Read Lovart generator catalog and OpenAPI schema."""

from __future__ import annotations

import sys
from typing import Any

from lovart_reverse.http import lgw_request
from lovart_reverse.io_json import read_json
from lovart_reverse.paths import GENERATOR_LIST_FILE, GENERATOR_SCHEMA_FILE


def _unwrap(payload: dict[str, Any]) -> dict[str, Any]:
    data = payload.get("data")
    return data if isinstance(data, dict) else payload


def generator_list(biz_type: int = 16, language: str = "zh", live: bool = True) -> dict[str, Any]:
    if live:
        try:
            return _unwrap(lgw_request("GET", "/v1/generator/list", params={"biz_type": biz_type}, language=language).json())
        except Exception as exc:
            print(f"warning: remote generator list fetch failed, falling back to ref: {exc}", file=sys.stderr)
    return _unwrap(read_json(GENERATOR_LIST_FILE))


def generator_schema(biz_type: int = 16, language: str = "zh", live: bool = True) -> dict[str, Any]:
    if live:
        try:
            return _unwrap(lgw_request("GET", "/v1/generator/schema", params={"biz_type": biz_type}, language=language).json())
        except Exception as exc:
            print(f"warning: remote generator schema fetch failed, falling back to ref: {exc}", file=sys.stderr)
    return _unwrap(read_json(GENERATOR_SCHEMA_FILE))
