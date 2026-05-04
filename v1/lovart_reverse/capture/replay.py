"""Replay captured Lovart HTTP requests."""

from __future__ import annotations

from pathlib import Path
from typing import Any

import requests

from lovart_reverse.io_json import read_json


HOP_BY_HOP_HEADERS = {
    "host",
    "content-length",
    "connection",
    "accept-encoding",
    "transfer-encoding",
}


def replay_capture(path: Path, submit: bool = False) -> dict[str, Any]:
    capture = read_json(path)
    request = capture.get("request") or capture
    method = str(request.get("method") or "GET").upper()
    url = request.get("url")
    headers = {
        key: value
        for key, value in (request.get("headers") or {}).items()
        if str(key).lower() not in HOP_BY_HOP_HEADERS
    }
    body = request.get("body") or request.get("json")
    preview = {"method": method, "url": url, "headers": sorted(headers.keys()), "has_body": body is not None}
    if not submit:
        return {"submitted": False, "preview": preview}
    response = requests.request(method, url, headers=headers, json=body if isinstance(body, (dict, list)) else None, data=body if isinstance(body, str) else None, timeout=60)
    return {"submitted": True, "preview": preview, "status_code": response.status_code, "response": _response_body(response)}


def _response_body(response: requests.Response) -> Any:
    try:
        return response.json()
    except ValueError:
        return response.text[:2000]
