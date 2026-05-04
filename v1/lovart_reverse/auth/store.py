"""Credential storage for Lovart browser headers."""

from __future__ import annotations

import json
import time
from pathlib import Path
from typing import Any

from lovart_reverse.paths import CREDS_FILE, LEGACY_CREDS_FILE


def credential_path() -> Path:
    if CREDS_FILE.exists() or not LEGACY_CREDS_FILE.exists():
        return CREDS_FILE
    return LEGACY_CREDS_FILE


def load_credentials() -> dict[str, Any]:
    path = credential_path()
    if path.exists():
        return json.loads(path.read_text())
    return {"headers": {}, "ids": {}, "source_capture": None, "updated_at": None}


def save_credentials(headers: dict[str, str], ids: dict[str, Any], source_capture: str | None) -> None:
    CREDS_FILE.parent.mkdir(parents=True, exist_ok=True)
    CREDS_FILE.write_text(
        json.dumps(
            {
                "headers": headers,
                "ids": ids,
                "source_capture": source_capture,
                "updated_at": time.strftime("%Y-%m-%d %H:%M:%S"),
            },
            ensure_ascii=False,
            indent=2,
        )
        + "\n"
    )


def auth_headers() -> dict[str, str]:
    return dict(load_credentials().get("headers", {}))


def saved_ids() -> dict[str, Any]:
    return dict(load_credentials().get("ids", {}))


def status() -> dict[str, Any]:
    data = load_credentials()
    headers = data.get("headers", {})
    return {
        "path": str(credential_path()),
        "exists": credential_path().exists(),
        "updated_at": data.get("updated_at"),
        "source_capture": data.get("source_capture"),
        "header_names": sorted(headers.keys()) if isinstance(headers, dict) else [],
        "id_keys": sorted((data.get("ids") or {}).keys()),
    }
