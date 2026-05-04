"""One-command browser capture session launcher."""

from __future__ import annotations

import os
import platform
import shutil
import subprocess
import sys
import time
from pathlib import Path
from typing import Any

from lovart_reverse.capture.runtime import reverse_extra_status
from lovart_reverse.errors import InputError
from lovart_reverse.paths import CAPTURES_DIR, PACKAGE_DIR

DEFAULT_CAPTURE_URL = "https://www.lovart.ai/canvas"


def default_profile_dir() -> Path:
    return Path(os.environ.get("TMPDIR") or "/tmp") / "lovart-capture-profile"


def find_chrome(explicit: Path | None = None) -> Path | None:
    if explicit:
        return explicit
    system = platform.system().lower()
    candidates: list[str | None]
    if system == "darwin":
        candidates = [
            "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
            str(Path.home() / "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
            shutil.which("google-chrome"),
            shutil.which("chromium"),
        ]
    elif system == "windows":
        local = os.environ.get("LOCALAPPDATA")
        program_files = os.environ.get("PROGRAMFILES")
        program_files_x86 = os.environ.get("PROGRAMFILES(X86)")
        candidates = [
            str(Path(local) / "Google/Chrome/Application/chrome.exe") if local else None,
            str(Path(program_files) / "Google/Chrome/Application/chrome.exe") if program_files else None,
            str(Path(program_files_x86) / "Google/Chrome/Application/chrome.exe") if program_files_x86 else None,
            shutil.which("chrome"),
            shutil.which("chrome.exe"),
        ]
    else:
        candidates = [
            shutil.which("google-chrome"),
            shutil.which("google-chrome-stable"),
            shutil.which("chromium"),
            shutil.which("chromium-browser"),
        ]
    for candidate in candidates:
        if candidate and Path(candidate).exists():
            return Path(candidate)
    return None


def capture_session_plan(
    port: int = 8080,
    url: str = DEFAULT_CAPTURE_URL,
    profile_dir: Path | None = None,
    browser: Path | None = None,
    open_browser: bool = True,
) -> dict[str, Any]:
    status = reverse_extra_status()
    if not status["available"]:
        raise InputError(
            "reverse start requires optional reverse dependencies",
            {
                "reverse_extra": status,
                "recommended_actions": [
                    'install a maintainer environment with "lovart-reverse[reverse]"',
                    "run lovart reverse capture for the low-level command if mitmdump is installed elsewhere",
                ],
            },
        )
    addon = PACKAGE_DIR / "capture" / "mitm_addon.py"
    profile = profile_dir or default_profile_dir()
    mitmdump = status["mitmdump"] or "mitmdump"
    browser_path = find_chrome(browser) if open_browser else None
    if open_browser and not browser_path:
        raise InputError(
            "Google Chrome was not found",
            {
                "recommended_actions": [
                    "install Google Chrome",
                    "pass --browser /absolute/path/to/chrome",
                    "or pass --no-browser and open Lovart manually through the proxy",
                ]
            },
        )
    browser_command = None
    if open_browser and browser_path:
        browser_command = [
            str(browser_path),
            f"--user-data-dir={profile}",
            f"--proxy-server=http://127.0.0.1:{port}",
            "--no-first-run",
            "--no-default-browser-check",
            "--new-window",
            url,
        ]
    return {
        "port": port,
        "proxy": f"http://127.0.0.1:{port}",
        "captures_dir": str(CAPTURES_DIR),
        "profile_dir": str(profile),
        "url": url,
        "mitmdump_command": [mitmdump, "-s", str(addon), "--listen-port", str(port)],
        "browser_command": browser_command,
    }


def run_capture_session(
    port: int = 8080,
    url: str = DEFAULT_CAPTURE_URL,
    profile_dir: Path | None = None,
    browser: Path | None = None,
    open_browser: bool = True,
    dry_run: bool = False,
) -> dict[str, Any]:
    plan = capture_session_plan(port=port, url=url, profile_dir=profile_dir, browser=browser, open_browser=open_browser)
    if dry_run:
        plan["dry_run"] = True
        return plan

    CAPTURES_DIR.mkdir(parents=True, exist_ok=True)
    # Clean up previous capture files before starting a new session.
    cleaned = 0
    for old in CAPTURES_DIR.glob("*.json"):
        old.unlink()
        cleaned += 1
    before: set[str] = set()
    sys.stderr.write(f"lovart reverse start: proxy {plan['proxy']}\n")
    sys.stderr.write(f"lovart reverse start: cleaned {cleaned} previous captures\n")
    sys.stderr.write(f"lovart reverse start: captures {plan['captures_dir']}\n")

    mitmdump_env = os.environ.copy()
    # Ensure mitmdump's Python can find the lovart_reverse package (the addon imports from it).
    py_root = str(PACKAGE_DIR.parent.resolve())
    existing = mitmdump_env.get("PYTHONPATH", "")
    mitmdump_env["PYTHONPATH"] = f"{py_root}:{existing}" if existing else py_root

    mitm = subprocess.Popen(
        plan["mitmdump_command"],
        stdout=sys.stderr,
        stderr=sys.stderr,
        env=mitmdump_env,
        cwd=py_root,
    )
    time.sleep(1.0)
    if mitm.poll() is not None:
        raise InputError("mitmdump exited during startup", {"returncode": mitm.returncode, "proxy": plan["proxy"]})

    browser_started = False
    browser_proc = None
    if plan["browser_command"]:
        Path(plan["profile_dir"]).mkdir(parents=True, exist_ok=True)
        browser_proc = subprocess.Popen(plan["browser_command"], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        browser_started = True
        sys.stderr.write("lovart reverse start: Chrome launched with capture proxy\n")
    else:
        sys.stderr.write(f"lovart reverse start: open {url} manually through {plan['proxy']}\n")
    sys.stderr.write("lovart reverse start: press Ctrl-C here when the browser test is done\n")
    sys.stderr.write("lovart reverse start: Chrome will be closed automatically\n")

    interrupted = False
    try:
        returncode = mitm.wait()
    except KeyboardInterrupt:
        interrupted = True
        mitm.terminate()
        try:
            returncode = mitm.wait(timeout=5)
        except subprocess.TimeoutExpired:
            mitm.kill()
            returncode = mitm.wait(timeout=5)
    finally:
        # Always close the browser when the capture session ends.
        if browser_proc is not None and browser_proc.poll() is None:
            browser_proc.terminate()
            try:
                browser_proc.wait(timeout=3)
            except subprocess.TimeoutExpired:
                browser_proc.kill()
            sys.stderr.write("lovart reverse start: Chrome closed\n")

    after = sorted(CAPTURES_DIR.glob("*.json"), key=lambda path: path.stat().st_mtime)
    new_files = [path.name for path in after]
    return {
        "stopped": True,
        "interrupted": interrupted,
        "returncode": returncode,
        "browser_started": browser_started,
        "proxy": plan["proxy"],
        "captures_dir": plan["captures_dir"],
        "captured_count": len(new_files),
        "captured_files": new_files[-50:],
        "next_actions": [
            "inspect the captured submit, pricing, task polling, and artifact download requests",
            "run lovart auth extract captures/<lovart-request>.json if credentials changed",
        ],
    }
