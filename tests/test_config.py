from __future__ import annotations

import contextlib
import io
import json
import unittest

from lovart_reverse.cli.main import main
from lovart_reverse.config import config_for_model, global_config


def field_map(model: str) -> dict[str, dict[str, object]]:
    return {field["key"]: field for field in config_for_model(model)["fields"]}


class ConfigDiscoveryTest(unittest.TestCase):
    def test_gpt_image_2_values_are_exhaustive(self) -> None:
        fields = field_map("openai/gpt-image-2")
        self.assertEqual(fields["quality"]["values"], ["auto", "high", "medium", "low"])
        self.assertEqual(
            fields["size"]["values"],
            ["1024*1024", "1536*1024", "1024*1536", "2048*2048", "2048*1152", "3840*2160", "2160*3840", "auto"],
        )
        self.assertEqual(fields["background"]["values"], ["auto", "opaque", "transparent"])
        self.assertEqual(fields["output_format"]["values"], ["png", "jpeg", "webp"])
        self.assertEqual(fields["moderation"]["values"], ["low", "auto"])
        self.assertEqual(fields["n"]["minimum"], 1)
        self.assertEqual(fields["n"]["maximum"], 10)
        self.assertEqual(fields["image"]["minItems"], 1)
        self.assertEqual(fields["image"]["maxItems"], 16)
        self.assertFalse(fields["prompt"]["enumerable"])
        self.assertTrue(fields["quality"]["quality_affecting"])
        self.assertTrue(fields["size"]["quality_affecting"])
        self.assertTrue(fields["size"]["cost_affecting"])
        self.assertEqual(fields["size"]["resolution_mapping"]["3840*2160"], "16:9(4k)")
        self.assertEqual(fields["prompt"]["route_role"], "free_input")

    def test_nano_banana_2_values_are_exhaustive(self) -> None:
        fields = field_map("vertex/nano-banana-2")
        self.assertEqual(
            fields["aspect_ratio"]["values"],
            ["8:1", "4:1", "21:9", "16:9", "3:2", "4:3", "5:4", "1:1", "4:5", "3:4", "2:3", "9:16", "1:4", "1:8"],
        )
        self.assertEqual(fields["resolution"]["values"], ["512", "1K", "2K", "4K"])
        self.assertEqual(fields["image"]["maxItems"], 20)
        self.assertFalse(fields["prompt"]["enumerable"])

    def test_seedream_max_images_is_quantity_field(self) -> None:
        fields = field_map("seedream/seedream-5-0")
        self.assertEqual(fields["max_images"]["minimum"], 1)
        self.assertEqual(fields["max_images"]["maximum"], 15)
        self.assertEqual(fields["max_images"]["route_role"], "quantity")
        self.assertTrue(fields["max_images"]["batch_relevant"])

    def test_seedance_2_values_are_exhaustive(self) -> None:
        fields = field_map("seedance/seedance-2-0")
        self.assertEqual(fields["duration"]["values"], [4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15])
        self.assertEqual(fields["ratio"]["values"], ["Auto", "16:9", "4:3", "1:1", "3:4", "9:16", "21:9"])
        self.assertEqual(fields["resolution"]["values"], ["480p", "720p", "1080p"])
        self.assertEqual(fields["generate_audio"]["values"], [True, False])
        self.assertEqual(fields["image_list"]["maxItems"], 9)
        self.assertEqual(fields["audio_list"]["maxItems"], 3)
        self.assertEqual(fields["video_list"]["maxItems"], 3)

    def test_kling_v3_values_are_exhaustive(self) -> None:
        fields = field_map("kling/kling-v3")
        self.assertEqual(fields["aspect_ratio"]["values"], ["16:9", "9:16", "1:1"])
        self.assertEqual(fields["duration"]["values"], ["3", "4", "5", "6", "7", "8", "9", "10", "11", "12", "13", "14", "15"])
        self.assertEqual(fields["mode"]["values"], ["std", "pro", "4k"])
        self.assertEqual(fields["sound"]["values"], ["on", "off"])

    def test_no_guess_example_values_are_legal(self) -> None:
        config = config_for_model("openai/gpt-image-2", example="zero_credit")
        fields = field_map("openai/gpt-image-2")
        body = config["example"]["body"]
        for key, value in body.items():
            values = fields[key].get("values")
            if isinstance(values, list):
                self.assertIn(value, values)

    def test_global_config_contract(self) -> None:
        cfg = global_config()
        self.assertIn("--allow-paid", cfg["generation_flags"])
        self.assertEqual(cfg["paid_policy"]["default"], "zero_credit_only")

    def test_config_cli_json_envelope(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["config", "openai/gpt-image-2"])
        self.assertEqual(code, 0)
        payload = json.loads(output.getvalue())
        self.assertTrue(payload["ok"])
        self.assertIn("fields", payload["data"])

    def test_config_unknown_model_errors(self) -> None:
        output = io.StringIO()
        with contextlib.redirect_stdout(output):
            code = main(["config", "unknown/model"])
        self.assertEqual(code, 2)
        payload = json.loads(output.getvalue())
        self.assertEqual(payload["error"]["code"], "schema_invalid")


if __name__ == "__main__":
    unittest.main()
