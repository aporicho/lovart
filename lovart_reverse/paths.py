"""Project paths used by Lovart reverse tooling."""

from __future__ import annotations

import os
from pathlib import Path


def _project_root() -> Path:
    env_root = os.environ.get("LOVART_REVERSE_ROOT")
    if env_root:
        return Path(env_root).expanduser().resolve()
    cwd = Path.cwd().resolve()
    for candidate in (cwd, *cwd.parents):
        if (candidate / "ref" / "lovart_manifest.json").exists() or (candidate / "pyproject.toml").exists():
            return candidate
    return Path(__file__).resolve().parent.parent


ROOT = _project_root()
REF_DIR = ROOT / "ref"
CAPTURES_DIR = ROOT / "captures"
DOWNLOADS_DIR = ROOT / "downloads"
RUNTIME_DIR = ROOT / ".lovart"
CREDS_FILE = RUNTIME_DIR / "creds.json"
LEGACY_CREDS_FILE = ROOT / "scripts" / "creds.json"
MANIFEST_FILE = REF_DIR / "lovart_manifest.json"
GENERATOR_LIST_FILE = REF_DIR / "lovart_generator_list.json"
GENERATOR_SCHEMA_FILE = REF_DIR / "lovart_generator_schema.json"
PRICING_TABLE_FILE = REF_DIR / "lovart_pricing_table.json"
SIGNATURE_JS = ROOT / "lovart_reverse" / "signing" / "signature.js"
SIGNER_WASM = REF_DIR / "lovart_static_assets" / "26bd3a5bd74c3c92.wasm"
