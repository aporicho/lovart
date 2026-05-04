from __future__ import annotations

import unittest
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

from lovart_reverse.capture.runtime import _mitmdump_path


class CaptureRuntimeTest(unittest.TestCase):
    def test_mitmdump_path_falls_back_to_current_tool_environment(self) -> None:
        with TemporaryDirectory() as tmp:
            bindir = Path(tmp) / "bin"
            bindir.mkdir()
            python = bindir / "python"
            python.write_text("")
            mitmdump = bindir / "mitmdump"
            mitmdump.write_text("")
            with (
                patch("lovart_reverse.capture.runtime.shutil.which", return_value=None),
                patch("lovart_reverse.capture.runtime.sys.executable", str(python)),
            ):
                self.assertEqual(_mitmdump_path(), str(mitmdump))


if __name__ == "__main__":
    unittest.main()
