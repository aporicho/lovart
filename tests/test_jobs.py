from __future__ import annotations

import contextlib
import io
import json
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from lovart_reverse.cli.main import main
from lovart_reverse.errors import CreditRiskError, InputError
from lovart_reverse.jobs.expansion import expand_jobs
from lovart_reverse.jobs.records import load_job_records
from lovart_reverse.jobs.orchestrator import quote_jobs, quote_status, resume_jobs, run_jobs, status_jobs
from lovart_reverse.jobs.quote_signature import cost_signature_for_request


def write_jobs(path: Path, jobs: list[dict[str, object]]) -> None:
    path.write_text("\n".join(json.dumps(job, ensure_ascii=False) for job in jobs) + "\n")


def gpt_job(job_id: str, prompt: str = "x", *, quality: str = "low", size: str = "1024*1024") -> dict[str, object]:
    return {
        "job_id": job_id,
        "title": f"job {job_id}",
        "model": "openai/gpt-image-2",
        "mode": "auto",
        "body": {"prompt": prompt, "quality": quality, "size": size},
    }


def user_job(job_id: str, model: str, outputs: int = 10) -> dict[str, object]:
    return {
        "job_id": job_id,
        "title": f"job {job_id}",
        "model": model,
        "mode": "relax",
        "outputs": outputs,
        "body": {"prompt": "x"},
    }


def fake_preflight(model: str, body: dict[str, object], mode: str, allow_paid: bool, max_credits: float | None, live: bool = True):
    credits = 12.0 if body.get("prompt") == "paid" else 0.0
    pricing = {"model": model, "quoted": True, "credits": credits, "warnings": []}
    preflight = {
        "auth": {"exists": True, "header_names": ["token"]},
        "update": {"status": "fresh", "signer_maybe_stale": False, "changes": {}},
        "schema_errors": [],
        "gate": {"allowed": credits == 0 or allow_paid, "pricing": pricing, "entitlement": {"zero_credit": credits == 0}},
        "can_submit": credits == 0 or allow_paid,
        "blocking_error": None,
        "recommended_actions": [],
    }
    if credits > 0 and not allow_paid:
        error = CreditRiskError("generation may spend credits", {"quoted_credits": credits})
        preflight["blocking_error"] = {"code": error.code, "message": error.message, "details": error.details}
        preflight["can_submit"] = False
        return preflight, error
    return preflight, None


def patch_quote_client(side_effect=None, return_value=None):
    class FakeQuoteClient:
        init_count = 0

        def __init__(self, language: str = "en", persistent_signer: bool = False):
            type(self).init_count += 1
            self.language = language
            self.persistent_signer = persistent_signer
            self.warnings: list[str] = []

        def __enter__(self):
            return self

        def __exit__(self, exc_type, exc, tb):
            return None

        def quote(self, model: str, body: dict[str, object]) -> dict[str, object]:
            if side_effect is not None:
                return side_effect(model, body, language=self.language)
            return dict(return_value or {"model": model, "quoted": True, "credits": 0.0, "warnings": []})

    return patch("lovart_reverse.jobs.orchestrator.QuoteClient", FakeQuoteClient), FakeQuoteClient


class JobsTest(unittest.TestCase):
    def test_gpt_image_outputs_maps_to_n(self) -> None:
        jobs = expand_jobs([user_job("001", "openai/gpt-image-2", outputs=10)])
        requests = jobs[0]["remote_requests"]
        self.assertEqual(len(requests), 1)
        self.assertEqual(requests[0]["output_count"], 10)
        self.assertEqual(requests[0]["body"]["n"], 10)

    def test_seedream_outputs_maps_to_max_images(self) -> None:
        jobs = expand_jobs([user_job("001", "seedream/seedream-5-0", outputs=10)])
        requests = jobs[0]["remote_requests"]
        self.assertEqual(len(requests), 1)
        self.assertEqual(requests[0]["output_count"], 10)
        self.assertEqual(requests[0]["body"]["max_images"], 10)

    def test_outputs_split_by_quantity_maximum(self) -> None:
        fake_config = {
            "fields": [
                {"key": "count", "type": "integer", "route_role": "quantity", "maximum": 4},
            ]
        }
        with patch("lovart_reverse.jobs.expansion.config_for_model", return_value=fake_config):
            jobs = expand_jobs([user_job("001", "fake/model", outputs=10)])
        requests = jobs[0]["remote_requests"]
        self.assertEqual([request["output_count"] for request in requests], [4, 4, 2])
        self.assertEqual([request["body"]["count"] for request in requests], [4, 4, 2])

    def test_outputs_split_to_single_requests_without_quantity_field(self) -> None:
        with patch("lovart_reverse.jobs.expansion.config_for_model", return_value={"fields": []}):
            jobs = expand_jobs([user_job("001", "fake/model", outputs=3)])
        requests = jobs[0]["remote_requests"]
        self.assertEqual(len(requests), 3)
        self.assertEqual([request["output_count"] for request in requests], [1, 1, 1])
        self.assertNotIn("n", requests[0]["body"])

    def test_outputs_conflicts_with_body_quantity_field(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            job = user_job("001", "openai/gpt-image-2", outputs=10)
            body = job["body"]
            assert isinstance(body, dict)
            body["n"] = 10
            write_jobs(jobs_file, [job])
            with self.assertRaises(InputError):
                load_job_records(jobs_file)

    def test_duplicate_job_id_is_rejected(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001"), gpt_job("001")])
            with self.assertRaisesRegex(Exception, "duplicate job_id"):
                load_job_records(jobs_file)

    def test_quote_jobs_aggregates_total_credits(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="x"), gpt_job("002", prompt="y", quality="high")])

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                credits = 0.0 if body["quality"] == "low" else 12.0
                return {"model": model, "quoted": True, "credits": credits, "warnings": []}

            quote_client, _ = patch_quote_client(side_effect=quoted)
            with quote_client:
                result = quote_jobs(jobs_file, progress=False)
            self.assertEqual(result["summary"]["total_jobs"], 2)
            self.assertEqual(result["summary"]["total_credits"], 12.0)
            self.assertEqual(result["summary"]["total_payable_credits"], 12.0)
            self.assertTrue(Path(result["quote_file"]).exists())
            self.assertTrue(Path(result["state_file"]).exists())
            self.assertTrue(Path(result["full_quote_file"]).exists())
            self.assertNotIn("remote_requests", result)
            self.assertNotIn("jobs", result)

    def test_quote_jobs_uses_one_quote_client_for_selected_batch(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job(str(index), prompt=str(index)) for index in range(100)])
            calls: list[str] = []

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                calls.append(str(body["prompt"]))
                return {"model": model, "quoted": True, "credits": 0.0, "payable_credits": 0.0, "warnings": []}

            quote_client, fake_client = patch_quote_client(side_effect=quoted)
            with quote_client:
                result = quote_jobs(jobs_file, progress=False)
            self.assertEqual(fake_client.init_count, 1)
            self.assertEqual(calls, ["0"])
            self.assertEqual(result["summary"]["quoted_remote_requests"], 100)
            self.assertEqual(result["summary"]["cache_hits"], 99)
            self.assertEqual(result["summary"]["quoted_representative_requests"], 1)

    def test_cost_signature_excludes_prompt_but_keeps_price_fields(self) -> None:
        base = {"prompt": "a", "quality": "low", "size": "1024*1024", "n": 1}
        same_price = {"prompt": "b", "quality": "low", "size": "1024*1024", "n": 1}
        different_quality = {"prompt": "a", "quality": "high", "size": "1024*1024", "n": 1}
        unknown_field = {"prompt": "a", "quality": "low", "size": "1024*1024", "n": 1, "unknown_price_field": "x"}
        self.assertEqual(
            cost_signature_for_request("openai/gpt-image-2", "auto", base)["signature"],
            cost_signature_for_request("openai/gpt-image-2", "auto", same_price)["signature"],
        )
        self.assertNotEqual(
            cost_signature_for_request("openai/gpt-image-2", "auto", base)["signature"],
            cost_signature_for_request("openai/gpt-image-2", "auto", different_quality)["signature"],
        )
        self.assertNotEqual(
            cost_signature_for_request("openai/gpt-image-2", "auto", base)["signature"],
            cost_signature_for_request("openai/gpt-image-2", "auto", unknown_field)["signature"],
        )

    def test_quote_jobs_auto_limit_processes_100_remote_requests(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job(str(index), prompt=str(index)) for index in range(1000)])
            calls: list[str] = []

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                calls.append(str(body["prompt"]))
                return {"model": model, "quoted": True, "credits": 0.0, "payable_credits": 0.0, "warnings": []}

            quote_client, _ = patch_quote_client(side_effect=quoted)
            with quote_client:
                result = quote_jobs(jobs_file, progress=False)
            self.assertEqual(calls, ["0"])
            self.assertEqual(result["summary"]["effective_limit"], 100)
            self.assertEqual(result["summary"]["quoted_remote_requests"], 100)
            self.assertEqual(result["summary"]["pending_quote_remote_requests"], 900)

    def test_quote_jobs_all_processes_all_pending_requests(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job(str(index), prompt=str(index)) for index in range(101)])
            quote_client, _ = patch_quote_client(
                return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "payable_credits": 0.0, "warnings": []}
            )
            with quote_client:
                result = quote_jobs(jobs_file, all_requests=True, progress=False)
            self.assertEqual(result["summary"]["effective_limit"], 101)
            self.assertEqual(result["summary"]["quoted_remote_requests"], 101)
            self.assertEqual(result["summary"]["pending_quote_remote_requests"], 0)

    def test_quote_jobs_summarizes_user_level_outputs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            jobs = [
                {
                    "job_id": "001",
                    "model": "openai/gpt-image-2",
                    "mode": "relax",
                    "outputs": 10,
                    "body": {"prompt": "a", "quality": "low", "size": "1024*1024"},
                },
                {
                    "job_id": "002",
                    "model": "openai/gpt-image-2",
                    "mode": "relax",
                    "outputs": 10,
                    "body": {"prompt": "b", "quality": "low", "size": "1024*1024"},
                },
            ]
            write_jobs(jobs_file, jobs)
            quote_client, _ = patch_quote_client(return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "warnings": []})
            with quote_client:
                result = quote_jobs(jobs_file, detail="full", progress=False)
            self.assertEqual(result["summary"]["logical_jobs"], 2)
            self.assertEqual(result["summary"]["requested_outputs"], 20)
            self.assertEqual(result["summary"]["remote_requests"], 2)
            self.assertEqual(result["remote_requests"][0]["body"]["n"], 10)

    def test_quote_jobs_distinguishes_payable_and_listed_credits(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="x")])
            quoted = {
                "model": "openai/gpt-image-2",
                "quoted": True,
                "credits": 0.0,
                "payable_credits": 0.0,
                "listed_credits": 120.0,
                "warnings": [],
            }
            quote_client, _ = patch_quote_client(return_value=quoted)
            with quote_client:
                result = quote_jobs(jobs_file, progress=False)
            self.assertEqual(result["summary"]["total_credits"], 0.0)
            self.assertEqual(result["summary"]["total_payable_credits"], 0.0)
            self.assertEqual(result["summary"]["total_listed_credits"], 120.0)
            self.assertEqual(result["summary"]["listed_but_zero_payable_remote_requests"], 1)
            self.assertIn("payable_credits=0", result["warnings"][0])

    def test_quote_jobs_requests_detail_excludes_prompt_body_and_raw(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="secret prompt")])
            quote_result = {
                "model": "openai/gpt-image-2",
                "quoted": True,
                "credits": 0.0,
                "payable_credits": 0.0,
                "listed_credits": 120.0,
                "raw": {"data": "large"},
                "warnings": [],
            }
            quote_client, _ = patch_quote_client(return_value=quote_result)
            with quote_client:
                result = quote_jobs(jobs_file, detail="requests", progress=False)
            request = result["remote_requests"][0]
            self.assertNotIn("body", request)
            self.assertNotIn("raw", json.dumps(request))
            self.assertEqual(request["quote_summary"]["payable_credits"], 0.0)
            self.assertEqual(request["quote_summary"]["listed_credits"], 120.0)

    def test_quote_jobs_limit_and_resume_skips_already_quoted(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a"), gpt_job("002", prompt="b")])
            calls: list[str] = []

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                calls.append(str(body["prompt"]))
                return {"model": model, "quoted": True, "credits": 0.0, "payable_credits": 0.0, "warnings": []}

            quote_client, fake_client = patch_quote_client(side_effect=quoted)
            with quote_client:
                first = quote_jobs(jobs_file, limit=1, progress=False)
                second = quote_jobs(jobs_file, progress=False)
            self.assertEqual(calls, ["a"])
            self.assertEqual(fake_client.init_count, 1)
            self.assertEqual(first["summary"]["quoted_remote_requests"], 1)
            self.assertEqual(first["summary"]["pending_quote_remote_requests"], 1)
            self.assertEqual(second["summary"]["quoted_remote_requests"], 2)
            self.assertEqual(second["summary"]["pending_quote_remote_requests"], 0)

    def test_quote_jobs_isolates_changed_file_without_refresh(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a")])
            quote_client, _ = patch_quote_client(return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "warnings": []})
            with quote_client:
                first = quote_jobs(jobs_file, progress=False)
            write_jobs(jobs_file, [gpt_job("001", prompt="changed")])
            quote_client, _ = patch_quote_client(return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "warnings": []})
            with quote_client:
                second = quote_jobs(jobs_file, progress=False)
            self.assertEqual(second["summary"]["quoted_remote_requests"], 1)
            self.assertNotEqual(first["state_file"], second["state_file"])

    def test_quote_status_lists_multiple_per_file_states(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            first_file = Path(tmp) / "first.jsonl"
            second_file = Path(tmp) / "second.jsonl"
            write_jobs(first_file, [gpt_job("001", prompt="a")])
            write_jobs(second_file, [gpt_job("001", prompt="b")])
            quote_client, _ = patch_quote_client(return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "warnings": []})
            with quote_client:
                quote_jobs(first_file, progress=False)
                quote_jobs(second_file, progress=False)
            result = quote_status(Path(tmp))
            self.assertEqual(result["operation"], "quote_status")
            self.assertEqual(result["state_count"], 2)
            self.assertEqual(result["summary"]["quoted_remote_requests"], 2)

    def test_quote_jobs_failure_is_recorded_without_blocking_summary(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a"), gpt_job("002", prompt="b", quality="high")])

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                if body["prompt"] == "b":
                    raise RuntimeError("pricing failed")
                return {"model": model, "quoted": True, "credits": 0.0, "payable_credits": 0.0, "warnings": []}

            quote_client, _ = patch_quote_client(side_effect=quoted)
            with quote_client:
                result = quote_jobs(jobs_file, progress=False)
            self.assertEqual(result["summary"]["quoted_remote_requests"], 1)
            self.assertEqual(result["summary"]["failed_quote_remote_requests"], 1)

    def test_quote_jobs_network_failure_stops_early_and_leaves_pending(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job(str(index), prompt=str(index)) for index in range(5)])
            calls: list[str] = []

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                calls.append(str(body["prompt"]))
                raise RuntimeError("NameResolutionError: Failed to resolve 'www.lovart.ai'")

            quote_client, fake_client = patch_quote_client(side_effect=quoted)
            with quote_client:
                result = quote_jobs(jobs_file, concurrency=2, progress=False)
            self.assertEqual(len(calls), 3)
            self.assertEqual(fake_client.init_count, 1)
            self.assertEqual(result["summary"]["failed_quote_remote_requests"], 1)
            self.assertEqual(result["summary"]["pending_quote_remote_requests"], 4)
            self.assertEqual(result["summary"]["network_unavailable_remote_requests"], 1)
            self.assertEqual(result["summary"]["error_counts"]["network_unavailable"], 1)
            self.assertIn("network/DNS", result["warnings"][0])
            self.assertEqual(result["quote_blocker"]["code"], "network_unavailable")

    def test_quote_jobs_retries_previous_network_failures(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a"), gpt_job("002", prompt="b")])

            def network_down(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                raise RuntimeError("NameResolutionError: Failed to resolve 'www.lovart.ai'")

            quote_client, _ = patch_quote_client(side_effect=network_down)
            with quote_client:
                first = quote_jobs(jobs_file, concurrency=1, progress=False)
            self.assertEqual(first["summary"]["failed_quote_remote_requests"], 1)
            self.assertEqual(first["summary"]["pending_quote_remote_requests"], 1)

            calls: list[str] = []

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                calls.append(str(body["prompt"]))
                return {"model": model, "quoted": True, "credits": 0.0, "payable_credits": 0.0, "warnings": []}

            quote_client, _ = patch_quote_client(side_effect=quoted)
            with quote_client:
                second = quote_jobs(jobs_file, progress=False)
            self.assertEqual(calls, ["a"])
            self.assertEqual(second["summary"]["quoted_remote_requests"], 2)
            self.assertEqual(second["summary"]["failed_quote_remote_requests"], 0)
            self.assertNotIn("quote_blocker", second)

    def test_quote_jobs_caps_high_concurrency_with_warning(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a")])
            quote_client, _ = patch_quote_client(return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "warnings": []})
            with quote_client:
                result = quote_jobs(jobs_file, concurrency=99, progress=False)
            self.assertIn("capped at 4", result["warnings"][0])

    def test_paid_batch_is_rejected_without_budget_and_not_submitted(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="paid")])
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model") as submit,
            ):
                with self.assertRaises(CreditRiskError):
                    run_jobs(jobs_file)
            submit.assert_not_called()

    def test_run_submits_all_jobs_before_polling(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a"), gpt_job("002", prompt="b")])
            events: list[str] = []

            def submit(model: str, body: dict[str, object], language: str = "en", mode: str = "auto") -> dict[str, object]:
                events.append(f"submit:{body['prompt']}")
                return {"data": {"task_id": f"task-{body['prompt']}"}}

            def task(task_id: str, language: str = "en") -> dict[str, object]:
                events.append(f"task:{task_id}")
                return {"status": "completed", "artifacts": [], "raw": {}}

            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model", side_effect=submit),
                patch("lovart_reverse.jobs.orchestrator.task_info", side_effect=task),
            ):
                result = run_jobs(jobs_file, wait=True, poll_interval=0)
            self.assertEqual(events, ["submit:a", "submit:b", "task:task-a", "task:task-b"])
            self.assertEqual(result["summary"]["status_counts"]["completed"], 2)
            self.assertEqual(result["summary"]["remote_requests"], 2)

    def test_resume_does_not_resubmit_existing_task_id(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001")])
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model", return_value={"task_id": "task-001"}),
            ):
                run_jobs(jobs_file)
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model") as submit,
                patch("lovart_reverse.jobs.orchestrator.task_info", return_value={"status": "completed", "artifacts": [], "raw": {}}),
            ):
                result = resume_jobs(jobs_file, wait=True, poll_interval=0)
            submit.assert_not_called()
            self.assertEqual(result["summary"]["status_counts"]["completed"], 1)

    def test_jobs_status_defaults_to_compact_payload(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="secret prompt")])

            def task(task_id: str, language: str = "en") -> dict[str, object]:
                return {"status": "running", "artifacts": [], "raw": {"prompt": "secret prompt"}}

            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model", return_value={"task_id": "task-001"}),
                patch("lovart_reverse.jobs.orchestrator.task_info", side_effect=task),
            ):
                run_jobs(jobs_file, wait=True, timeout_seconds=0, poll_interval=0)
            result = status_jobs(Path(tmp))
            serialized = json.dumps(result)
            self.assertEqual(result["operation"], "status")
            self.assertIn("tasks", result)
            self.assertNotIn("jobs", result)
            self.assertNotIn("remote_requests", result)
            self.assertNotIn("secret prompt", serialized)
            self.assertIn("lovart jobs resume", result["recommended_actions"][0])

    def test_jobs_status_full_detail_keeps_legacy_payload(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001")])
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model", return_value={"task_id": "task-001"}),
            ):
                run_jobs(jobs_file)
            result = status_jobs(Path(tmp), detail="full")
            self.assertIn("jobs", result)
            self.assertIn("remote_requests", result)

    def test_resume_downloads_already_completed_artifacts(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001")])
            artifact = {"content": "https://a.lovart.ai/artifacts/generator/test.png"}
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model", return_value={"task_id": "task-001"}),
                patch("lovart_reverse.jobs.orchestrator.task_info", return_value={"status": "completed", "artifacts": [artifact], "raw": {}}),
            ):
                run_jobs(jobs_file, wait=True, poll_interval=0)
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.download_artifacts", return_value=[{"path": "/tmp/test.png"}]) as download,
            ):
                result = resume_jobs(jobs_file, download=True, detail="summary")
            download.assert_called_once_with([artifact], task_id="task-001")
            self.assertEqual(result["summary"]["status_counts"]["downloaded"], 1)

    def test_resume_rejects_changed_jobs_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001")])
            with (
                patch("lovart_reverse.jobs.orchestrator.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.orchestrator.submit_model", return_value={"task_id": "task-001"}),
            ):
                run_jobs(jobs_file)
            write_jobs(jobs_file, [gpt_job("001", prompt="changed")])
            with self.assertRaises(InputError):
                resume_jobs(jobs_file)

    def test_jobs_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.application.jobs_dry_run_command", return_value={"operation": "dry_run", "summary": {}}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["jobs", "dry-run", "runs/fanren/jobs.jsonl"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["operation"], "dry_run")

    def test_jobs_quote_cli_accepts_v2_flags(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.application.jobs_quote_command", return_value={"operation": "quote", "summary": {}}) as quote_command,
            contextlib.redirect_stdout(output),
        ):
            code = main(
                [
                    "jobs",
                    "quote",
                    "--jobs-file",
                    "runs/fanren/jobs.jsonl",
                    "--detail",
                    "requests",
                    "--concurrency",
                    "99",
                    "--limit",
                    "10",
                    "--all",
                    "--refresh",
                    "--no-progress",
                ]
            )
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["operation"], "quote")
        self.assertEqual(quote_command.call_args.kwargs["detail"], "requests")
        self.assertEqual(quote_command.call_args.kwargs["concurrency"], 99)
        self.assertEqual(quote_command.call_args.kwargs["limit"], "10")
        self.assertTrue(quote_command.call_args.kwargs["all_requests"])
        self.assertTrue(quote_command.call_args.kwargs["refresh"])
        self.assertFalse(quote_command.call_args.kwargs["progress"])

    def test_jobs_quote_status_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.application.jobs_quote_status_command", return_value={"operation": "quote_status", "summary": {}}) as status_command,
            contextlib.redirect_stdout(output),
        ):
            code = main(["jobs", "quote-status", "runs/fanren", "--jobs-file", "runs/fanren/jobs.jsonl"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["operation"], "quote_status")
        self.assertEqual(str(status_command.call_args.kwargs["jobs_file"]), "runs/fanren/jobs.jsonl")


if __name__ == "__main__":
    unittest.main()
