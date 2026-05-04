from __future__ import annotations

import json
import unittest
from unittest.mock import patch

from lovart_reverse.mcp.server import UNSAFE_TOOL_NAMES, call_tool_envelope, handle_message, list_tools


class McpTest(unittest.TestCase):
    def test_lists_only_safe_tools(self) -> None:
        names = {tool["name"] for tool in list_tools()}
        self.assertIn("lovart_setup", names)
        self.assertIn("lovart_generate", names)
        self.assertIn("lovart_jobs_resume", names)
        self.assertIn("lovart_jobs_quote_status", names)
        self.assertTrue(names.isdisjoint(UNSAFE_TOOL_NAMES))
        for name in names:
            self.assertNotIn("capture", name)
            self.assertNotIn("auth_extract", name)
            self.assertNotIn("update_sync", name)

    def test_tool_call_returns_cli_compatible_envelope(self) -> None:
        result = call_tool_envelope("lovart_setup", {"offline": True})
        self.assertTrue(result["ok"])
        self.assertIn("ready", result["data"])

    def test_json_rpc_tools_list(self) -> None:
        response = handle_message({"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
        self.assertIsNotNone(response)
        self.assertEqual(response["id"], 1)
        self.assertIn("tools", response["result"])

    def test_json_rpc_tool_call_wraps_envelope_as_text(self) -> None:
        response = handle_message(
            {
                "jsonrpc": "2.0",
                "id": 2,
                "method": "tools/call",
                "params": {"name": "lovart_setup", "arguments": {"offline": True}},
            }
        )
        self.assertIsNotNone(response)
        content = response["result"]["content"]
        payload = json.loads(content[0]["text"])
        self.assertTrue(payload["ok"])

    def test_generate_uses_paid_gate_and_does_not_submit_when_blocked(self) -> None:
        with (
            patch("lovart_reverse.generation.preflight.auth_status", return_value={"exists": False, "header_names": []}),
            patch("lovart_reverse.commands.facade.submit_model") as submit,
        ):
            result = call_tool_envelope(
                "lovart_generate",
                {"model": "openai/gpt-image-2", "body": {"prompt": "x", "quality": "low", "size": "1024*1024"}, "offline": True},
            )
        self.assertFalse(result["ok"])
        self.assertEqual(result["error"]["code"], "auth_missing")
        submit.assert_not_called()

    def test_jobs_resume_caps_mcp_wait_and_uses_compact_detail(self) -> None:
        with patch("lovart_reverse.mcp.server.jobs_resume_command", return_value={"operation": "resume", "warnings": []}) as resume:
            result = call_tool_envelope(
                "lovart_jobs_resume",
                {"jobs_file": "runs/x/jobs.jsonl", "wait": True, "timeout_seconds": 999, "download_dir": "runs/x/images"},
            )
        self.assertTrue(result["ok"])
        self.assertIn("capped", result["data"]["warnings"][0])
        self.assertEqual(resume.call_args.kwargs["timeout_seconds"], 90.0)
        self.assertEqual(resume.call_args.kwargs["detail"], "summary")
        self.assertEqual(str(resume.call_args.kwargs["download_dir"]), "runs/x/images")

    def test_jobs_status_uses_summary_detail_by_default(self) -> None:
        with patch("lovart_reverse.mcp.server.jobs_status_command", return_value={"operation": "status"}) as status:
            result = call_tool_envelope("lovart_jobs_status", {"run_dir": "runs/x"})
        self.assertTrue(result["ok"])
        self.assertEqual(status.call_args.kwargs["detail"], "summary")


if __name__ == "__main__":
    unittest.main()
