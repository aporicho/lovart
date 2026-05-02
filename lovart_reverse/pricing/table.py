"""Lovart pricing table parsing."""

from __future__ import annotations

import json
import re
import sys
from dataclasses import asdict, dataclass
from pathlib import Path
from typing import Any

from lovart_reverse.http.client import WWW_BASE, www_session
from lovart_reverse.paths import CAPTURES_DIR, PRICING_TABLE_FILE

PRICING_CONFIG_ID = 78


@dataclass(frozen=True)
class PriceRow:
    model_name: str
    unit_price_num: float
    unit_price_text: str
    category: str = ""

    def to_dict(self) -> dict[str, object]:
        return asdict(self)


def _number_from_text(text: str) -> float | None:
    match = re.search(r"(\d+(?:\.\d+)?)", text)
    return float(match.group(1)) if match else None


def parse_pricing_payload(payload: Any) -> list[PriceRow]:
    rows: list[PriceRow] = []

    def walk(node: Any, category: str = "") -> None:
        if isinstance(node, dict):
            next_category = category
            if isinstance(node.get("type"), str):
                next_category = node["type"]
            if "modelName" in node and ("unitPriceNum" in node or "unitPrice" in node):
                model_name = node.get("modelName")
                if isinstance(model_name, dict):
                    model_name = model_name.get("en") or model_name.get("zh")
                unit_price = node.get("unitPrice")
                if isinstance(unit_price, dict):
                    unit_price = unit_price.get("en") or unit_price.get("zh") or ""
                try:
                    unit_price_num = float(node.get("unitPriceNum"))
                except (TypeError, ValueError):
                    unit_price_num = _number_from_text(str(unit_price))
                if model_name and unit_price_num is not None:
                    rows.append(PriceRow(str(model_name), float(unit_price_num), str(unit_price or ""), next_category))
            for value in node.values():
                walk(value, next_category)
        elif isinstance(node, list):
            for item in node:
                walk(item, category)

    walk(payload)
    return dedupe_rows(rows)


def dedupe_rows(rows: list[PriceRow]) -> list[PriceRow]:
    seen: set[tuple[str, float, str]] = set()
    result: list[PriceRow] = []
    for row in rows:
        key = (row.model_name, row.unit_price_num, row.unit_price_text)
        if key not in seen:
            seen.add(key)
            result.append(row)
    return result


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
            print(f"warning: live pricing fetch failed, falling back to local data: {exc}", file=sys.stderr)

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


def fetch_pricing_rows(live: bool = True) -> list[PriceRow]:
    return parse_pricing_payload(fetch_pricing_payload(live=live))


def rows_as_json(rows: list[PriceRow]) -> list[dict[str, object]]:
    return [row.to_dict() for row in rows]
