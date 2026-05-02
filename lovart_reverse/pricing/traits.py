"""Request trait extraction for pricing and entitlement."""

from __future__ import annotations

from typing import Any


def size_bucket(size: Any, body: dict[str, Any]) -> str | None:
    text = str(size or body.get("resolution") or body.get("quality") or "")
    lower = text.lower()
    if "512" in lower:
        return "512P"
    if "4k" in lower or "4096" in lower or "3840" in lower or "2160" in lower:
        return "4K"
    if "2k" in lower or "2048" in lower or "1536" in lower:
        return "2K"
    if "1k" in lower or "1024" in lower:
        return "1K"
    if "1080" in lower:
        return "1080p"
    if "768" in lower:
        return "768p"
    if "720" in lower:
        return "720p"
    if "540" in lower:
        return "540p"
    if "480" in lower:
        return "480p"
    return None


def bucket_rank(bucket: str | None) -> int | None:
    if not bucket:
        return None
    return {
        "512p": 0,
        "480p": 0,
        "540p": 1,
        "720p": 2,
        "768p": 2,
        "1k": 3,
        "1080p": 3,
        "2k": 4,
        "4k": 5,
    }.get(bucket.lower())


def has_reference(body: dict[str, Any]) -> bool:
    for key in ("image", "images", "image_url", "image_tail", "video", "video_url", "video_list"):
        if body.get(key):
            return True
    return False


def quality(body: dict[str, Any]) -> str | None:
    value = body.get("quality") or body.get("mode")
    return str(value).lower() if value is not None else None


def duration_seconds(body: dict[str, Any]) -> int:
    value = body.get("duration") or body.get("duration_seconds") or body.get("seconds") or 5
    try:
        return int(value)
    except (TypeError, ValueError):
        return 5
