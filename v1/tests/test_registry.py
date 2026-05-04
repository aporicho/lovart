from __future__ import annotations

import unittest

from lovart_reverse.registry import load_ref_registry, model_records, validate_body


class RegistryTest(unittest.TestCase):
    def test_ref_registry_has_expected_models(self) -> None:
        records = model_records(load_ref_registry())
        self.assertEqual(len(records), 55)
        self.assertIn("openai/gpt-image-2", {record.model for record in records})

    def test_basic_schema_validation(self) -> None:
        errors = validate_body(load_ref_registry(), "openai/gpt-image-2", {"prompt": "test", "quality": "low"})
        self.assertEqual(errors, [])


if __name__ == "__main__":
    unittest.main()
