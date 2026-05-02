"""Submit Lovart LGW model requests."""

from __future__ import annotations

from typing import Any

from lovart_reverse.http.client import lgw_request


def dry_run_request(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    path = "/" + model.strip("/")
    return {
        "method": "POST",
        "path": path,
        "language": language,
        "body": body,
        "signature_required": True,
    }


def submit_model(model: str, body: dict[str, Any], language: str = "en") -> dict[str, Any]:
    response = lgw_request("POST", "/" + model.strip("/"), body=body, language=language)
    return response.json()
