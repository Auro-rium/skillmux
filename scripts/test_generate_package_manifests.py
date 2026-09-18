import json
import tempfile
import unittest
from pathlib import Path

from generate_package_manifests import homebrew, parse_checksums, scoop


class PackageManifestTests(unittest.TestCase):
    def setUp(self):
        self.checksums = {
            "skillmux_linux_amd64.tar.gz": "1" * 64,
            "skillmux_linux_arm64.tar.gz": "2" * 64,
            "skillmux_darwin_amd64.tar.gz": "3" * 64,
            "skillmux_darwin_arm64.tar.gz": "4" * 64,
            "skillmux_windows_amd64.zip": "5" * 64,
        }

    def test_homebrew_manifest_points_at_exact_release_assets(self):
        formula = homebrew("0.1.0", self.checksums)
        self.assertIn('version "0.1.0"', formula)
        self.assertIn("/releases/download/v0.1.0/skillmux_darwin_arm64.tar.gz", formula)
        self.assertIn("/releases/download/v0.1.0/skillmux_linux_amd64.tar.gz", formula)
        self.assertIn("4" * 64, formula)
        self.assertIn("1" * 64, formula)
        self.assertIn('assert_match "skillmux v#{version}"', formula)
        self.assertIn('shell_output("#{bin}/skillmux version")', formula)

    def test_scoop_manifest_is_valid_and_pinned(self):
        manifest = json.loads(scoop("0.1.0", self.checksums))
        self.assertEqual(manifest["version"], "0.1.0")
        self.assertEqual(manifest["checkver"], "github")
        self.assertEqual(manifest["architecture"]["64bit"]["hash"], "5" * 64)
        self.assertTrue(
            manifest["architecture"]["64bit"]["url"].endswith(
                "/releases/download/v0.1.0/skillmux_windows_amd64.zip"
            )
        )

    def test_checksum_parser_accepts_sha256sum_output(self):
        with tempfile.TemporaryDirectory() as td:
            p = Path(td) / "checksums.txt"
            p.write_text(
                ("a" * 64) + "  skillmux_linux_amd64.tar.gz\n",
                encoding="utf-8",
            )
            parsed = parse_checksums(p)
        self.assertEqual(parsed["skillmux_linux_amd64.tar.gz"], "a" * 64)


if __name__ == "__main__":
    unittest.main()
