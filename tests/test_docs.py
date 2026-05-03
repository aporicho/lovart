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
            "lovart plan",
            "lovart quote",
            "lovart jobs quote",
            "lovart jobs dry-run",
            "lovart jobs run",
            "lovart jobs status",
            "lovart jobs resume",
            "lovart generate",
            "lovart update sync --metadata-only",
            "lovart reverse capture",
        ):
            self.assertIn(command, text)

    def test_lovart_cli_expert_doc_exists(self) -> None:
        text = (ROOT / "docs" / "concepts" / "LovartCLI生成专家.md").read_text()
        self.assertIn("lovart jobs quote", text)
        self.assertIn("lovart jobs resume", text)
        self.assertIn("不读取 `.lovart/creds.json`", text)


if __name__ == "__main__":
    unittest.main()
