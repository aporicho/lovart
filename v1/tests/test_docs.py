from __future__ import annotations

import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent


class DocsTest(unittest.TestCase):
    def test_agent_docs_reference_existing_commands(self) -> None:
        text = (ROOT / "AGENTS.md").read_text() + "\n" + (ROOT / "README.md").read_text()
        for command in (
            "lovart setup",
            "lovart config",
            "lovart quote",
            "lovart jobs quote",
            "lovart jobs quote-status",
            "lovart jobs dry-run",
            "lovart jobs run",
            "lovart jobs status",
            "lovart jobs resume",
            "lovart generate",
            "lovart mcp status",
            "lovart mcp install",
            "lovart update sync --metadata-only",
            "lovart reverse capture",
        ):
            self.assertIn(command, text)

    def test_mcp_install_doc_uses_single_binary_mcp(self) -> None:
        text = (ROOT / "docs" / "mcp-install.md").read_text()
        self.assertIn("lovart-macos-arm64", text)
        self.assertIn("install.sh", text)
        self.assertIn("install.ps1", text)
        self.assertIn('args = ["mcp"]', text)
        self.assertIn("lovart jobs resume", text)

    def test_batch_docs_use_outputs_not_body_quantity(self) -> None:
        text = "\n".join(
            [
                (ROOT / "AGENTS.md").read_text(),
                (ROOT / "README.md").read_text(),
                (ROOT / "docs" / "agent-contract.md").read_text(),
                (ROOT / "docs" / "mcp-install.md").read_text(),
            ]
        )
        self.assertIn('"outputs":10', text)
        self.assertIn("CLI", text)
        self.assertIn("total_payable_credits", text)
        self.assertIn("listed_credits", text)
        self.assertNotIn('"n":1', text)


if __name__ == "__main__":
    unittest.main()
