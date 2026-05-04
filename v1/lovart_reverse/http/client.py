"""HTTP client profiles for Lovart endpoints."""

from __future__ import annotations

from typing import Any

import requests

from lovart_reverse.auth import auth_headers
from lovart_reverse.signing import signed_headers, sync_time

WWW_BASE = "https://www.lovart.ai"
CANVA_BASE = "https://www.lovart.ai/api/canva"
LGW_BASE = "https://lgw.lovart.ai"

DYNAMIC_AUTH_HEADERS = {"x-send-timestamp", "x-req-uuid", "x-client-signature"}

DEFAULT_HEADERS = {
    "accept": "application/json, text/plain, */*",
    "origin": "https://www.lovart.ai",
    "referer": "https://www.lovart.ai/",
    "user-agent": (
        "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) "
        "AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36"
    ),
}


def _clean_auth_headers() -> dict[str, str]:
    headers: dict[str, str] = {}
    for name, value in auth_headers().items():
        if name.lower() in DYNAMIC_AUTH_HEADERS:
            continue
        headers[name] = value
    return headers


def www_session(extra_headers: dict[str, str] | None = None) -> requests.Session:
    sess = requests.Session()
    sess.headers.update(DEFAULT_HEADERS)
    sess.headers.update(_clean_auth_headers())
    if extra_headers:
        sess.headers.update(extra_headers)
    return sess


def lgw_session(language: str = "zh", do_sync: bool = True) -> tuple[requests.Session, int]:
    sess = www_session({"accept-language": language})
    offset_ms = sync_time(sess) if do_sync else 0
    return sess, offset_ms


def lgw_request(
    method: str,
    path: str,
    *,
    params: dict[str, Any] | None = None,
    body: dict[str, Any] | None = None,
    language: str = "zh",
    do_sync: bool = True,
    timeout: int = 120,
) -> requests.Response:
    sess, offset_ms = lgw_session(language=language, do_sync=do_sync)
    headers = signed_headers(offset_ms=offset_ms, language=language)
    if body is not None:
        headers["content-type"] = "application/json"
    url = path if path.startswith("http") else f"{LGW_BASE}{path}"
    response = sess.request(method, url, params=params, json=body, headers=headers, timeout=timeout)
    response.raise_for_status()
    return response
