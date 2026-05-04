from __future__ import annotations

import unittest

from lovart_reverse.io_json import hash_value
from lovart_reverse.update.manifest import manifest_from_parts


class UpdateTest(unittest.TestCase):
    def test_manifest_hash_fields_are_stable(self) -> None:
        manifest = manifest_from_parts({"generator_list_hash": hash_value({"b": 1, "a": 2})})
        self.assertEqual(manifest["generator_list_hash"], hash_value({"a": 2, "b": 1}))


if __name__ == "__main__":
    unittest.main()
