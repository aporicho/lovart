from __future__ import annotations

import unittest
from unittest.mock import patch

from lovart_reverse.entitlement.checks import free_check


FAST_PAYLOAD = {
    "code": 0,
    "data": {
        "unlimited": True,
        "unlimited_list": [
            {
                "model_display_name": "GPT Image 2",
                "status": 1,
                "extraItem": "low 2K",
                "alias_list": ["openai/gpt-image-2", "generate_image_gpt_image_2_low"],
            }
        ],
    },
}


class EntitlementTest(unittest.TestCase):
    def test_fast_zero_credit_matches_alias_and_extra_item(self) -> None:
        with patch("lovart_reverse.entitlement.checks.fetch_unlimited", return_value=(FAST_PAYLOAD, "fixture")):
            result = free_check(
                "openai/gpt-image-2",
                {"prompt": "x", "quality": "low", "size": "1024*1024"},
                mode="fast",
                live=False,
            )
        self.assertTrue(result["zero_credit"])
        self.assertEqual(result["selected_mode"], "fast")


if __name__ == "__main__":
    unittest.main()
