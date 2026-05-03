from __future__ import annotations

import contextlib
import io
import json
import subprocess
import sys
import unittest
from unittest.mock import patch

from lovart_reverse.cli.main import main
from lovart_reverse.cli.application import build_parser


class CliTest(unittest.TestCase):
    def test_version_stdout_is_json_envelope(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["--version"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["package"], "lovart-reverse")
        self.assertIn("manifest", payload["data"])

    def test_self_test_stdout_is_json_envelope(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["self-test"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertIn("checks", payload["data"])
        self.assertTrue(payload["data"]["checks"]["mcp_command_supported"])
        self.assertIn("binary_mode", payload["data"]["runtime"])
        self.assertIn("reverse_extra_available", payload["data"]["runtime"])

    def test_help_lists_current_agent_commands(self) -> None:
        help_text = build_parser().format_help()
        for command in ("config", "plan", "quote", "jobs", "mcp", "agent"):
            self.assertIn(command, help_text)

    def test_agent_status_stdout_is_json_envelope(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["agent", "status", "--agents", "all"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual({item["agent"] for item in payload["data"]["agents"]}, {"codex", "claude", "opencode", "openclaw"})

    def test_mcp_subcommand_starts_stdio_server(self) -> None:
        result = subprocess.run(
            [sys.executable, "-m", "lovart_reverse.cli.main", "mcp"],
            input='{"jsonrpc":"2.0","id":1,"method":"tools/list"}\n',
            capture_output=True,
            text=True,
            timeout=5,
            check=False,
        )
        self.assertEqual(result.returncode, 0)
        payload = json.loads(result.stdout.splitlines()[0])
        self.assertEqual(payload["id"], 1)
        self.assertIn("tools", payload["result"])

    def test_models_stdout_is_json_envelope(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["models"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["count"], 55)

    def test_setup_stdout_is_json_envelope(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["setup", "--offline"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertIn("ready", payload["data"])

    def test_generate_dry_run_returns_preflight(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(
                [
                    "generate",
                    "openai/gpt-image-2",
                    "--offline",
                    "--dry-run",
                    "--body",
                    '{"prompt":"test","quality":"low","size":"1024*1024"}',
                ]
            )
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertFalse(payload["data"]["submitted"])
        self.assertIn("preflight", payload["data"])

    def test_generate_missing_auth_errors_before_submit(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.generation.preflight.auth_status", return_value={"exists": False, "header_names": []}),
            patch("lovart_reverse.commands.facade.submit_model") as submit,
            contextlib.redirect_stdout(output),
        ):
            code = main(
                [
                    "generate",
                    "openai/gpt-image-2",
                    "--offline",
                    "--body",
                    '{"prompt":"test","quality":"low","size":"1024*1024"}',
                ]
            )
        self.assertEqual(code, 2)
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "auth_missing")
        submit.assert_not_called()

    def test_generate_stale_signer_errors_before_submit(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.generation.preflight.auth_status", return_value={"exists": True, "header_names": ["token"]}),
            patch("lovart_reverse.generation.preflight._update_status", return_value={"status": "stale", "signer_maybe_stale": True, "changes": {"frontend_bundle": True}, "recommended_actions": ["run lovart update sync --metadata-only"]}),
            patch("lovart_reverse.commands.facade.submit_model") as submit,
            contextlib.redirect_stdout(output),
        ):
            code = main(
                [
                    "generate",
                    "openai/gpt-image-2",
                    "--body",
                    '{"prompt":"test","quality":"low","size":"1024*1024"}',
                ]
            )
        self.assertEqual(code, 2)
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "signer_stale")
        submit.assert_not_called()

    def test_reverse_capture_requires_reverse_extra(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.capture.runtime.reverse_extra_status", return_value={"available": False, "mitmproxy_module": False, "mitmdump": None}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["reverse", "capture"])
        self.assertEqual(code, 2)
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "input_error")
        self.assertIn("reverse_extra", payload["error"]["details"])


if __name__ == "__main__":
    unittest.main()
