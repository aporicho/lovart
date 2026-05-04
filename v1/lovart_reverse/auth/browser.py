"""Proxy browser launcher for Lovart capture sessions."""

from __future__ import annotations

import platform
import subprocess
import time
from pathlib import Path

from lovart_reverse.paths import ROOT

CHROME_PROFILE = ROOT / ".lovart-chrome-profile"
POWERSHELL = "/mnt/c/Windows/System32/WindowsPowerShell/v1.0/powershell.exe"


class BrowserSession:
    def __init__(self, proc: subprocess.Popen | None = None, win_pid: int | None = None):
        self._proc = proc
        self._win_pid = win_pid

    def is_closed(self) -> bool:
        if self._proc is not None:
            return self._proc.poll() is not None
        if self._win_pid is not None:
            result = subprocess.run(
                [POWERSHELL, "-Command", f"Get-Process -Id {self._win_pid} -ErrorAction SilentlyContinue"],
                capture_output=True,
            )
            return not result.stdout.strip()
        return True

    def close(self) -> None:
        if self._proc is not None and self._proc.poll() is None:
            self._proc.terminate()
        elif self._win_pid is not None:
            subprocess.run(
                [POWERSHELL, "-Command", f"Stop-Process -Id {self._win_pid} -Force -ErrorAction SilentlyContinue"],
                capture_output=True,
            )

    def wait(self, interval: float = 1.0) -> None:
        while not self.is_closed():
            time.sleep(interval)


def open_browser(url: str = "https://www.lovart.ai", proxy_port: int = 8080) -> BrowserSession:
    CHROME_PROFILE.mkdir(exist_ok=True)
    proxy = f"http=127.0.0.1:{proxy_port};https=127.0.0.1:{proxy_port}"
    system = platform.system()
    if system == "Darwin":
        chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
        proc = subprocess.Popen(
            [chrome, "--no-first-run", "--new-window", f"--proxy-server={proxy}", f"--user-data-dir={CHROME_PROFILE}", url],
            stdout=subprocess.DEVNULL,
            stderr=subprocess.DEVNULL,
        )
        return BrowserSession(proc=proc)
    if system == "Linux" and Path(POWERSHELL).exists():
        result = subprocess.run(
            [
                POWERSHELL,
                "-Command",
                '$p = Start-Process "C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe"'
                f' -ArgumentList "--no-first-run --new-window --proxy-server={proxy}'
                ' --user-data-dir=C:\\LovartChromeProxy'
                f' {url}" -PassThru; Write-Output $p.Id',
            ],
            capture_output=True,
            text=True,
        )
        return BrowserSession(win_pid=int(result.stdout.strip()))
    for chrome in ("google-chrome", "google-chrome-stable", "chromium", "chromium-browser"):
        try:
            proc = subprocess.Popen(
                [chrome, "--no-first-run", "--new-window", f"--proxy-server={proxy}", f"--user-data-dir={CHROME_PROFILE}", url],
                stdout=subprocess.DEVNULL,
                stderr=subprocess.DEVNULL,
            )
            return BrowserSession(proc=proc)
        except FileNotFoundError:
            continue
    raise RuntimeError("Chrome/Chromium not found")
