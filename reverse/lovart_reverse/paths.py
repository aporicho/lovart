"""Filesystem paths used by Lovart reverse capture tooling."""
from __future__ import annotations

import os
from pathlib import Path

PACKAGE_DIR = Path(__file__).resolve().parent


def _repo_root() -> Path | None:
    """Return the repo root if go.mod exists in the parent chain."""
    d = PACKAGE_DIR.parent
    while d != d.parent:
        if (d / "go.mod").exists():
            return d
        d = d.parent
    return None


def _runtime_root() -> Path:
    env_root = os.environ.get("LOVART_REVERSE_ROOT")
    if env_root:
        return Path(env_root).expanduser().resolve()
    repo = _repo_root()
    if repo:
        return repo
    home = Path(os.environ.get("LOVART_REVERSE_HOME", "~/.lovart-reverse")).expanduser()
    return home.resolve()


ROOT = _runtime_root()
CAPTURES_DIR = ROOT / "captures"
RUNTIME_DIR = ROOT / ".lovart"
CREDS_FILE = RUNTIME_DIR / "creds.json"
LEGACY_CREDS_FILE = ROOT / "scripts" / "creds.json"
