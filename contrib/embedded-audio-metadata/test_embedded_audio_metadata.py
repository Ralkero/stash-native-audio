import hashlib
from pathlib import Path
import tempfile
import unittest

from embedded_audio_metadata import GraphQL, classify_fields, read_mp3
from mutagen.id3 import APIC, COMM, ID3, TALB, TIT1, TIT2, TPE1, TPE2, TCON, TXXX


class EmbeddedMetadataTests(unittest.TestCase):
    def make_tagged_file(self, v2_version: int = 4) -> Path:
        path = Path(self.tempdir.name) / f"unicode-v2{v2_version}.mp3"
        tags = ID3()
        tags.add(TIT2(encoding=3, text=["  Café   Confession "]))
        tags.add(TPE1(encoding=3, text=["Fallback Artist"]))
        tags.add(TPE2(encoding=3, text=["YumPrincess"]))
        tags.add(TALB(encoding=3, text=["Erotic Audio"]))
        tags.add(TIT1(encoding=3, text=["Roleplay"]))
        tags.add(TCON(encoding=3, text=["Fallback Genre"]))
        tags.add(TXXX(encoding=3, desc="Stash Tags", text=["Goth; Tsundere; goth"]))
        tags.add(TXXX(encoding=3, desc="Stash Descriptors", text=["F4M; Fdom to Fsub"]))
        tags.add(TXXX(encoding=3, desc="Audience", text=["Adults"]))
        tags.add(TXXX(encoding=3, desc="Content Type", text=["Erotic Audio"]))
        tags.add(TXXX(encoding=3, desc="Original Filename", text=["original name.mp3"]))
        tags.add(COMM(encoding=3, lang="eng", desc="Description", text=["  A   description. "]))
        tags.add(APIC(encoding=3, mime="image/jpeg", type=3, desc="Cover", data=b"fake-image"))
        tags.save(path, v2_version=v2_version)
        return path

    def setUp(self):
        self.tempdir = tempfile.TemporaryDirectory()

    def tearDown(self):
        self.tempdir.cleanup()

    def test_mapping_normalization_and_read_only_behavior(self):
        path = self.make_tagged_file()
        before = hashlib.sha256(path.read_bytes()).hexdigest()
        metadata = read_mp3(str(path))
        after = hashlib.sha256(path.read_bytes()).hexdigest()
        self.assertEqual(before, after)
        self.assertEqual(metadata["title"], "Café Confession")
        self.assertEqual(metadata["authors"], ["YumPrincess"])
        self.assertEqual(metadata["genres"], ["Goth", "Tsundere"])
        self.assertEqual(metadata["descriptors"], ["F4M", "Fdom to Fsub"])
        self.assertEqual(metadata["details"], "A description.")
        self.assertTrue(metadata["cover_hash"])

    def test_id3v23_and_id3v24_are_supported(self):
        for version in (3, 4):
            with self.subTest(version=version):
                metadata = read_mp3(str(self.make_tagged_file(version)))
                self.assertEqual(metadata["title"], "Café Confession")
                self.assertEqual(metadata["authors"], ["YumPrincess"])

    def test_manual_edit_becomes_conflict(self):
        current = {"title": "Manual title"}
        incoming = {"title": "File title"}
        result = classify_fields(current, incoming, {"title": "Previously imported"})
        self.assertIn("title", result["conflicts"])

    def test_unchanged_previous_import_can_advance(self):
        current = {"title": "Previously imported"}
        incoming = {"title": "File title"}
        result = classify_fields(current, incoming, {"title": "Previously imported"})
        self.assertEqual(result["changes"]["title"], "File title")

    def test_wildcard_server_host_uses_loopback(self):
        self.assertEqual(GraphQL({"Host": "0.0.0.0", "Port": 9999}).url, "http://127.0.0.1:9999/graphql")
        self.assertEqual(GraphQL({"Host": "::", "Port": 9999}).url, "http://127.0.0.1:9999/graphql")


if __name__ == "__main__":
    unittest.main()
