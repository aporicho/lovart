from __future__ import annotations

import contextlib
import io
import json
import unittest
from unittest.mock import patch

from lovart_reverse.cli.main import main
from lovart_reverse.config import config_for_model
from lovart_reverse.planning.planner import plan_for_model


def fake_setup(*args: object, **kwargs: object) -> dict[str, object]:
    return {
        "ready": True,
        "real_generation_enabled": True,
        "recommended_actions": [],
        "update": {"status": "fresh"},
        "signer": {"maybe_stale": False},
    }


def fake_free(model: str, body: dict[str, object], mode: str = "auto", live: bool = True) -> dict[str, object]:
    zero = body.get("quality") == "low" and body.get("size") == "1024*1024"
    selected = "fast" if mode == "fast" else ("relax" if zero else mode)
    return {
        "model": model,
        "requested_mode": mode,
        "selected_mode": selected,
        "zero_credit": zero,
        "checks": [],
    }


def fake_quote(model: str, body: dict[str, object], live: bool = True) -> dict[str, object]:
    if body.get("quality") == "high" and body.get("size") in {"3840*2160", "2160*3840"}:
        credits = 80.0
    elif body.get("quality") == "low" and body.get("size") == "1024*1024":
        credits = 0.0
    else:
        credits = 12.0
    return {
        "model": model,
        "quoted": live,
        "credits": credits,
        "balance": 27100,
        "price_detail": {"search_key": f"{body.get('size')}_{body.get('quality')}"},
        "warnings": [],
    }


class PlanningTest(unittest.TestCase):
    def test_gpt_image_2_returns_three_routes_with_legal_values(self) -> None:
        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", intent="image-concept", live=False)
        self.assertEqual([route["id"] for route in result["routes"]], ["quality_best", "cost_best", "speed_best"])
        fields = {field["key"]: field for field in config_for_model("openai/gpt-image-2")["fields"]}
        for route in result["routes"]:
            for key, value in route["body_patch"].items():
                values = fields[key].get("values")
                if isinstance(values, list):
                    self.assertIn(value, values)

    def test_cost_best_prefers_zero_credit_route(self) -> None:
        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", live=False)
        route = next(route for route in result["routes"] if route["id"] == "cost_best")
        self.assertTrue(route["zero_credit"])
        self.assertFalse(route["requires_paid_confirmation"])
        self.assertEqual(route["body_patch"]["quality"], "low")
        self.assertEqual(route["body_patch"]["size"], "1024*1024")

    def test_quality_best_requires_paid_confirmation_when_not_free(self) -> None:
        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", live=False)
        route = next(route for route in result["routes"] if route["id"] == "quality_best")
        self.assertEqual(route["body_patch"]["quality"], "high")
        self.assertEqual(route["body_patch"]["size"], "3840*2160")
        self.assertTrue(route["requires_paid_confirmation"])

    def test_speed_best_checks_fast_mode(self) -> None:
        calls: list[str] = []

        def recording_free(model: str, body: dict[str, object], mode: str = "auto", live: bool = True) -> dict[str, object]:
            calls.append(mode)
            return fake_free(model, body, mode=mode, live=live)

        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.free_check", side_effect=recording_free),
        ):
            result = plan_for_model("openai/gpt-image-2", live=False)
        route = next(route for route in result["routes"] if route["id"] == "speed_best")
        self.assertEqual(route["mode"], "fast")
        self.assertIn("fast", calls)

    def test_live_quote_drives_cost_best_degradation(self) -> None:
        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.quote_or_unknown", side_effect=fake_quote),
            patch("lovart_reverse.planning.planner.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", live=True)
        quality = next(route for route in result["routes"] if route["id"] == "quality_best")
        cost = next(route for route in result["routes"] if route["id"] == "cost_best")
        self.assertEqual(quality["body_patch"]["quality"], "high")
        self.assertIn(quality["body_patch"]["size"], {"3840*2160", "2160*3840"})
        self.assertEqual(quality["quote"]["credits"], 80.0)
        self.assertEqual(cost["body_patch"]["quality"], "low")
        self.assertEqual(cost["body_patch"]["size"], "1024*1024")
        self.assertTrue(cost["quote"]["exact"])
        self.assertTrue(cost["zero_credit"])
        self.assertIn("quality: high -> low", cost["degraded_steps"])

    def test_free_input_fields_are_not_fabricated(self) -> None:
        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", partial_body={"prompt": "凡人流修仙概念设计"}, live=False)
        for route in result["routes"]:
            self.assertNotIn("prompt", route["body_patch"])
            self.assertEqual(route["request_body"]["prompt"], "凡人流修仙概念设计")

    def test_plan_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.application.plan_for_model", return_value={"model": "openai/gpt-image-2", "routes": []}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["plan", "openai/gpt-image-2", "--intent", "image-concept"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertIn("routes", payload["data"])

    def test_plan_cli_without_model_selects_image_candidates(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.planning.planner.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.planner.free_check", side_effect=fake_free),
            contextlib.redirect_stdout(output),
        ):
            code = main(["plan", "--intent", "image-concept", "--quote", "offline", "--candidate-limit", "2"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["planning_scope"], "all_models")
        self.assertEqual(len(payload["data"]["routes"]), 3)

    def test_plan_unknown_model_errors(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["plan", "unknown/model", "--offline"])
        self.assertEqual(code, 2)
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "schema_invalid")


if __name__ == "__main__":
    unittest.main()
