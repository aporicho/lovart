"""Shared preflight checks for Lovart generation."""

from __future__ import annotations

from typing import Any

from lovart_reverse.auth.store import status as auth_status
from lovart_reverse.errors import (
    AuthMissingError,
    CreditRiskError,
    LovartError,
    MetadataStaleError,
    SchemaInvalidError,
    SignerStaleError,
    UnknownPricingError,
)
from lovart_reverse.generation.gate import generation_gate
from lovart_reverse.pricing.table import PriceRow, fetch_pricing_rows
from lovart_reverse.registry import load_ref_registry, validate_body
from lovart_reverse.setup.service import _offline_update_status
from lovart_reverse.update import check_update


RISKY_UPDATE_KEYS = {"generator_schema", "pricing", "entitlements"}


def _update_status(live: bool) -> dict[str, Any]:
    if not live:
        return _offline_update_status()
    try:
        return check_update()
    except Exception as exc:
        return {
            "status": "error",
            "changes": {},
            "signer_maybe_stale": True,
            "recommended_actions": ["run lovart update check", "rerun lovart setup when network is available"],
            "error": {"type": exc.__class__.__name__, "message": str(exc)},
        }


def _gate_result(
    model: str,
    body: dict[str, Any],
    rows: list[PriceRow],
    mode: str,
    allow_paid: bool,
    max_credits: float | None,
    live: bool,
) -> tuple[dict[str, Any], LovartError | None]:
    try:
        return generation_gate(model, body, rows, mode=mode, allow_paid=allow_paid, max_credits=max_credits, live=live), None
    except (UnknownPricingError, CreditRiskError) as exc:
        return {"allowed": False, "reason": exc.code, "error": {"code": exc.code, "message": exc.message, "details": exc.details}}, exc


def generation_preflight(
    model: str,
    body: dict[str, Any],
    mode: str,
    allow_paid: bool,
    max_credits: float | None,
    live: bool = True,
) -> tuple[dict[str, Any], LovartError | None]:
    auth = auth_status()
    update = _update_status(live=live)
    schema_errors = validate_body(load_ref_registry(), model, body)
    rows = fetch_pricing_rows(live=live)
    gate, gate_error = _gate_result(model, body, rows, mode, allow_paid, max_credits, live=live)
    changes = update.get("changes") or {}
    recommended_actions = list(update.get("recommended_actions") or [])
    blocking_error: LovartError | None = None
    if not auth.get("exists") or not auth.get("header_names"):
        blocking_error = AuthMissingError(
            "Lovart authentication is missing; capture browser traffic and extract auth first",
            {"recommended_actions": ["run lovart reverse capture", "run lovart auth extract <capture.json>"]},
        )
    elif update.get("signer_maybe_stale"):
        blocking_error = SignerStaleError(
            "Lovart frontend or signer changed; real generation is disabled until signing is revalidated",
            {"update": update, "recommended_actions": recommended_actions},
        )
    elif update.get("status") not in {"fresh", "offline_cached"}:
        blocking_error = MetadataStaleError(
            "Lovart metadata is not fresh enough for real generation",
            {"update": update, "recommended_actions": recommended_actions},
        )
    elif any(changes.get(key) for key in RISKY_UPDATE_KEYS):
        blocking_error = MetadataStaleError(
            "Lovart schema, pricing, or entitlement metadata changed",
            {"update": update, "recommended_actions": recommended_actions},
        )
    elif schema_errors:
        blocking_error = SchemaInvalidError("request body does not match model schema", {"schema_errors": schema_errors})
    elif gate_error:
        blocking_error = gate_error
    if blocking_error and not recommended_actions:
        recommended_actions = list(blocking_error.details.get("recommended_actions", []))
    preflight = {
        "auth": auth,
        "update": update,
        "schema_errors": schema_errors,
        "gate": gate,
        "can_submit": blocking_error is None,
        "blocking_error": _error_payload(blocking_error),
        "recommended_actions": recommended_actions,
    }
    return preflight, blocking_error


def _error_payload(error: LovartError | None) -> dict[str, Any] | None:
    if error is None:
        return None
    return {"code": error.code, "message": error.message, "details": error.details}
