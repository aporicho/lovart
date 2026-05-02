"""Credit estimation for Lovart model requests."""

from __future__ import annotations

import math
import re
from typing import Any

from lovart_reverse.pricing.table import PriceRow
from lovart_reverse.pricing.traits import duration_seconds, has_reference, size_bucket


def _pick_gpt_image_2(rows: list[PriceRow], body: dict[str, Any]) -> tuple[PriceRow | None, list[str]]:
    warnings: list[str] = []
    quality = str(body.get("quality") or "medium").lower()
    bucket = size_bucket(body.get("size"), body) or ("2K" if quality == "medium" else "1K")
    if has_reference(body):
        warnings.append("GPT Image 2 table rows explicitly say 'no ref image'; reference-image pricing is not confirmed.")
    if quality not in {"low", "medium"}:
        return None, [f"no confirmed GPT Image 2 pricing row for quality={quality!r}"]
    candidates = [row for row in rows if row.model_name.lower().startswith("gpt image 2")]
    candidates = [row for row in candidates if bucket.lower() in row.model_name.lower()]
    candidates = [row for row in candidates if quality in row.unit_price_text.lower()]
    if candidates:
        return candidates[0], warnings
    warnings.append(f"no exact GPT Image 2 row for quality={quality!r}, size_bucket={bucket!r}")
    return None, warnings


def _pick_image_row(model: str, rows: list[PriceRow], body: dict[str, Any]) -> tuple[PriceRow | None, list[str]]:
    key = model.strip("/").lower()
    bucket = size_bucket(body.get("size"), body)
    if key == "openai/gpt-image-2":
        return _pick_gpt_image_2(rows, body)
    aliases = {
        "openai/gpt-image-1-5": ["GPT Image 1.5"],
        "vertex/nano-banana-2": ["Nano Banana 2"],
        "vertex/anon-bob": ["Nano Banana Pro"],
        "vertex/nano-banana": ["Nano Banana"],
        "seedream/seedream-4-0": ["Seedream 4.0"],
        "seedream/seedream-4-5": ["Seedream 4.5"],
        "seedream/seedream-5-0": ["Seedream 5.0"],
        "fal/flux-2-pro": ["Flux 2 Pro"],
        "fal/flux-2-max": ["Flux 2 Max"],
        "openai/gpt-image-1": ["GPT Image"],
        "fal/flux-kontext": ["Flux Kontext"],
    }.get(key, [])
    matches = [row for row in rows for alias in aliases if row.model_name.lower().startswith(alias.lower())]
    if bucket:
        bucketed = [row for row in matches if bucket.lower() in row.model_name.lower()]
        if bucketed:
            matches = bucketed
    return (matches[0] if matches else None), ([] if matches else [f"no pricing row mapped for {model}"])


def _unit_seconds(row: PriceRow) -> int:
    match = re.search(r"/(\d+)s", row.unit_price_text)
    return int(match.group(1)) if match else 1


def _pick_video_row(model: str, rows: list[PriceRow], body: dict[str, Any]) -> tuple[PriceRow | None, list[str]]:
    key = model.strip("/").lower()
    display = {
        "seedance/seedance-2-0": "Seedance 2.0",
        "seedance/seedance-2-0-fast": "Seedance 2.0",
        "kling/kling-v3": "Kling 3.0 Pro",
        "kling/kling-v3-omni": "Kling 3.0 Omni",
        "kling/kling-video-o1": "Kling O1",
        "kling/kling-v2-6": "Kling 2.6",
        "kling/kling-v2-5-turbo": "Kling 2.5 Turbo",
        "kling/kling-v2-1": "Kling 2.1",
        "kling/kling-v2-1-master": "Kling 2.1 Master",
        "seedance/seedance-1-5-pro": "Seedance 1.5 Pro",
        "seedance/seedance-1-0-pro": "Seedance 1.0 Pro",
        "openai/sora-2": "Sora 2",
        "openai/sora-2-pro": "Sora 2 Pro",
        "vertex/veo3-1": "Veo 3.1",
        "vertex/veo3-1-fast": "Veo 3.1 Fast",
        "vertex/veo3": "Veo 3",
        "vertex/veo3-fast": "Veo 3 Fast",
        "wan/wan-2-6": "Wan 2.6",
        "vidu/vidu-q2": "Vidu Q2",
        "vidu/vidu-q1": "Vidu Q1",
        "minimax/minimax-hailuo-02": "Hailuo 02",
        "minimax/minimax-hailuo-2-3": "Hailuo 2.3",
        "minimax/minimax-hailuo-2-3-fast": "Hailuo 2.3 Fast",
    }.get(key)
    if not display:
        return None, [f"no video pricing row mapped for {model}"]
    matches = [row for row in rows if row.model_name.lower().startswith(display.lower())]
    bucket = size_bucket(body.get("resolution"), body)
    if bucket:
        bucketed = [row for row in matches if bucket.lower() in row.model_name.lower() or bucket.lower() in row.unit_price_text.lower()]
        if bucketed:
            matches = bucketed
    if has_reference(body):
        ref = [row for row in matches if "reference" in row.model_name.lower() or "reference" in row.unit_price_text.lower()]
        if ref:
            matches = ref
    return (matches[0] if matches else None), ([] if matches else [f"no exact video pricing row for {model}"])


def estimate(model: str, body: dict[str, Any], rows: list[PriceRow]) -> dict[str, Any]:
    model_key = model.strip("/").lower()
    if any(prefix in model_key for prefix in ("kling/", "seedance/", "openai/sora", "vertex/veo", "wan/", "vidu/", "minimax/")):
        row, warnings = _pick_video_row(model, rows, body)
        unit_multiplier = max(1, math.ceil(duration_seconds(body) / _unit_seconds(row))) if row else 1
    else:
        row, warnings = _pick_image_row(model, rows, body)
        unit_multiplier = int(body.get("n") or body.get("count") or 1)
    if not row:
        return {"model": model, "estimated": False, "warnings": warnings}
    credits = row.unit_price_num * unit_multiplier
    notes: list[str] = []
    bucket = size_bucket(body.get("size"), body)
    if model_key == "openai/gpt-image-2" and str(body.get("quality") or "").lower() == "low" and bucket in {"1K", "2K"} and not has_reference(body):
        notes.append("GPT Image 2 low 1K/2K no-reference may be covered by fast zero-credit entitlement; run free check.")
    return {
        "model": model,
        "estimated": True,
        "matched_row": row.to_dict(),
        "unit_multiplier": unit_multiplier,
        "credits": credits,
        "warnings": warnings,
        "notes": notes,
    }
