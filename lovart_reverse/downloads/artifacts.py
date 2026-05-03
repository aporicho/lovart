"""Download generation artifacts."""

from __future__ import annotations

from pathlib import Path
from typing import Any
from urllib.parse import urlparse

import requests

from lovart_reverse.paths import DOWNLOADS_DIR


def _artifact_url(item: Any) -> str | None:
    if isinstance(item, str) and item.startswith("http"):
        return item
    if isinstance(item, dict):
        for key in ("url", "src", "downloadUrl", "imageUrl", "videoUrl", "content"):
            value = item.get(key)
            if isinstance(value, str) and value.startswith("http"):
                return value
            nested = _artifact_url(value)
            if nested:
                return nested
    if isinstance(item, list):
        for value in item:
            nested = _artifact_url(value)
            if nested:
                return nested
    return None


def download_artifacts(
    artifacts: list[Any],
    output_dir: Path = DOWNLOADS_DIR,
    task_id: str | None = None,
) -> list[dict[str, Any]]:
    target_dir = output_dir / task_id if task_id else output_dir
    target_dir.mkdir(parents=True, exist_ok=True)
    saved: list[dict[str, Any]] = []
    for index, artifact in enumerate(artifacts, start=1):
        url = _artifact_url(artifact)
        if not url:
            saved.append({"index": index, "saved": False, "reason": "no downloadable URL", "artifact": artifact})
            continue
        suffix = Path(urlparse(url).path).suffix or ".bin"
        path = target_dir / f"artifact-{index}{suffix}"
        response = requests.get(url, timeout=60)
        response.raise_for_status()
        path.write_bytes(response.content)
        saved.append({"index": index, "saved": True, "path": str(path), "url": url})
    return saved
