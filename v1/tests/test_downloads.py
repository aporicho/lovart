from __future__ import annotations

import unittest

from lovart_reverse.downloads.artifacts import _artifact_url


class DownloadArtifactsTest(unittest.TestCase):
    def test_artifact_url_accepts_lovart_content_field(self) -> None:
        artifact = {"type": "image", "content": "https://a.lovart.ai/artifacts/generator/example.png"}
        self.assertEqual(_artifact_url(artifact), "https://a.lovart.ai/artifacts/generator/example.png")

    def test_artifact_url_accepts_nested_content_object(self) -> None:
        artifact = {"type": "image", "content": {"url": "https://a.lovart.ai/artifacts/generator/example.png"}}
        self.assertEqual(_artifact_url(artifact), "https://a.lovart.ai/artifacts/generator/example.png")


if __name__ == "__main__":
    unittest.main()
