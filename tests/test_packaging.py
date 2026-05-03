from __future__ import annotations

import tomllib
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class PackagingTest(unittest.TestCase):
    def test_default_dependencies_exclude_reverse_capture_stack(self) -> None:
        pyproject = tomllib.loads((ROOT / "pyproject.toml").read_text())
        dependencies = pyproject["project"]["dependencies"]
        self.assertIn("requests>=2.33.1", dependencies)
        self.assertTrue(all(not dependency.startswith("mitmproxy") for dependency in dependencies))

    def test_reverse_extra_declares_mitmproxy(self) -> None:
        pyproject = tomllib.loads((ROOT / "pyproject.toml").read_text())
        reverse = pyproject["project"]["optional-dependencies"]["reverse"]
        self.assertIn("mitmproxy>=11.0.2", reverse)

    def test_pyinstaller_spec_exists_for_single_lovart_binary(self) -> None:
        spec = (ROOT / "packaging" / "pyinstaller" / "lovart.spec").read_text()
        self.assertIn('name="lovart"', spec)
        self.assertIn("lovart_generator_schema.json", spec)
        self.assertIn("lovart_manifest.json", spec)
        self.assertIn("26bd3a5bd74c3c92.wasm", spec)
        self.assertIn("build_info.json", spec)
        self.assertIn('excludes=["mitmproxy"]', spec)

    def test_release_workflow_targets_three_binary_assets(self) -> None:
        workflow = (ROOT / ".github" / "workflows" / "release-binaries.yml").read_text()
        self.assertIn("lovart-macos-arm64", workflow)
        self.assertIn("lovart-linux-x64", workflow)
        self.assertIn("lovart-windows-x64.exe", workflow)
        self.assertIn("lovart.spec", workflow)
        self.assertIn('method":"tools/list', workflow)
        self.assertIn('"${{ matrix.binary_path }}" mcp', workflow)


if __name__ == "__main__":
    unittest.main()
