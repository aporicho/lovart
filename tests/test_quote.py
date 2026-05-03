from __future__ import annotations

import contextlib
import io
import json
import shutil
import unittest
from unittest.mock import Mock, patch

from lovart_reverse.cli.main import main
from lovart_reverse.errors import CreditRiskError, UnknownPricingError
from lovart_reverse.generation.gate import generation_gate
from lovart_reverse.pricing.quote import QuoteClient, quote
from lovart_reverse.pricing.web_parity import build_original_unit_data, pricing_input_args
from lovart_reverse.signing import PersistentSigner


class QuoteTest(unittest.TestCase):
    def test_quote_normalizes_lovart_pricing_response(self) -> None:
        response = Mock()
        response.json.return_value = {
            "code": 0,
            "message": "success",
            "data": {
                "balance": 27100,
                "price": 80,
                "price_detail": {"search_key": "4K_high", "unit_price": 80, "total_price": 120},
            },
        }
        with (
            patch("lovart_reverse.pricing.quote.sync_time", return_value=0),
            patch("lovart_reverse.pricing.quote.signed_headers", return_value={}),
            patch("lovart_reverse.pricing.quote.www_session") as session_factory,
        ):
            session = session_factory.return_value
            session.post.return_value = response
            result = quote("openai/gpt-image-2", {"prompt": "x", "quality": "high", "size": "3840*2160"})
        self.assertTrue(result["quoted"])
        self.assertEqual(result["credits"], 80.0)
        self.assertEqual(result["payable_credits"], 80.0)
        self.assertEqual(result["listed_credits"], 120.0)
        self.assertEqual(result["credit_basis"]["summary_total_credits"], "payable_credits")
        self.assertEqual(result["balance"], 27100)
        session.post.assert_called_once()
        payload = session.post.call_args.kwargs["json"]
        self.assertEqual(payload["generator_name"], "openai/gpt-image-2")
        self.assertTrue(result["request_shape"]["sent_original_unit_data"])

    def test_quote_client_syncs_time_once_for_multiple_quotes(self) -> None:
        response = Mock()
        response.json.return_value = {"code": 0, "message": "success", "data": {"balance": 1, "price": 0, "price_detail": {"total_price": 0}}}
        with (
            patch("lovart_reverse.pricing.quote.sync_time", return_value=123) as sync,
            patch("lovart_reverse.pricing.quote.signed_headers", return_value={}),
            patch("lovart_reverse.pricing.quote.www_session") as session_factory,
        ):
            session = session_factory.return_value
            session.post.return_value = response
            with QuoteClient(language="en") as client:
                client.quote("openai/gpt-image-2", {"prompt": "x", "quality": "medium", "size": "1024*1024"})
                client.quote("openai/gpt-image-2", {"prompt": "x", "quality": "high", "size": "1024*1024"})
        sync.assert_called_once()
        self.assertEqual(session.post.call_count, 2)

    def test_original_unit_data_is_internal_web_parity_payload(self) -> None:
        args, sent = pricing_input_args("openai/gpt-image-2", {"prompt": "secret", "quality": "high", "size": "1024*1536", "n": 4})
        self.assertTrue(sent)
        original = args["original_unit_data"]
        self.assertEqual(original["generator_name"], "openai/gpt-image-2")
        self.assertEqual(original["width"], 1024)
        self.assertEqual(original["height"], 1536)
        self.assertEqual(original["count"], 4)
        self.assertEqual(original["quality"], "high")

    def test_original_unit_data_handles_captured_models(self) -> None:
        anon = build_original_unit_data("vertex/anon-bob", {"prompt": "x", "resolution": "2K", "aspect_ratio": "4:3"})
        flux = build_original_unit_data("fal/flux-2-max", {"prompt": "x", "resolution": "2K", "aspect_ratio": "16:9"})
        midjourney = build_original_unit_data(
            "youchuan/midjourney",
            {"prompt": "x", "aspect_ratio": "16:9", "model": "niji7", "mode": "normal", "render_speed": "fast"},
        )
        self.assertEqual(anon["resolution"], "2K")
        self.assertEqual(anon["ratio"], "4:3")
        self.assertEqual(flux["generatorName"], "fal/flux-2-max")
        self.assertEqual(midjourney["model"], "niji7")
        self.assertEqual(midjourney["renderSpeed"], "fast")

    @unittest.skipUnless(shutil.which("node"), "node is required for persistent signer smoke")
    def test_persistent_signer_signs_multiple_requests(self) -> None:
        with PersistentSigner() as signer:
            first = signer.sign("1777810571005", "abc123")
            second = signer.sign("1777810571006", "abc124")
        self.assertTrue(first)
        self.assertTrue(second)
        self.assertNotEqual(first, second)

    def test_gate_allows_live_quote_zero_credit(self) -> None:
        with (
            patch("lovart_reverse.generation.gate.free_check", return_value={"zero_credit": False}),
            patch(
                "lovart_reverse.generation.gate.quote_or_unknown",
                return_value={"quoted": True, "credits": 0.0, "price_detail": {"search_key": "2K_low"}},
            ),
        ):
            result = generation_gate("openai/gpt-image-2", {"prompt": "x"}, mode="auto", allow_paid=False, max_credits=None, live=True)
        self.assertTrue(result["allowed"])
        self.assertEqual(result["reason"], "quote_zero_credit")

    def test_gate_live_quote_positive_overrides_entitlement(self) -> None:
        with (
            patch("lovart_reverse.generation.gate.free_check", return_value={"zero_credit": True}),
            patch(
                "lovart_reverse.generation.gate.quote_or_unknown",
                return_value={"quoted": True, "credits": 12.0, "price_detail": {"total_price": 12}},
            ),
        ):
            with self.assertRaises(CreditRiskError):
                generation_gate("openai/gpt-image-2", {"prompt": "x"}, mode="relax", allow_paid=False, max_credits=None, live=True)

    def test_gate_live_unknown_pricing_blocks_entitlement(self) -> None:
        with (
            patch("lovart_reverse.generation.gate.free_check", return_value={"zero_credit": True}),
            patch("lovart_reverse.generation.gate.quote_or_unknown", return_value={"quoted": False, "credits": None}),
        ):
            with self.assertRaises(UnknownPricingError):
                generation_gate("openai/gpt-image-2", {"prompt": "x"}, mode="relax", allow_paid=False, max_credits=None, live=True)

    def test_quote_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with (
            patch("lovart_reverse.commands.facade.quote", return_value={"model": "openai/gpt-image-2", "quoted": True, "credits": 12}),
            contextlib.redirect_stdout(output),
        ):
            code = main(["quote", "openai/gpt-image-2", "--body", '{"prompt":"x","quality":"medium","size":"2048*2048"}'])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertEqual(payload["data"]["credits"], 12)


if __name__ == "__main__":
    unittest.main()
