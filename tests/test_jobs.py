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
from lovart_reverse.jobs.service import quote_jobs, resume_jobs, run_jobs


def write_jobs(path: Path, jobs: list[dict[str, object]]) -> None:
    path.write_text("\n".join(json.dumps(job, ensure_ascii=False) for job in jobs) + "\n")


def gpt_job(job_id: str, prompt: str = "x") -> dict[str, object]:
    return {
        "job_id": job_id,
        "title": f"job {job_id}",
        "model": "openai/gpt-image-2",
        "mode": "auto",
        "body": {"prompt": prompt, "quality": "low", "size": "1024*1024"},
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
            write_jobs(jobs_file, [gpt_job("001", prompt="x"), gpt_job("002", prompt="y")])

            def quoted(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                credits = 0.0 if body["prompt"] == "x" else 12.0
                return {"model": model, "quoted": True, "credits": credits, "warnings": []}

            with patch("lovart_reverse.jobs.service.quote", side_effect=quoted):
                result = quote_jobs(jobs_file)
            self.assertEqual(result["summary"]["total_jobs"], 2)
            self.assertEqual(result["summary"]["total_credits"], 12.0)
            self.assertTrue((Path(tmp) / "jobs_quote.json").exists())

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
            with patch("lovart_reverse.jobs.service.quote", return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 0.0, "warnings": []}):
                result = quote_jobs(jobs_file)
            self.assertEqual(result["summary"]["logical_jobs"], 2)
            self.assertEqual(result["summary"]["requested_outputs"], 20)
            self.assertEqual(result["summary"]["remote_requests"], 2)
            self.assertEqual(result["remote_requests"][0]["body"]["n"], 10)

    def test_paid_batch_is_rejected_without_budget_and_not_submitted(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="paid")])
            with (
                patch("lovart_reverse.jobs.service.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.service.submit_model") as submit,
            ):
                with self.assertRaises(CreditRiskError):
                    run_jobs(jobs_file)
            submit.assert_not_called()

    def test_run_submits_all_jobs_before_polling(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001", prompt="a"), gpt_job("002", prompt="b")])
            events: list[str] = []

            def submit(model: str, body: dict[str, object], language: str = "en") -> dict[str, object]:
                events.append(f"submit:{body['prompt']}")
                return {"data": {"task_id": f"task-{body['prompt']}"}}

            def task(task_id: str) -> dict[str, object]:
                events.append(f"task:{task_id}")
                return {"status": "completed", "artifacts": [], "raw": {}}

            with (
                patch("lovart_reverse.jobs.service.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.service.submit_model", side_effect=submit),
                patch("lovart_reverse.jobs.service.task_info", side_effect=task),
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
                patch("lovart_reverse.jobs.service.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.service.submit_model", return_value={"task_id": "task-001"}),
            ):
                run_jobs(jobs_file)
            with (
                patch("lovart_reverse.jobs.service.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.service.submit_model") as submit,
                patch("lovart_reverse.jobs.service.task_info", return_value={"status": "completed", "artifacts": [], "raw": {}}),
            ):
                result = resume_jobs(jobs_file, wait=True, poll_interval=0)
            submit.assert_not_called()
            self.assertEqual(result["summary"]["status_counts"]["completed"], 1)

    def test_resume_rejects_changed_jobs_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            jobs_file = Path(tmp) / "jobs.jsonl"
            write_jobs(jobs_file, [gpt_job("001")])
            with (
                patch("lovart_reverse.jobs.service.generation_preflight", side_effect=fake_preflight),
                patch("lovart_reverse.jobs.service.submit_model", return_value={"task_id": "task-001"}),
            ):
                run_jobs(jobs_file)
            write_jobs(jobs_file, [gpt_job("001", prompt="changed")])
            with self.assertRaises(InputError):
                resume_jobs(jobs_file)

    def test_jobs_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.main.dry_run_jobs", return_value={"operation": "dry_run", "summary": {}}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["jobs", "dry-run", "runs/fanren/jobs.jsonl"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["operation"], "dry_run")


if __name__ == "__main__":
    unittest.main()
