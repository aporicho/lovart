"""Lovart account balance and time-variant pricing."""

from __future__ import annotations

from datetime import datetime, timezone
from typing import Any

from lovart_reverse.http.client import WWW_BASE, www_session


def fetch_account() -> dict[str, Any]:
    response = www_session().get(f"{WWW_BASE}/api/www/lovart/member/account", timeout=30)
    response.raise_for_status()
    return response.json()


def fetch_power_detail() -> dict[str, Any]:
    response = www_session().get(f"{WWW_BASE}/api/www/lovart/member/power/detail", timeout=30)
    response.raise_for_status()
    return response.json()


def fetch_time_variant() -> dict[str, Any]:
    response = www_session().get(f"{WWW_BASE}/api/canva/agent-cashier/pricing/timeVariantConfig", timeout=30)
    response.raise_for_status()
    return response.json()


def balance_summary() -> dict[str, Any]:
    account = fetch_account().get("data") or {}
    attr = account.get("attr") or {}
    detail: Any
    try:
        detail = fetch_power_detail().get("data")
    except Exception as exc:
        detail = {"warning": str(exc)}
    return {
        "accountLevel": account.get("accountLevel"),
        "accountLevelDesc": account.get("accountLevelDesc"),
        "usablePower": attr.get("usablePower"),
        "totalPower": attr.get("totalPower"),
        "usedPower": attr.get("usedPower"),
        "taskConcurrent": attr.get("taskConcurrent"),
        "detail": detail,
    }


def time_variant_summary() -> dict[str, Any]:
    cfg = fetch_time_variant().get("data") or {}
    now_ms = int(datetime.now(timezone.utc).timestamp() * 1000) % 86_400_000

    def active(start_key: str, end_key: str, enable_key: str) -> bool:
        if not cfg.get(enable_key):
            return False
        try:
            start = int(cfg[start_key])
            end = int(cfg[end_key])
        except (KeyError, TypeError, ValueError):
            return False
        return start <= now_ms < end if start <= end else now_ms >= start or now_ms < end

    rate = 1.0
    label = "normal"
    if active("offPeakStartTime", "offPeakEndTime", "offPeakEnable"):
        rate = float(cfg.get("offPeakRate") or 1)
        label = "offPeak"
    elif active("peakStartTime", "peakEndTime", "peakEnable"):
        rate = float(cfg.get("peakRate") or 1)
        label = "peak"
    return {"label": label, "rate": rate, "config": cfg}
