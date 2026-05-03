"""Live generator pricing quotes from Lovart LGW."""

from __future__ import annotations

from typing import Any

from lovart_reverse.http.client import lgw_request


def _float_or_none(value: Any) -> float | None:
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _listed_credits(price_detail: Any) -> float | None:
    if not isinstance(price_detail, dict):
        return None
    return _float_or_none(price_detail.get("total_price"))


def quote(model: str, body: dict[str, Any], *, language: str = "en") -> dict[str, Any]:
    """Ask Lovart for the exact pre-submit credit quote shown by the web UI."""

    payload = {
        "generator_name": model.strip("/"),
        "input_args": body,
    }
    response = lgw_request("POST", "/v1/generator/pricing", body=payload, language=language, timeout=30)
    data = response.json()
    quoted = data.get("data") if isinstance(data, dict) else None
    if not isinstance(quoted, dict):
        return {"model": model, "quoted": False, "raw": data, "warnings": ["quote response did not contain data"]}
    price = quoted.get("price")
    price_detail = quoted.get("price_detail")
    credits = _float_or_none(price)
    listed_credits = _listed_credits(price_detail)
    return {
        "model": model,
        "quoted": credits is not None,
        "credits": credits,
        "payable_credits": credits,
        "listed_credits": listed_credits,
        "credit_basis": {
            "payable_credits": "data.price",
            "listed_credits": "data.price_detail.total_price",
            "summary_total_credits": "payable_credits",
        },
        "balance": quoted.get("balance"),
        "price": price,
        "price_detail": price_detail,
        "raw": data,
        "warnings": [],
    }


def quote_or_unknown(model: str, body: dict[str, Any], *, live: bool = True, language: str = "en") -> dict[str, Any]:
    """Use the live quote endpoint when available; never estimate spend."""

    if live:
        try:
            result = quote(model, body, language=language)
            if result.get("quoted"):
                return result
        except Exception as exc:
            return {
                "model": model,
                "quoted": False,
                "credits": None,
                "quote_error": {"type": exc.__class__.__name__, "message": str(exc)},
                "warnings": ["live quote failed; credit spend is unknown"],
            }
    return {
        "model": model,
        "quoted": False,
        "credits": None,
        "warnings": ["live quote was disabled; credit spend is unknown"],
    }
