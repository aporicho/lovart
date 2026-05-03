"""Filesystem paths used by Lovart reverse tooling."""

from __future__ import annotations

import os
from pathlib import Path


PACKAGE_DIR = Path(__file__).resolve().parent
PACKAGE_REF_DIR = PACKAGE_DIR / "data" / "ref"
USER_ROOT = Path(os.environ.get("LOVART_REVERSE_HOME", "~/.lovart-reverse")).expanduser().resolve()


def _repo_root() -> Path | None:
    package_root = PACKAGE_DIR.parent
    if (package_root / "ref" / "lovart_manifest.json").exists() and (package_root / "pyproject.toml").exists():
        return package_root
    return None


def _runtime_root() -> Path:
    env_root = os.environ.get("LOVART_REVERSE_ROOT")
    if env_root:
        return Path(env_root).expanduser().resolve()
    repo_root = _repo_root()
    return repo_root if repo_root else USER_ROOT


def _readable_ref_file(name: str) -> Path:
    user_file = REF_DIR / name
    if user_file.exists():
        return user_file
    package_file = PACKAGE_REF_DIR / name
    return package_file if package_file.exists() else user_file


ROOT = _runtime_root()
REF_DIR = ROOT / "ref"
CAPTURES_DIR = ROOT / "captures"
DOWNLOADS_DIR = ROOT / "downloads"
RUNS_DIR = ROOT / "runs"
RUNTIME_DIR = ROOT / ".lovart"
CREDS_FILE = RUNTIME_DIR / "creds.json"
LEGACY_CREDS_FILE = ROOT / "scripts" / "creds.json"
MANIFEST_FILE = _readable_ref_file("lovart_manifest.json")
GENERATOR_LIST_FILE = _readable_ref_file("lovart_generator_list.json")
GENERATOR_SCHEMA_FILE = _readable_ref_file("lovart_generator_schema.json")
PRICING_TABLE_FILE = _readable_ref_file("lovart_pricing_table.json")
SIGNATURE_JS = PACKAGE_DIR / "signing" / "signature.js"
SIGNER_WASM = _readable_ref_file("lovart_static_assets/26bd3a5bd74c3c92.wasm")
WRITABLE_MANIFEST_FILE = REF_DIR / "lovart_manifest.json"
WRITABLE_GENERATOR_LIST_FILE = REF_DIR / "lovart_generator_list.json"
WRITABLE_GENERATOR_SCHEMA_FILE = REF_DIR / "lovart_generator_schema.json"
WRITABLE_PRICING_TABLE_FILE = REF_DIR / "lovart_pricing_table.json"
