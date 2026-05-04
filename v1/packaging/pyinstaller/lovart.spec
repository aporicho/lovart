# -*- mode: python ; coding: utf-8 -*-

import json
import os
import subprocess
from pathlib import Path


ROOT = Path(SPECPATH).parents[1]
PACKAGE = ROOT / "lovart_reverse"
BUILD_INFO = ROOT / "build" / "build_info.json"


def git_commit():
    value = os.environ.get("LOVART_BUILD_COMMIT")
    if value:
        return value
    try:
        result = subprocess.run(
            ["git", "-C", str(ROOT), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
            check=True,
        )
    except Exception:
        return None
    return result.stdout.strip() or None


BUILD_INFO.parent.mkdir(parents=True, exist_ok=True)
BUILD_INFO.write_text(json.dumps({"git_commit": git_commit()}, separators=(",", ":")))

datas = [
    (str(PACKAGE / "signing" / "signature.js"), "lovart_reverse/signing"),
    (str(PACKAGE / "data" / "ref" / "lovart_generator_list.json"), "lovart_reverse/data/ref"),
    (str(PACKAGE / "data" / "ref" / "lovart_generator_schema.json"), "lovart_reverse/data/ref"),
    (str(PACKAGE / "data" / "ref" / "lovart_manifest.json"), "lovart_reverse/data/ref"),
    (str(PACKAGE / "data" / "ref" / "lovart_pricing_table.json"), "lovart_reverse/data/ref"),
    (
        str(PACKAGE / "data" / "ref" / "lovart_static_assets" / "26bd3a5bd74c3c92.wasm"),
        "lovart_reverse/data/ref/lovart_static_assets",
    ),
    (str(BUILD_INFO), "lovart_reverse/data"),
]

hiddenimports = []
for path in PACKAGE.rglob("*.py"):
    relative = path.relative_to(ROOT).with_suffix("")
    parts = list(relative.parts)
    if parts[-1] == "__init__":
        parts = parts[:-1]
    hiddenimports.append(".".join(parts))

a = Analysis(
    [str(PACKAGE / "cli" / "main.py")],
    pathex=[str(ROOT)],
    binaries=[],
    datas=datas,
    hiddenimports=hiddenimports,
    hookspath=[],
    hooksconfig={},
    runtime_hooks=[],
    excludes=["mitmproxy"],
    noarchive=False,
    optimize=1,
)
pyz = PYZ(a.pure)

exe = EXE(
    pyz,
    a.scripts,
    a.binaries,
    a.datas,
    [],
    name="lovart",
    debug=False,
    bootloader_ignore_signals=False,
    strip=False,
    upx=True,
    upx_exclude=[],
    runtime_tmpdir=None,
    console=True,
    disable_windowed_traceback=False,
    argv_emulation=False,
    target_arch=None,
    codesign_identity=None,
    entitlements_file=None,
)
