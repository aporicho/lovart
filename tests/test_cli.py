from __future__ import annotations

import contextlib
import io
import json
import unittest

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


if __name__ == "__main__":
    unittest.main()
