"""Architecture guardrails for the Lovart reverse package."""

from __future__ import annotations

import ast
import re
from dataclasses import asdict, dataclass
from pathlib import Path

from lovart_reverse.paths import ROOT

FORBIDDEN_FILE_STEMS = {"utils", "helpers", "common", "service", "legacy", "compat", "glue"}
SENSITIVE_PATTERNS = [
    re.compile(r"Bearer\s+[A-Za-z0-9._-]{20,}"),
    re.compile(r"(?i)(cookie|authorization)[\"']?\s*[:=]\s*[\"'][^\"']{20,}"),
    re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}"),
]


@dataclass(frozen=True)
class CheckResult:
    ok: bool
    violations: list[str]

    def to_dict(self) -> dict[str, object]:
        return asdict(self)


def _python_files() -> list[Path]:
    return [path for path in (ROOT / "lovart_reverse").rglob("*.py") if "__pycache__" not in path.parts]


def _check_file_names(violations: list[str]) -> None:
    for path in _python_files():
        if path.stem in FORBIDDEN_FILE_STEMS:
            violations.append(f"forbidden vague module name: {path.relative_to(ROOT)}")


def _check_init_files(violations: list[str]) -> None:
    for path in (ROOT / "lovart_reverse").rglob("__init__.py"):
        tree = ast.parse(path.read_text())
        for node in tree.body:
            if isinstance(node, (ast.Import, ast.ImportFrom, ast.Assign, ast.AnnAssign)):
                continue
            if isinstance(node, ast.Expr) and isinstance(node.value, ast.Constant) and isinstance(node.value.value, str):
                continue
            violations.append(f"__init__.py contains non-export logic: {path.relative_to(ROOT)}")
            break


def _check_cli_dependencies(violations: list[str]) -> None:
    for path in _python_files():
        if "cli" in path.relative_to(ROOT).parts:
            continue
        tree = ast.parse(path.read_text())
        for node in ast.walk(tree):
            if isinstance(node, (ast.Import, ast.ImportFrom)):
                names = [alias.name for alias in node.names] if isinstance(node, ast.Import) else [node.module or ""]
                if any(name.startswith("lovart_reverse.cli") for name in names):
                    violations.append(f"business module imports CLI: {path.relative_to(ROOT)}")


def _check_scripts(violations: list[str]) -> None:
    scripts = ROOT / "scripts"
    if not scripts.exists():
        return
    allowed = {"lovart.py"}
    for path in scripts.glob("*.py"):
        if path.name not in allowed:
            violations.append(f"scripts/ allows only lovart.py: {path.relative_to(ROOT)}")


def _check_sensitive_text(violations: list[str]) -> None:
    scan_roots = [ROOT / "lovart_reverse", ROOT / "docs", ROOT / "tests"]
    for root in scan_roots:
        if not root.exists():
            continue
        for path in root.rglob("*"):
            if not path.is_file() or path.suffix not in {".py", ".md", ".json", ".js"}:
                continue
            text = path.read_text(errors="ignore")
            for pattern in SENSITIVE_PATTERNS:
                if pattern.search(text):
                    violations.append(f"possible sensitive value in {path.relative_to(ROOT)}")
                    break


def run_checks() -> CheckResult:
    violations: list[str] = []
    _check_file_names(violations)
    _check_init_files(violations)
    _check_cli_dependencies(violations)
    _check_scripts(violations)
    _check_sensitive_text(violations)
    return CheckResult(ok=not violations, violations=violations)
