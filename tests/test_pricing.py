from __future__ import annotations

import unittest

from lovart_reverse.pricing.estimator import estimate
from lovart_reverse.pricing.table import PriceRow


class PricingTest(unittest.TestCase):
    def test_gpt_image_2_low_1k(self) -> None:
        rows = [PriceRow("GPT Image 2 1K", 1.0, "1 credits/image, low, no ref image", "image-model")]
        result = estimate("openai/gpt-image-2", {"prompt": "x", "quality": "low", "size": "1024*1024"}, rows)
        self.assertTrue(result["estimated"])
        self.assertEqual(result["credits"], 1.0)


if __name__ == "__main__":
    unittest.main()
