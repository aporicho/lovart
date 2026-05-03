from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from lovart_reverse.agent import agent_install, agent_status
from lovart_reverse.errors import LovartError


class AgentConfigTest(unittest.TestCase):
    def test_agent_status_lists_supported_agents(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            result = agent_status(agents="all", lovart_path=Path("/tmp/lovart"), home=Path(temp))
        names = {item["agent"] for item in result["agents"]}
        self.assertEqual(names, {"codex", "claude", "opencode", "openclaw"})

    def test_codex_dry_run_outputs_toml_without_writing(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            result = agent_install(agents="codex", lovart_path=Path("/tmp/lovart"), home=home, dry_run=True, yes=True)
            config = home / ".codex" / "config.toml"
        self.assertFalse(config.exists())
        self.assertEqual(result["results"][0]["status"], "dry_run")
        self.assertIn("[mcp_servers.lovart]", result["results"][0]["preview"]["toml"])

    def test_opencode_dry_run_outputs_json_without_writing(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            result = agent_install(agents="opencode", lovart_path=Path("/tmp/lovart"), home=home, dry_run=True, yes=True)
            config = home / ".config" / "opencode" / "opencode.json"
        self.assertFalse(config.exists())
        preview = result["results"][0]["preview"]["json"]
        self.assertEqual(preview["command"], [str(Path("/tmp/lovart").resolve()), "mcp"])
        self.assertTrue(preview["enabled"])

    def test_codex_unmanaged_config_conflict_requires_force(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            path = home / ".codex" / "config.toml"
            path.parent.mkdir(parents=True)
            path.write_text('[mcp_servers.lovart]\ncommand = "/other/lovart"\nargs = ["mcp"]\n')
            with self.assertRaises(LovartError) as raised:
                agent_install(agents="codex", lovart_path=Path("/tmp/lovart"), home=home, yes=True)
        self.assertEqual(raised.exception.code, "config_conflict")

    def test_codex_write_creates_backup(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            path = home / ".codex" / "config.toml"
            path.parent.mkdir(parents=True)
            path.write_text("# existing\n")
            result = agent_install(agents="codex", lovart_path=Path("/tmp/lovart"), home=home, yes=True)
            text = path.read_text()
            backup = Path(result["results"][0]["backup"])
            self.assertIn(str(Path("/tmp/lovart").resolve()), text)
            self.assertTrue(backup.exists())

    def test_agents_none_makes_no_changes(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            result = agent_install(agents="none", lovart_path=Path("/tmp/lovart"), home=home, yes=True)
        self.assertEqual(result["agents_selected"], [])
        self.assertEqual(result["results"], [])

    def test_opencode_invalid_json_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as temp:
            home = Path(temp)
            path = home / ".config" / "opencode" / "opencode.json"
            path.parent.mkdir(parents=True)
            path.write_text("{")
            with self.assertRaises(LovartError) as raised:
                agent_install(agents="opencode", lovart_path=Path("/tmp/lovart"), home=home, yes=True)
        self.assertEqual(raised.exception.code, "config_invalid")


if __name__ == "__main__":
    unittest.main()
