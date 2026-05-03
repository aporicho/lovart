"""Configure local agents to use the Lovart MCP server."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from lovart_reverse.errors import LovartError


SUPPORTED_AGENTS = ("codex", "claude", "opencode", "openclaw")
MANAGED_MARKER = "# Managed by lovart-reverse"


class ConfigConflictError(LovartError):
    def __init__(self, message: str, details: dict[str, Any] | None = None):
        super().__init__("config_conflict", message, details or {}, 2)


@dataclass(frozen=True)
class AgentContext:
    lovart_path: Path
    home: Path
    dry_run: bool = False
    yes: bool = False
    force: bool = False


def agent_status(*, agents: str = "auto", lovart_path: Path | None = None, home: Path | None = None) -> dict[str, Any]:
    ctx = AgentContext(_lovart_path(lovart_path), _home(home), dry_run=True)
    selected = _select_agents(agents, ctx, include_missing=True)
    statuses = [_status_for(agent, ctx) for agent in selected]
    return {"lovart_path": str(ctx.lovart_path), "agents": statuses}


def agent_install(
    *,
    agents: str = "auto",
    lovart_path: Path | None = None,
    home: Path | None = None,
    dry_run: bool = False,
    yes: bool = False,
    force: bool = False,
) -> dict[str, Any]:
    ctx = AgentContext(_lovart_path(lovart_path), _home(home), dry_run=dry_run, yes=yes, force=force)
    selected = _select_agents(agents, ctx, include_missing=False)
    if agents == "none":
        selected = []
    results = [_install_agent(agent, ctx) for agent in selected]
    return {
        "lovart_path": str(ctx.lovart_path),
        "dry_run": dry_run,
        "force": force,
        "agents_requested": agents,
        "agents_selected": selected,
        "results": results,
    }


def _lovart_path(path: Path | None) -> Path:
    if path:
        return path.expanduser().resolve()
    if getattr(sys, "frozen", False):
        return Path(sys.executable).resolve()
    argv0 = Path(sys.argv[0])
    if argv0.exists():
        return argv0.expanduser().resolve()
    found = shutil.which("lovart")
    return Path(found).resolve() if found else argv0


def _home(path: Path | None) -> Path:
    return (path or Path.home()).expanduser().resolve()


def _select_agents(agents: str, ctx: AgentContext, *, include_missing: bool) -> list[str]:
    if agents == "none":
        return []
    if agents == "all":
        return list(SUPPORTED_AGENTS)
    if agents == "auto":
        selected = [agent for agent in SUPPORTED_AGENTS if _agent_detected(agent, ctx)]
        if selected:
            return selected
        return list(SUPPORTED_AGENTS) if include_missing else ["codex"]
    selected = [item.strip().lower() for item in agents.split(",") if item.strip()]
    unknown = sorted(set(selected) - set(SUPPORTED_AGENTS))
    if unknown:
        raise LovartError("input_error", "unknown agent", {"agents": unknown, "supported_agents": list(SUPPORTED_AGENTS)}, 2)
    return selected


def _agent_detected(agent: str, ctx: AgentContext) -> bool:
    if agent == "codex":
        return (ctx.home / ".codex").exists()
    if agent == "claude":
        return shutil.which("claude") is not None
    if agent == "opencode":
        return shutil.which("opencode") is not None or (ctx.home / ".config" / "opencode").exists()
    if agent == "openclaw":
        return shutil.which("openclaw") is not None
    return False


def _status_for(agent: str, ctx: AgentContext) -> dict[str, Any]:
    if agent == "codex":
        return _codex_status(ctx)
    if agent == "claude":
        return _command_status("claude", ["claude", "mcp", "add", "--transport", "stdio", "lovart", "--scope", "user", "--", str(ctx.lovart_path), "mcp"])
    if agent == "opencode":
        return _opencode_status(ctx)
    if agent == "openclaw":
        return _command_status("openclaw", ["openclaw", "mcp", "set", "lovart", _openclaw_payload(ctx)])
    raise LovartError("input_error", "unknown agent", {"agent": agent}, 2)


def _install_agent(agent: str, ctx: AgentContext) -> dict[str, Any]:
    if agent == "codex":
        return _install_codex(ctx)
    if agent == "claude":
        return _install_cli_agent("claude", ["claude", "mcp", "add", "--transport", "stdio", "lovart", "--scope", "user", "--", str(ctx.lovart_path), "mcp"], ctx)
    if agent == "opencode":
        return _install_opencode(ctx)
    if agent == "openclaw":
        return _install_cli_agent("openclaw", ["openclaw", "mcp", "set", "lovart", _openclaw_payload(ctx)], ctx)
    raise LovartError("input_error", "unknown agent", {"agent": agent}, 2)


def _codex_config_path(ctx: AgentContext) -> Path:
    return ctx.home / ".codex" / "config.toml"


def _codex_block(ctx: AgentContext) -> str:
    return "\n".join(
        [
            MANAGED_MARKER,
            "[mcp_servers.lovart]",
            f'command = "{_toml_string(str(ctx.lovart_path))}"',
            'args = ["mcp"]',
            "",
        ]
    )


def _codex_status(ctx: AgentContext) -> dict[str, Any]:
    path = _codex_config_path(ctx)
    text = _read_text(path)
    configured = "[mcp_servers.lovart]" in text and str(ctx.lovart_path) in text and 'args = ["mcp"]' in text
    managed = MANAGED_MARKER in text and "[mcp_servers.lovart]" in text
    return {"agent": "codex", "type": "file", "path": str(path), "exists": path.exists(), "configured": configured, "managed": managed}


def _install_codex(ctx: AgentContext) -> dict[str, Any]:
    path = _codex_config_path(ctx)
    text = _read_text(path)
    has_block = "[mcp_servers.lovart]" in text
    managed = MANAGED_MARKER in text and has_block
    if has_block and not managed and not ctx.force:
        raise ConfigConflictError("existing unmanaged Codex Lovart MCP config", {"agent": "codex", "path": str(path), "recommended_actions": ["rerun with --force", "edit the config manually"]})
    next_text = _replace_toml_lovart_block(text, _codex_block(ctx))
    return _write_config_result("codex", path, next_text, ctx, preview={"toml": _codex_block(ctx)})


def _replace_toml_lovart_block(text: str, block: str) -> str:
    lines = text.splitlines()
    start = next((index for index, line in enumerate(lines) if line.strip() == "[mcp_servers.lovart]"), None)
    if start is None:
        prefix = text.rstrip()
        return (prefix + "\n\n" if prefix else "") + block
    if start > 0 and lines[start - 1].strip() == MANAGED_MARKER:
        start -= 1
    end = start + 1
    while end < len(lines):
        stripped = lines[end].strip()
        if stripped.startswith("[") and stripped.endswith("]") and stripped != "[mcp_servers.lovart]":
            break
        end += 1
    new_lines = lines[:start] + block.rstrip().splitlines() + lines[end:]
    return "\n".join(new_lines).rstrip() + "\n"


def _opencode_config_path(ctx: AgentContext) -> Path:
    return ctx.home / ".config" / "opencode" / "opencode.json"


def _opencode_status(ctx: AgentContext) -> dict[str, Any]:
    path = _opencode_config_path(ctx)
    data = _read_json(path)
    config = (data.get("mcp") or {}).get("lovart") if isinstance(data, dict) else None
    configured = isinstance(config, dict) and config.get("command") == [str(ctx.lovart_path), "mcp"] and config.get("enabled") is True
    managed = isinstance(config, dict) and config.get("managed_by") == "lovart-reverse"
    return {"agent": "opencode", "type": "file", "path": str(path), "exists": path.exists(), "configured": configured, "managed": managed}


def _install_opencode(ctx: AgentContext) -> dict[str, Any]:
    path = _opencode_config_path(ctx)
    data = _read_json(path)
    mcp = data.setdefault("mcp", {})
    existing = mcp.get("lovart")
    if existing and not (isinstance(existing, dict) and existing.get("managed_by") == "lovart-reverse") and not ctx.force:
        raise ConfigConflictError("existing unmanaged OpenCode Lovart MCP config", {"agent": "opencode", "path": str(path), "recommended_actions": ["rerun with --force", "edit the config manually"]})
    mcp["lovart"] = {"type": "local", "command": [str(ctx.lovart_path), "mcp"], "enabled": True, "managed_by": "lovart-reverse"}
    text = json.dumps(data, ensure_ascii=False, indent=2) + "\n"
    return _write_config_result("opencode", path, text, ctx, preview={"json": mcp["lovart"]})


def _install_cli_agent(agent: str, command: list[str], ctx: AgentContext) -> dict[str, Any]:
    executable = command[0]
    available = shutil.which(executable) is not None
    result = {"agent": agent, "type": "command", "available": available, "command": command, "manual_command": _shell_join(command), "changed": False}
    if ctx.dry_run or not available:
        result["status"] = "dry_run" if ctx.dry_run else "manual_required"
        return result
    completed = subprocess.run(command, capture_output=True, text=True, check=False)
    result.update({"status": "configured" if completed.returncode == 0 else "failed", "changed": completed.returncode == 0, "returncode": completed.returncode, "stderr": completed.stderr[-2000:], "stdout": completed.stdout[-2000:]})
    if completed.returncode != 0:
        raise LovartError("agent_config_failed", f"{agent} MCP configuration failed", result, 2)
    return result


def _command_status(agent: str, command: list[str]) -> dict[str, Any]:
    available = shutil.which(command[0]) is not None
    return {"agent": agent, "type": "command", "available": available, "configured": None, "manual_command": _shell_join(command)}


def _openclaw_payload(ctx: AgentContext) -> str:
    return json.dumps({"command": str(ctx.lovart_path), "args": ["mcp"]}, separators=(",", ":"))


def _write_config_result(agent: str, path: Path, text: str, ctx: AgentContext, *, preview: dict[str, Any]) -> dict[str, Any]:
    backup = _backup_path(path)
    result = {"agent": agent, "type": "file", "path": str(path), "backup": str(backup), "changed": False, "dry_run": ctx.dry_run, "preview": preview}
    if ctx.dry_run:
        result["status"] = "dry_run"
        return result
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        backup.write_text(path.read_text())
    path.write_text(text)
    result.update({"changed": True, "status": "configured", "backup_created": path.exists() and backup.exists()})
    return result


def _backup_path(path: Path) -> Path:
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    return path.with_name(f"{path.name}.{stamp}.bak")


def _read_text(path: Path) -> str:
    try:
        return path.read_text()
    except FileNotFoundError:
        return ""


def _read_json(path: Path) -> dict[str, Any]:
    if not path.exists():
        return {}
    try:
        data = json.loads(path.read_text())
    except json.JSONDecodeError as exc:
        raise LovartError("config_invalid", "config file is not valid JSON", {"path": str(path), "error": str(exc)}, 2)
    if not isinstance(data, dict):
        raise LovartError("config_invalid", "config file root must be an object", {"path": str(path)}, 2)
    return data


def _toml_string(value: str) -> str:
    return value.replace("\\", "\\\\").replace('"', '\\"')


def _shell_join(command: list[str]) -> str:
    return " ".join(_shell_quote(part) for part in command)


def _shell_quote(value: str) -> str:
    if not value or any(ch.isspace() or ch in "'\"\\$`{}[]" for ch in value):
        return "'" + value.replace("'", "'\"'\"'") + "'"
    return value
