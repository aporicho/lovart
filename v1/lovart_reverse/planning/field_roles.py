"""Classify schema fields for route planning without inventing values."""

from __future__ import annotations

from typing import Any

COST_AFFECTING_FIELDS = {
    "n",
    "count",
    "max_images",
    "quality",
    "size",
    "resolution",
    "aspect_ratio",
    "ratio",
    "duration",
    "mode",
    "input_fidelity",
    "image",
    "image_url",
    "image_list",
    "start_frame_image",
    "tail_frame_image",
    "video",
    "video_list",
    "audio_list",
}
BATCH_RELEVANT_FIELDS = {"n", "count", "max_images", "prompt", "image", "image_url", "image_list"}
MEDIA_INPUT_FIELDS = {
    "image",
    "image_url",
    "image_list",
    "mask",
    "start_frame_image",
    "tail_frame_image",
    "video",
    "video_list",
    "audio_list",
}
FREE_INPUT_FIELDS = {
    "prompt",
    "negative_prompt",
    "image",
    "image_url",
    "image_list",
    "mask",
    "start_frame_image",
    "tail_frame_image",
    "video",
    "video_list",
    "audio_list",
}
QUALITY_AFFECTING_FIELDS = {
    "quality",
    "size",
    "resolution",
    "aspect_ratio",
    "ratio",
    "mode",
    "input_fidelity",
}
SPEED_AFFECTING_FIELDS = {"mode", "duration"}
QUANTITY_FIELDS = {"n", "count", "max_images"}
FORMAT_ONLY_FIELDS = {"output_format", "background", "moderation", "watermark", "sound", "generate_audio"}


def classify_field(key: str, field: dict[str, Any]) -> dict[str, Any]:
    """Return planner-facing roles derived from schema shape and field name."""

    values = field.get("values")
    lower_values = {str(value).lower() for value in values} if isinstance(values, list) else set()
    has_resolution_mapping = isinstance(field.get("resolution_mapping"), dict)
    enumerable = bool(field.get("enumerable"))
    field_type = str(field.get("type") or "")

    quality_affecting = key in QUALITY_AFFECTING_FIELDS or has_resolution_mapping
    speed_affecting = key in SPEED_AFFECTING_FIELDS or bool(lower_values & {"fast", "turbo", "quick"})
    media_input = key in MEDIA_INPUT_FIELDS
    free_input = key in FREE_INPUT_FIELDS or (not enumerable and field_type in {"string", "array", "object", "unknown"})
    quantity = key in QUANTITY_FIELDS
    cost_affecting = key in COST_AFFECTING_FIELDS or quality_affecting or quantity or media_input
    format_only = key in FORMAT_ONLY_FIELDS and not quality_affecting and not speed_affecting

    if media_input:
        route_role = "media_input"
    elif free_input:
        route_role = "free_input"
    elif quantity:
        route_role = "quantity"
    elif speed_affecting:
        route_role = "speed_affecting"
    elif quality_affecting:
        route_role = "quality_affecting"
    elif cost_affecting:
        route_role = "cost_affecting"
    elif format_only:
        route_role = "format_only"
    else:
        route_role = "other"

    return {
        "quality_affecting": quality_affecting,
        "cost_affecting": cost_affecting,
        "speed_affecting": speed_affecting,
        "batch_relevant": key in BATCH_RELEVANT_FIELDS,
        "media_input": media_input,
        "free_input": free_input,
        "format_only": format_only,
        "route_role": route_role,
    }
