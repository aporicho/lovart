from __future__ import annotations

import contextlib
import io
import json
import unittest
from unittest.mock import patch

from lovart_reverse.cli.main import main


class CliTest(unittest.TestCase):
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
            patch("lovart_reverse.cli.main.submit_model") as submit,
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
            patch("lovart_reverse.cli.main.submit_model") as submit,
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


if __name__ == "__main__":
    unittest.main()
