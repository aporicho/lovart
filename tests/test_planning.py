from __future__ import annotations

import contextlib
import io
import json
import unittest
from unittest.mock import patch

from lovart_reverse.cli.main import main
from lovart_reverse.config import config_for_model
from lovart_reverse.planning import plan_for_model
from lovart_reverse.pricing.table import PriceRow


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


class PlanningTest(unittest.TestCase):
    def setUp(self) -> None:
        self.rows = [
            PriceRow("GPT Image 2 1K", 1.0, "1 credits/image, low, no ref image", "image-model"),
            PriceRow("GPT Image 2 2K", 2.0, "2 credits/image, medium, no ref image", "image-model"),
        ]

    def test_gpt_image_2_returns_three_routes_with_legal_values(self) -> None:
        with (
            patch("lovart_reverse.planning.service.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.service.fetch_pricing_rows", return_value=self.rows),
            patch("lovart_reverse.planning.service.free_check", side_effect=fake_free),
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
            patch("lovart_reverse.planning.service.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.service.fetch_pricing_rows", return_value=self.rows),
            patch("lovart_reverse.planning.service.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", live=False)
        route = next(route for route in result["routes"] if route["id"] == "cost_best")
        self.assertTrue(route["zero_credit"])
        self.assertFalse(route["requires_paid_confirmation"])
        self.assertEqual(route["body_patch"]["quality"], "low")
        self.assertEqual(route["body_patch"]["size"], "1024*1024")

    def test_quality_best_requires_paid_confirmation_when_not_free(self) -> None:
        with (
            patch("lovart_reverse.planning.service.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.service.fetch_pricing_rows", return_value=self.rows),
            patch("lovart_reverse.planning.service.free_check", side_effect=fake_free),
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
            patch("lovart_reverse.planning.service.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.service.fetch_pricing_rows", return_value=self.rows),
            patch("lovart_reverse.planning.service.free_check", side_effect=recording_free),
        ):
            result = plan_for_model("openai/gpt-image-2", live=False)
        route = next(route for route in result["routes"] if route["id"] == "speed_best")
        self.assertEqual(route["mode"], "fast")
        self.assertIn("fast", calls)

    def test_free_input_fields_are_not_fabricated(self) -> None:
        with (
            patch("lovart_reverse.planning.service.setup_status", side_effect=fake_setup),
            patch("lovart_reverse.planning.service.fetch_pricing_rows", return_value=self.rows),
            patch("lovart_reverse.planning.service.free_check", side_effect=fake_free),
        ):
            result = plan_for_model("openai/gpt-image-2", partial_body={"prompt": "凡人流修仙概念设计"}, live=False)
        for route in result["routes"]:
            self.assertNotIn("prompt", route["body_patch"])
            self.assertEqual(route["request_body"]["prompt"], "凡人流修仙概念设计")

    def test_plan_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.main.plan_for_model", return_value={"model": "openai/gpt-image-2", "routes": []}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["plan", "openai/gpt-image-2", "--intent", "image-concept"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertIn("routes", payload["data"])

    def test_plan_unknown_model_errors(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["plan", "unknown/model", "--offline"])
        self.assertEqual(code, 2)
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "schema_invalid")


if __name__ == "__main__":
    unittest.main()
