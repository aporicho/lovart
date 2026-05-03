"""Runtime helpers for optional reverse-capture tooling."""

from __future__ import annotations

import importlib.util
import shutil
from pathlib import Path
from typing import Any

from lovart_reverse.errors import InputError


def reverse_extra_status() -> dict[str, Any]:
    mitmproxy_module = importlib.util.find_spec("mitmproxy") is not None
    mitmdump = shutil.which("mitmdump")
    return {
        "available": bool(mitmproxy_module and mitmdump),
        "mitmproxy_module": mitmproxy_module,
        "mitmdump": mitmdump,
    }


def reverse_extra_available() -> bool:
    return bool(reverse_extra_status()["available"])


def capture_command(port: int, addon: Path) -> dict[str, Any]:
    status = reverse_extra_status()
    if not status["available"]:
        raise InputError(
            "reverse capture requires optional reverse dependencies",
            {
                "reverse_extra": status,
                "recommended_actions": [
                    'install a maintainer environment with "lovart-reverse[reverse]"',
                    "use an existing reverse-maintenance checkout that has mitmdump available",
                ],
            },
        )
    return {
        "command": ["mitmdump", "-s", str(addon), "--listen-port", str(port)],
        "proxy": f"http://127.0.0.1:{port}",
        "note": "run this command in a separate shell and browse Lovart through the proxy",
    }
