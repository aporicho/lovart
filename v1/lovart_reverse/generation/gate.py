"""Credit-spend gate for generation commands."""

from __future__ import annotations

from typing import Any

from lovart_reverse.entitlement import free_check
from lovart_reverse.errors import CreditRiskError, UnknownPricingError
from lovart_reverse.pricing.quote import quote_or_unknown


def generation_gate(
    model: str,
    body: dict[str, Any],
    mode: str,
    allow_paid: bool,
    max_credits: float | None,
    live: bool = True,
) -> dict[str, Any]:
    entitlement = free_check(model, body, mode=mode, live=live)
    pricing = quote_or_unknown(model, body, live=live)
    if pricing.get("quoted"):
        credits = float(pricing.get("credits") or 0)
        if credits == 0:
            return {"allowed": True, "reason": "quote_zero_credit", "entitlement": entitlement, "pricing": pricing}
        if not allow_paid:
            raise CreditRiskError(
                "generation may spend credits; pass --allow-paid --max-credits N to allow it",
                {"model": model, "quoted_credits": credits, "entitlement": entitlement, "pricing": pricing},
            )
        if max_credits is None:
            raise CreditRiskError("--max-credits is required with --allow-paid", {"quoted_credits": credits})
        if credits > max_credits:
            raise CreditRiskError(
                "quoted credits exceed --max-credits",
                {"quoted_credits": credits, "max_credits": max_credits},
            )
        return {"allowed": True, "reason": "paid_allowed", "entitlement": entitlement, "pricing": pricing}
    if not live and entitlement.get("zero_credit"):
        return {"allowed": True, "reason": "zero_credit_entitlement", "entitlement": entitlement, "pricing": pricing}
    if not pricing.get("quoted"):
        raise UnknownPricingError(
            "pricing is unknown; refusing to submit generation",
            {"model": model, "mode": mode, "pricing": pricing, "entitlement": entitlement},
        )
