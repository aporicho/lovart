"""Runtime helpers for optional reverse-capture tooling."""

from __future__ import annotations

import importlib.util
import shutil
import sys
from pathlib import Path
from typing import Any

from lovart_reverse.errors import InputError


def _mitmdump_path() -> str | None:
    direct = shutil.which("mitmdump")
    if direct:
        return direct
    executable_dir = Path(sys.executable).parent
    names = ["mitmdump.exe", "mitmdump"] if sys.platform.startswith("win") else ["mitmdump"]
    for name in names:
        candidate = executable_dir / name
        if candidate.exists():
            return str(candidate)
    return None


def reverse_extra_status() -> dict[str, Any]:
    mitmproxy_module = importlib.util.find_spec("mitmproxy") is not None
    mitmdump = _mitmdump_path()
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
