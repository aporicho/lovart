"""Web-style pricing payload helpers."""

from __future__ import annotations

from math import gcd
from typing import Any


def pricing_input_args(model: str, body: dict[str, Any]) -> tuple[dict[str, Any], bool]:
    """Return pricing input args with internal Lovart web parity metadata."""

    args = dict(body)
    if isinstance(args.get("original_unit_data"), dict):
        return args, False
    original = build_original_unit_data(model, body)
    if not original:
        return args, False
    args["original_unit_data"] = original
    return args, True


def build_original_unit_data(model: str, body: dict[str, Any]) -> dict[str, Any]:
    width, height = _dimensions(body)
    count = _count(body)
    resolution = body.get("resolution") or _resolution_from_size(body.get("size"))
    ratio = body.get("aspect_ratio") or body.get("ratio") or _ratio(width, height)
    quality = body.get("quality") or "medium"
    payload: dict[str, Any] = {
        "title": "Image Generator",
        "name": "Image Generator 1",
        "fillColor": "#E6E6E6",
        "generatorName": model.strip("/"),
        "generator_name": model.strip("/"),
        "quality": quality,
        "count": count,
    }
    if width and height:
        payload.update(
            {
                "w": width,
                "h": height,
                "width": width,
                "height": height,
                "configWidth": width,
                "configHeight": height,
            }
        )
    if ratio:
        payload["ratio"] = ratio
    if resolution:
        payload["resolution"] = resolution
    for key in ("model", "mode", "render_speed", "duration"):
        if key in body:
            payload[key] = body[key]
    if "render_speed" in body:
        payload["renderSpeed"] = body["render_speed"]
    return payload


def _count(body: dict[str, Any]) -> int:
    for key in ("n", "max_images", "count", "image_count"):
        value = body.get(key)
        try:
            count = int(value)
        except (TypeError, ValueError):
            continue
        if count > 0:
            return count
    return 1


def _dimensions(body: dict[str, Any]) -> tuple[int | None, int | None]:
    size = body.get("size")
    parsed = _parse_size(size)
    if parsed:
        return parsed
    ratio = body.get("aspect_ratio") or body.get("ratio")
    resolution = body.get("resolution")
    return _dimensions_from_ratio_resolution(ratio, resolution)


def _parse_size(value: Any) -> tuple[int, int] | None:
    if not isinstance(value, str):
        return None
    separator = "*" if "*" in value else "x" if "x" in value else None
    if not separator:
        return None
    left, right = value.split(separator, 1)
    try:
        width = int(left)
        height = int(right)
    except ValueError:
        return None
    return (width, height) if width > 0 and height > 0 else None


def _dimensions_from_ratio_resolution(ratio: Any, resolution: Any) -> tuple[int | None, int | None]:
    if not isinstance(ratio, str):
        return None, None
    if ":" not in ratio:
        return None, None
    try:
        left, right = ratio.split(":", 1)
        ratio_w = int(left)
        ratio_h = int(right)
    except ValueError:
        return None, None
    if ratio_w <= 0 or ratio_h <= 0:
        return None, None
    base = _resolution_base(resolution)
    if ratio_w >= ratio_h:
        width = base
        height = round(base * ratio_h / ratio_w)
    else:
        height = base
        width = round(base * ratio_w / ratio_h)
    return width, height


def _resolution_base(value: Any) -> int:
    if isinstance(value, str):
        upper = value.upper()
        if upper == "4K":
            return 4096
        if upper == "2K":
            return 2048
        if upper == "1K":
            return 1024
        try:
            return int(upper.rstrip("P"))
        except ValueError:
            return 1024
    return 1024


def _resolution_from_size(value: Any) -> str | None:
    parsed = _parse_size(value)
    if not parsed:
        return None
    width, height = parsed
    longest = max(width, height)
    if longest >= 3000:
        return "4K"
    if longest >= 1800:
        return "2K"
    return "1K"


def _ratio(width: int | None, height: int | None) -> str | None:
    if not width or not height:
        return None
    factor = gcd(width, height)
    return f"{width // factor}:{height // factor}"
