from __future__ import annotations

import contextlib
import io
import json
import unittest
from unittest.mock import Mock, patch

from lovart_reverse.cli.main import main
from lovart_reverse.generation.gate import generation_gate
from lovart_reverse.pricing.quote import quote


class QuoteTest(unittest.TestCase):
    def test_quote_normalizes_lovart_pricing_response(self) -> None:
        response = Mock()
        response.json.return_value = {
            "code": 0,
            "message": "success",
            "data": {
                "balance": 27100,
                "price": 80,
                "price_detail": {"search_key": "4K_high", "unit_price": 80},
            },
        }
        with patch("lovart_reverse.pricing.quote.lgw_request", return_value=response) as lgw:
            result = quote("openai/gpt-image-2", {"prompt": "x", "quality": "high", "size": "3840*2160"})
        self.assertTrue(result["quoted"])
        self.assertEqual(result["credits"], 80.0)
        self.assertEqual(result["balance"], 27100)
        lgw.assert_called_once()
        self.assertEqual(lgw.call_args.args[:2], ("POST", "/v1/generator/pricing"))
        self.assertEqual(lgw.call_args.kwargs["body"]["generator_name"], "openai/gpt-image-2")

    def test_gate_allows_live_quote_zero_credit(self) -> None:
        with (
            patch("lovart_reverse.generation.gate.free_check", return_value={"zero_credit": False}),
            patch(
                "lovart_reverse.generation.gate.quote_or_estimate",
                return_value={"quoted": True, "estimated": True, "credits": 0.0, "price_detail": {"search_key": "2K_low"}},
            ),
        ):
            result = generation_gate("openai/gpt-image-2", {"prompt": "x"}, [], mode="auto", allow_paid=False, max_credits=None, live=True)
        self.assertTrue(result["allowed"])
        self.assertEqual(result["reason"], "quote_zero_credit")

    def test_quote_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.cli.main.quote", return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 12}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["quote", "openai/gpt-image-2", "--body", '{"prompt":"x","quality":"medium","size":"2048*2048"}'])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["credits"], 12)


if __name__ == "__main__":
    unittest.main()
