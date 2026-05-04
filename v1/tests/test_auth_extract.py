from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from lovart_reverse.auth.extract import extract_from_capture, extract_from_captures


class AuthExtractTest(unittest.TestCase):
    def test_extract_from_capture_saves_cid_and_project_id(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            capture = Path(tmp) / "submit.json"
            capture.write_text(
                json.dumps(
                    {
                        "request": {
                            "headers": {"token": "token-1", "cookie": "cookie-1"},
                            "body": {"cid": "cid-1", "project_id": "project-1"},
                        },
                        "response": {"body": {"code": 0}},
                    }
                )
            )
            with (
                patch("lovart_reverse.auth.extract.load_credentials", return_value={"headers": {}, "ids": {}}),
                patch("lovart_reverse.auth.extract.save_credentials") as save,
            ):
                result = extract_from_capture(capture)
        self.assertTrue(result["saved"])
        self.assertEqual(result["id_keys"], ["cid", "project_id"])
        save.assert_called_once()
        self.assertEqual(save.call_args.args[1], {"cid": "cid-1", "project_id": "project-1"})

    def test_extract_from_captures_reads_nested_capture_shape(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            capture = Path(tmp) / "submit.json"
            capture.write_text(
                json.dumps(
                    {
                        "request": {
                            "url": "https://lgw.lovart.ai/v1/generator/tasks",
                            "headers": {"token": "token-1"},
                            "body": {"cid": "cid-1", "project_id": "project-1"},
                        },
                        "response": {"body": {"data": {"generator_task_id": "task-1"}}},
                    }
                )
            )
            with (
                patch("lovart_reverse.auth.extract.load_credentials", return_value={"headers": {}, "ids": {}}),
                patch("lovart_reverse.auth.extract.save_credentials") as save,
            ):
                result = extract_from_captures(Path(tmp))
        self.assertTrue(result["saved"])
        self.assertEqual(result["id_keys"], ["cid", "project_id"])
        save.assert_called_once()

    def test_extract_merges_existing_credentials(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            capture = Path(tmp) / "request.json"
            capture.write_text(
                json.dumps(
                    {
                        "request": {
                            "headers": {"cookie": "cookie-1"},
                            "body": {"project_id": "project-1"},
                        }
                    }
                )
            )
            with (
                patch("lovart_reverse.auth.extract.load_credentials", return_value={"headers": {"token": "token-1"}, "ids": {"cid": "cid-1"}}),
                patch("lovart_reverse.auth.extract.save_credentials") as save,
            ):
                extract_from_capture(capture)
        self.assertEqual(save.call_args.args[0], {"token": "token-1", "cookie": "cookie-1"})
        self.assertEqual(save.call_args.args[1], {"cid": "cid-1", "project_id": "project-1"})


if __name__ == "__main__":
    unittest.main()
