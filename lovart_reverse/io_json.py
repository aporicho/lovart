"""JSON file and canonical hash operations."""

from __future__ import annotations

import hashlib
import json
from pathlib import Path
from typing import Any


def read_json(path: Path) -> Any:
    return json.loads(path.read_text())


def write_json(path: Path, value: Any) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, ensure_ascii=False, indent=2) + "\n")


def canonical_json(value: Any) -> str:
    return json.dumps(value, ensure_ascii=False, sort_keys=True, separators=(",", ":"))


def hash_value(value: Any) -> str:
    return hashlib.sha256(canonical_json(value).encode("utf-8")).hexdigest()


def hash_bytes(value: bytes) -> str:
    return hashlib.sha256(value).hexdigest()


def load_body(body: str | None, body_file: str | None, default: dict[str, Any] | None = None) -> dict[str, Any]:
    if body_file:
        value = Path(body_file).read_text()
    elif body:
        value = body
    else:
        return dict(default or {})
    data = json.loads(value)
    if not isinstance(data, dict):
        raise ValueError("body must be a JSON object")
    return data
