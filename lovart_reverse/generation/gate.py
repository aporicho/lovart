"""Credit-spend gate for generation commands."""

from __future__ import annotations

from typing import Any

from lovart_reverse.entitlement import free_check
from lovart_reverse.errors import CreditRiskError, UnknownPricingError
from lovart_reverse.pricing.estimator import estimate
from lovart_reverse.pricing.table import PriceRow


def generation_gate(
    model: str,
    body: dict[str, Any],
    rows: list[PriceRow],
    mode: str,
    allow_paid: bool,
    max_credits: float | None,
    live: bool = True,
) -> dict[str, Any]:
    entitlement = free_check(model, body, mode=mode, live=live)
    pricing = estimate(model, body, rows)
    allowed = bool(entitlement.get("zero_credit"))
    reason = "zero_credit_entitlement" if allowed else ""
    if allowed:
        return {"allowed": True, "reason": reason, "entitlement": entitlement, "pricing": pricing}
    if not pricing.get("estimated"):
        raise UnknownPricingError(
            "pricing is unknown; refusing to submit generation",
            {"model": model, "mode": mode, "pricing": pricing, "entitlement": entitlement},
        )
    credits = float(pricing["credits"])
    if not allow_paid:
        raise CreditRiskError(
            "generation may spend credits; pass --allow-paid --max-credits N to allow it",
            {"model": model, "estimated_credits": credits, "entitlement": entitlement, "pricing": pricing},
        )
    if max_credits is None:
        raise CreditRiskError("--max-credits is required with --allow-paid", {"estimated_credits": credits})
    if credits > max_credits:
        raise CreditRiskError(
            "estimated credits exceed --max-credits",
            {"estimated_credits": credits, "max_credits": max_credits},
        )
    return {"allowed": True, "reason": "paid_allowed", "entitlement": entitlement, "pricing": pricing}
