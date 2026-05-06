"""Raw Lovart pricing metadata for update drift checks."""

from __future__ import annotations

import json
import sys
from typing import Any

from lovart_reverse.http.client import WWW_BASE, www_session
from lovart_reverse.paths import CAPTURES_DIR, PRICING_TABLE_FILE

PRICING_CONFIG_ID = 78


def fetch_pricing_payload(live: bool = True) -> Any:
    if live:
        try:
            sess = www_session({"accept-language": "en"})
            response = sess.get(
                f"{WWW_BASE}/api/www/landing-activities/getById",
                params={"id": PRICING_CONFIG_ID},
                timeout=30,
            )
            response.raise_for_status()
            link_url = response.json().get("data", {}).get("linkUrl")
            if isinstance(link_url, str) and link_url.strip()[:1] in "[{":
                return json.loads(link_url)
        except Exception as exc:
            print(f"warning: remote pricing fetch failed, falling back to local data: {exc}", file=sys.stderr)

    if PRICING_TABLE_FILE.exists():
        return json.loads(PRICING_TABLE_FILE.read_text())

    for path in sorted(CAPTURES_DIR.glob("*landing-activities_getById*.json"), reverse=True):
        try:
            capture = json.loads(path.read_text())
            link_url = capture.get("response_body", {}).get("data", {}).get("linkUrl")
            if isinstance(link_url, str) and link_url.strip()[:1] in "[{" and "unitPriceNum" in link_url:
                return json.loads(link_url)
        except Exception:
            continue
    raise RuntimeError("no pricing table found")
