"""mitmproxy addon that stores Lovart traffic as JSON capture evidence."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from pathlib import Path

from lovart_reverse.paths import CAPTURES_DIR


def _safe_name(text: str) -> str:
    return "".join(ch if ch.isalnum() else "_" for ch in text)[:100].strip("_") or "request"


class LovartCapture:
    def response(self, flow) -> None:  # pragma: no cover - executed by mitmproxy
        host = flow.request.pretty_host
        if "lovart.ai" not in host and "lovart-api" not in host:
            return
        CAPTURES_DIR.mkdir(parents=True, exist_ok=True)
        timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
        name = f"{timestamp}_{_safe_name(flow.request.method + '_' + flow.request.path)}.json"
        path = CAPTURES_DIR / name
        try:
            request_body = json.loads(flow.request.get_text(strict=False) or "null")
        except Exception:
            request_body = flow.request.get_text(strict=False)
        try:
            response_body = json.loads(flow.response.get_text(strict=False) or "null")
        except Exception:
            response_body = flow.response.get_text(strict=False) if flow.response else None
        payload = {
            "captured_at": timestamp,
            "request": {
                "method": flow.request.method,
                "url": flow.request.pretty_url,
                "path": flow.request.path,
                "headers": dict(flow.request.headers),
                "body": request_body,
            },
            "response": {
                "status_code": flow.response.status_code if flow.response else None,
                "headers": dict(flow.response.headers) if flow.response else {},
                "body": response_body,
            },
            "response_body": response_body,
        }
        path.write_text(json.dumps(payload, ensure_ascii=False, indent=2))


addons = [LovartCapture()]
