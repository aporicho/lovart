from __future__ import annotations

import json
import subprocess
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

    def test_gitignore_does_not_hide_package_download_module(self) -> None:
        gitignore_lines = (ROOT / ".gitignore").read_text().splitlines()
        self.assertIn("/downloads/", gitignore_lines)
        self.assertNotIn("downloads/", gitignore_lines)

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
        self.assertIn("install.sh", workflow)
        self.assertIn("install.ps1", workflow)
        self.assertIn("SHA256SUMS", workflow)
        self.assertIn('method":"tools/list', workflow)
        self.assertIn('"${{ matrix.binary_path }}" mcp', workflow)

    def test_install_sh_dry_run_json_maps_current_platform(self) -> None:
        result = subprocess.run(
            ["bash", str(ROOT / "packaging" / "install" / "install.sh"), "--dry-run", "--json"],
            capture_output=True,
            text=True,
            check=False,
        )
        self.assertEqual(result.returncode, 0, result.stderr)
        payload = json.loads(result.stdout)
        self.assertTrue(payload["ok"])
        self.assertIn(payload["data"]["asset"], {"lovart-macos-arm64", "lovart-linux-x64"})
        self.assertTrue(payload["data"]["dry_run"])

    def test_install_scripts_require_gh_for_real_downloads(self) -> None:
        shell_script = (ROOT / "packaging" / "install" / "install.sh").read_text()
        powershell_script = (ROOT / "packaging" / "install" / "install.ps1").read_text()
        self.assertIn("gh auth status", shell_script)
        self.assertIn("gh release download", shell_script)
        self.assertIn("--mcp-clients", shell_script)
        self.assertIn('"mcp" "install"', shell_script)
        self.assertIn("Get-FileHash -Algorithm SHA256", powershell_script)
        self.assertIn("lovart-windows-x64.exe", powershell_script)
        self.assertIn("McpClients", powershell_script)
        self.assertIn('"mcp", "install"', powershell_script)


if __name__ == "__main__":
    unittest.main()
