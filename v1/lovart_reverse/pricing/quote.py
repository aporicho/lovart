"""Live generator pricing quotes from Lovart LGW."""

from __future__ import annotations

import threading
from typing import Any

import requests

from lovart_reverse.errors import SignatureError
from lovart_reverse.http.client import LGW_BASE, www_session
from lovart_reverse.pricing.web_parity import pricing_input_args
from lovart_reverse.signing import PersistentSigner, sign, signed_headers, sync_time


class QuoteNetworkError(RuntimeError):
    def __init__(self, phase: str, message: str):
        super().__init__(message)
        self.phase = phase


def _float_or_none(value: Any) -> float | None:
    try:
        return float(value)
    except (TypeError, ValueError):
        return None


def _listed_credits(price_detail: Any) -> float | None:
    if not isinstance(price_detail, dict):
        return None
    return _float_or_none(price_detail.get("total_price"))


class QuoteClient:
    """Web-parity Lovart pricing client.

    A client instance mirrors the browser: one session and one time sync, then
    many signed pricing calls.
    """

    def __init__(
        self,
        *,
        language: str = "en",
        session: requests.Session | None = None,
        persistent_signer: bool = False,
        include_original_unit_data: bool = True,
    ):
        self.language = language
        self.session = session or www_session({"accept-language": language})
        self.include_original_unit_data = include_original_unit_data
        self.warnings: list[str] = []
        self._sign_lock = threading.Lock()
        self._persistent_signer: PersistentSigner | None = None
        self._owns_signer = False
        if persistent_signer:
            try:
                self._persistent_signer = PersistentSigner()
                self._owns_signer = True
            except SignatureError as exc:
                self.warnings.append(f"persistent signer unavailable; falling back to one-shot signer: {exc}")
        try:
            self.offset_ms = sync_time(self.session)
        except requests.RequestException as exc:
            raise QuoteNetworkError("timestamp", str(exc)) from exc

    def quote(self, model: str, body: dict[str, Any]) -> dict[str, Any]:
        payload, sent_original_unit_data = self._payload(model, body)
        headers = signed_headers(offset_ms=self.offset_ms, language=self.language, signer=self._sign)
        headers["content-type"] = "application/json"
        try:
            response = self.session.post(f"{LGW_BASE}/v1/generator/pricing", json=payload, headers=headers, timeout=30)
            response.raise_for_status()
        except requests.RequestException as exc:
            raise QuoteNetworkError("pricing", str(exc)) from exc
        return _normalize_quote_response(
            model,
            response.json(),
            request_shape={
                "sent_original_unit_data": sent_original_unit_data,
                "input_arg_keys": sorted(payload["input_args"].keys()),
            },
            warnings=self._drain_warnings(),
        )

    def close(self) -> None:
        if self._owns_signer and self._persistent_signer is not None:
            self._persistent_signer.close()
            self._persistent_signer = None
        self.session.close()

    def __enter__(self) -> "QuoteClient":
        return self

    def __exit__(self, exc_type, exc, tb) -> None:
        self.close()

    def _payload(self, model: str, body: dict[str, Any]) -> tuple[dict[str, Any], bool]:
        input_args: dict[str, Any]
        sent_original_unit_data = False
        if self.include_original_unit_data:
            input_args, sent_original_unit_data = pricing_input_args(model, body)
        else:
            input_args = dict(body)
        return {"generator_name": model.strip("/"), "input_args": input_args}, sent_original_unit_data

    def _sign(self, timestamp: str, req_uuid: str, third: str = "", fourth: str = "") -> str:
        with self._sign_lock:
            if self._persistent_signer is not None:
                try:
                    return self._persistent_signer.sign(timestamp, req_uuid, third, fourth)
                except SignatureError as exc:
                    self.warnings.append(f"persistent signer failed; falling back to one-shot signer: {exc}")
                    if self._owns_signer:
                        self._persistent_signer.close()
                    self._persistent_signer = None
            return sign(timestamp, req_uuid, third, fourth)

    def _drain_warnings(self) -> list[str]:
        warnings = list(self.warnings)
        self.warnings.clear()
        return warnings


def quote(model: str, body: dict[str, Any], *, language: str = "en") -> dict[str, Any]:
    """Ask Lovart for the exact pre-submit credit quote shown by the web UI."""

    with QuoteClient(language=language, persistent_signer=False) as client:
        return client.quote(model, body)


def _normalize_quote_response(
    model: str,
    data: dict[str, Any],
    *,
    request_shape: dict[str, Any] | None = None,
    warnings: list[str] | None = None,
) -> dict[str, Any]:
    quoted = data.get("data") if isinstance(data, dict) else None
    if not isinstance(quoted, dict):
        return {
            "model": model,
            "quoted": False,
            "raw": data,
            "request_shape": request_shape or {},
            "warnings": list(warnings or []) + ["quote response did not contain data"],
        }
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
        "request_shape": request_shape or {},
        "warnings": list(warnings or []),
    }


def quote_or_unknown(model: str, body: dict[str, Any], *, live: bool = True, language: str = "en") -> dict[str, Any]:
    """Use the remote quote endpoint when available; never estimate spend."""

    if live:
        try:
            result = quote(model, body, language=language)
            if result.get("quoted"):
                return result
        except Exception as exc:
            code = "unknown_pricing"
            if isinstance(exc, QuoteNetworkError):
                code = f"{exc.phase}_network_unavailable"
            return {
                "model": model,
                "quoted": False,
                "credits": None,
                "payable_credits": None,
                "listed_credits": None,
                "quote_error": {"type": exc.__class__.__name__, "message": str(exc), "code": code},
                "warnings": ["remote quote failed; credit spend is unknown"],
            }
    return {
        "model": model,
        "quoted": False,
        "credits": None,
        "warnings": ["remote quote was disabled; credit spend is unknown"],
    }
