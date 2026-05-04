from __future__ import annotations

import unittest
from unittest.mock import patch

from lovart_reverse.errors import RemoteError
from lovart_reverse.generation.submit import (
    apply_generation_mode,
    dry_run_request,
    find_task_id,
    submit_model,
    take_generation_slot,
    task_request_payload,
)
from lovart_reverse.task.client import normalize_task_response, task_info


class _Response:
    def __init__(self, payload: dict[str, object]):
        self._payload = payload

    def json(self) -> dict[str, object]:
        return self._payload

    def raise_for_status(self) -> None:
        return None


class GenerationSubmitTest(unittest.TestCase):
    def test_task_request_payload_uses_generator_task_endpoint_shape(self) -> None:
        body = {"prompt": "test", "quality": "low", "size": "1024*1024"}
        payload = task_request_payload("openai/gpt-image-2", body, context_ids={"cid": "cid-1", "project_id": "project-1"})
        self.assertEqual(payload["cid"], "cid-1")
        self.assertEqual(payload["project_id"], "project-1")
        self.assertEqual(payload["generator_name"], "openai/gpt-image-2")
        self.assertEqual(payload["input_args"]["prompt"], "test")
        self.assertEqual(payload["input_args"]["width"], 1024)
        self.assertEqual(payload["input_args"]["height"], 1024)
        self.assertEqual(payload["input_args"]["original_unit_data"]["prompt"], "test")
        self.assertEqual(payload["input_args"]["original_unit_data"]["ratio"], "1:1(1k)")

    def test_dry_run_uses_generator_tasks_path(self) -> None:
        with patch("lovart_reverse.generation.submit.saved_ids", return_value={}):
            request = dry_run_request("openai/gpt-image-2", {"prompt": "test"}, language="en")
        self.assertEqual(request["method"], "POST")
        self.assertEqual(request["path"], "/v1/generator/tasks")
        self.assertEqual(request["language"], "en")
        self.assertEqual(request["body"]["generator_name"], "openai/gpt-image-2")
        self.assertEqual(request["body"]["input_args"]["prompt"], "test")
        self.assertIn("original_unit_data", request["body"]["input_args"])
        self.assertTrue(request["signature_required"])

    def test_submit_model_posts_lgw_generator_task_payload(self) -> None:
        with (
            patch("lovart_reverse.generation.submit.saved_ids", return_value={"cid": "cid-1", "project_id": "project-1"}),
            patch("lovart_reverse.generation.submit.take_generation_slot", return_value={"code": 0, "data": {"status": "SUCCESS"}}) as slot,
            patch("lovart_reverse.generation.submit.lgw_request", return_value=_Response({"code": 0})) as request,
        ):
            result = submit_model("openai/gpt-image-2", {"prompt": "test"}, language="zh")
        self.assertEqual(result, {"code": 0})
        slot.assert_called_once_with(language="zh")
        call = request.call_args
        self.assertEqual(call.args, ("POST", "/v1/generator/tasks"))
        self.assertEqual(call.kwargs["language"], "zh")
        self.assertEqual(call.kwargs["body"]["cid"], "cid-1")
        self.assertEqual(call.kwargs["body"]["project_id"], "project-1")
        self.assertEqual(call.kwargs["body"]["generator_name"], "openai/gpt-image-2")
        self.assertEqual(call.kwargs["body"]["input_args"]["prompt"], "test")

    def test_apply_generation_mode_switches_relax_to_unlimited(self) -> None:
        session = patch("lovart_reverse.generation.submit.www_session").start().return_value
        self.addCleanup(patch.stopall)
        session.post.return_value = _Response({"code": 0, "data": {"status": "SUCCESS"}})
        result = apply_generation_mode("relax", context_ids={"cid": "cid-1"}, language="zh")
        self.assertEqual(result, {"code": 0, "data": {"status": "SUCCESS"}})
        session.post.assert_called_once()
        self.assertEqual(session.post.call_args.kwargs["json"], {"unlimited": True, "cid": "cid-1"})

    def test_apply_generation_mode_requires_cid(self) -> None:
        with self.assertRaises(RemoteError):
            apply_generation_mode("fast", context_ids={}, language="zh")

    def test_take_generation_slot_uses_project_and_cid(self) -> None:
        session = patch("lovart_reverse.generation.submit.www_session").start().return_value
        self.addCleanup(patch.stopall)
        session.post.return_value = _Response({"code": 0, "data": {"status": "SUCCESS"}})
        result = take_generation_slot(context_ids={"cid": "cid-1", "project_id": "project-1"}, language="zh")
        self.assertEqual(result, {"code": 0, "data": {"status": "SUCCESS"}})
        session.post.assert_called_once()
        self.assertEqual(session.post.call_args.kwargs["json"], {"project_id": "project-1", "cid": "cid-1"})

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
