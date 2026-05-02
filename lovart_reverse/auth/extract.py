"""Extract Lovart authentication headers from captured requests."""

from __future__ import annotations

import json
from pathlib import Path
from typing import Any

from lovart_reverse.auth.store import save_credentials
from lovart_reverse.paths import CAPTURES_DIR

AUTH_HEADER_NAMES = {
    "authorization",
    "cookie",
    "token",
    "x-csrf-token",
    "x-xsrf-token",
    "csrf-token",
    "x-auth-token",
    "x-access-token",
    "x-session-id",
    "x-requested-with",
}

AUTH_HEADER_HINTS = ("auth", "token", "csrf", "session", "lovart", "workspace", "project")

ID_KEYS = {
    "user_id",
    "userId",
    "uid",
    "workspace_id",
    "workspaceId",
    "project_id",
    "projectId",
    "team_id",
    "teamId",
    "organization_id",
    "organizationId",
}


def _is_lovart(value: str) -> bool:
    return "lovart" in value.lower()


def _maybe_auth_header(name: str) -> bool:
    key = name.lower()
    return key in AUTH_HEADER_NAMES or key.startswith("x-") and any(hint in key for hint in AUTH_HEADER_HINTS)


def _walk_ids(value: Any, out: dict[str, Any]) -> None:
    if isinstance(value, dict):
        for key, item in value.items():
            if key in ID_KEYS and item not in (None, ""):
                out.setdefault(key, item)
            _walk_ids(item, out)
    elif isinstance(value, list):
        for item in value:
            _walk_ids(item, out)


def _jsonish(value: Any) -> Any:
    if not isinstance(value, str) or not value.strip():
        return value
    try:
        return json.loads(value)
    except json.JSONDecodeError:
        return value


def extract_from_captures(captures_dir: Path = CAPTURES_DIR) -> dict[str, Any]:
    files = sorted(captures_dir.glob("*.json"), key=lambda path: path.stat().st_mtime, reverse=True)
    headers: dict[str, str] = {}
    ids: dict[str, Any] = {}
    source: str | None = None
    for path in files:
        try:
            data = json.loads(path.read_text())
        except Exception:
            continue
        if not (_is_lovart(str(data.get("url", ""))) or _is_lovart(str(data.get("host", "")))):
            continue
        for name, value in (data.get("request_headers") or {}).items():
            if isinstance(value, str) and value and _maybe_auth_header(str(name)):
                headers.setdefault(str(name), value)
        _walk_ids(_jsonish(data.get("request_body")), ids)
        _walk_ids(data.get("response_body"), ids)
        if headers and source is None:
            source = path.name
        if any(name.lower() in {"authorization", "token", "x-auth-token", "x-access-token"} for name in headers):
            break
    if not headers:
        return {"saved": False, "header_names": [], "id_keys": [], "source_capture": None}
    save_credentials(headers, ids, source)
    return {
        "saved": True,
        "header_names": sorted(headers.keys()),
        "id_keys": sorted(ids.keys()),
        "source_capture": source,
    }


def extract_from_capture(path: Path) -> dict[str, Any]:
    data = json.loads(path.read_text())
    request = data.get("request") if isinstance(data.get("request"), dict) else data
    headers: dict[str, str] = {}
    ids: dict[str, Any] = {}
    for name, value in (request.get("headers") or data.get("request_headers") or {}).items():
        if isinstance(value, str) and value and _maybe_auth_header(str(name)):
            headers[str(name)] = value
    _walk_ids(_jsonish(request.get("body") or data.get("request_body")), ids)
    _walk_ids(data.get("response_body") or (data.get("response") or {}).get("body"), ids)
    if not headers:
        return {"saved": False, "header_names": [], "id_keys": [], "source_capture": path.name}
    save_credentials(headers, ids, path.name)
    return {"saved": True, "header_names": sorted(headers.keys()), "id_keys": sorted(ids.keys()), "source_capture": path.name}
