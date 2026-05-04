"""Asset upload is intentionally gated until reverse evidence exists."""

from __future__ import annotations


def upload_status() -> dict[str, object]:
    return {
        "implemented": False,
        "reason": "upload endpoints are not confirmed by capture evidence yet",
        "next_step": "capture a real Lovart upload flow before stabilizing this API",
    }
