from __future__ import annotations

import unittest
from unittest.mock import patch

from lovart_reverse.generation.submit import dry_run_request, find_task_id, submit_model, task_request_payload
from lovart_reverse.task.client import normalize_task_response, task_info


class _Response:
    def __init__(self, payload: dict[str, object]):
        self._payload = payload

    def json(self) -> dict[str, object]:
        return self._payload


class GenerationSubmitTest(unittest.TestCase):
    def test_task_request_payload_uses_generator_task_endpoint_shape(self) -> None:
        body = {"prompt": "test", "quality": "low", "size": "1024*1024"}
        self.assertEqual(
            task_request_payload("openai/gpt-image-2", body),
            {"generator_name": "openai/gpt-image-2", "input_args": body},
        )

    def test_dry_run_uses_generator_tasks_path(self) -> None:
        request = dry_run_request("openai/gpt-image-2", {"prompt": "test"}, language="en")
        self.assertEqual(request["method"], "POST")
        self.assertEqual(request["path"], "/v1/generator/tasks")
        self.assertEqual(request["language"], "en")
        self.assertEqual(request["body"], {"generator_name": "openai/gpt-image-2", "input_args": {"prompt": "test"}})
        self.assertTrue(request["signature_required"])

    def test_submit_model_posts_lgw_generator_task_payload(self) -> None:
        with patch("lovart_reverse.generation.submit.lgw_request", return_value=_Response({"code": 0})) as request:
            result = submit_model("openai/gpt-image-2", {"prompt": "test"}, language="zh")
        self.assertEqual(result, {"code": 0})
        request.assert_called_once_with(
            "POST",
            "/v1/generator/tasks",
            body={"generator_name": "openai/gpt-image-2", "input_args": {"prompt": "test"}},
            language="zh",
        )

    def test_find_task_id_accepts_generator_task_id(self) -> None:
        self.assertEqual(find_task_id({"data": {"generator_task_id": "task-123"}}), "task-123")

    def test_task_info_uses_lgw_generator_tasks_query(self) -> None:
        payload = {"code": 0, "data": {"task_id": "task-123", "status": "completed", "artifacts": [{"type": "image"}]}}
        with patch("lovart_reverse.task.client.lgw_request", return_value=_Response(payload)) as request:
            result = task_info("task-123", language="zh")
        self.assertEqual(result["task_id"], "task-123")
        self.assertEqual(result["status"], "completed")
        self.assertEqual(result["artifacts"], [{"type": "image"}])
        request.assert_called_once_with("GET", "/v1/generator/tasks", params={"task_id": "task-123"}, language="zh", timeout=30)

    def test_normalize_task_response_accepts_generator_task_id(self) -> None:
        result = normalize_task_response({"data": {"generator_task_id": "task-123", "status": "submitted"}})
        self.assertEqual(result["task_id"], "task-123")
        self.assertEqual(result["status"], "submitted")


if __name__ == "__main__":
    unittest.main()
