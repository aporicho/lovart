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
            "lovart generate",
            "lovart update sync --metadata-only",
            "lovart reverse capture",
        ):
            self.assertIn(command, text)


if __name__ == "__main__":
    unittest.main()
