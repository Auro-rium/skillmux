#!/usr/bin/env python3
import argparse
import json
from pathlib import Path

REPO = "Auro-rium/skillmux"
BASE = f"https://github.com/{REPO}/releases/download"


def parse_checksums(path: Path) -> dict[str, str]:
    result: dict[str, str] = {}
    for raw in path.read_text(encoding="utf-8").splitlines():
        raw = raw.strip()
        if not raw:
            continue
        parts = raw.split()
        if len(parts) < 2:
            raise SystemExit(f"invalid checksum line: {raw!r}")
        result[parts[-1].lstrip("*")] = parts[0].lower()
    return result


def require(checksums: dict[str, str], name: str) -> str:
    try:
        return checksums[name]
    except KeyError:
        raise SystemExit(f"missing checksum for {name}") from None


def homebrew(version: str, checksums: dict[str, str]) -> str:
    tag = f"v{version}"
    linux_amd64 = require(checksums, "skillmux_linux_amd64.tar.gz")
    linux_arm64 = require(checksums, "skillmux_linux_arm64.tar.gz")
    darwin_amd64 = require(checksums, "skillmux_darwin_amd64.tar.gz")
    darwin_arm64 = require(checksums, "skillmux_darwin_arm64.tar.gz")

    return f'''class Skillmux < Formula
  desc "Local-first SKILL.md manager across agent harnesses"
  homepage "https://github.com/{REPO}"
  version "{version}"

  on_macos do
    if Hardware::CPU.arm?
      url "{BASE}/{tag}/skillmux_darwin_arm64.tar.gz"
      sha256 "{darwin_arm64}"
    else
      url "{BASE}/{tag}/skillmux_darwin_amd64.tar.gz"
      sha256 "{darwin_amd64}"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "{BASE}/{tag}/skillmux_linux_arm64.tar.gz"
      sha256 "{linux_arm64}"
    else
      url "{BASE}/{tag}/skillmux_linux_amd64.tar.gz"
      sha256 "{linux_amd64}"
    end
  end

  def install
    bin.install "skillmux"
  end

  test do
    assert_match "skillmux v#{{version}}", shell_output("#{{bin}}/skillmux version")
  end
end
'''


def scoop(version: str, checksums: dict[str, str]) -> str:
    tag = f"v{version}"
    archive = "skillmux_windows_amd64.zip"
    manifest = {
        "version": version,
        "description": "Local-first SKILL.md manager across agent harnesses",
        "homepage": f"https://github.com/{REPO}",
        "architecture": {
            "64bit": {
                "url": f"{BASE}/{tag}/{archive}",
                "hash": require(checksums, archive),
            }
        },
        "bin": "skillmux.exe",
        "checkver": "github",
        "autoupdate": {
            "architecture": {
                "64bit": {
                    "url": f"{BASE}/v$version/{archive}",
                }
            }
        },
    }
    return json.dumps(manifest, indent=2) + "\n"


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--version", required=True)
    p.add_argument("--checksums", type=Path, required=True)
    p.add_argument("--root", type=Path, default=Path("."))
    args = p.parse_args()

    checksums = parse_checksums(args.checksums)

    formula = args.root / "Formula" / "skillmux.rb"
    bucket = args.root / "bucket" / "skillmux.json"
    formula.parent.mkdir(parents=True, exist_ok=True)
    bucket.parent.mkdir(parents=True, exist_ok=True)

    formula.write_text(homebrew(args.version, checksums), encoding="utf-8")
    bucket.write_text(scoop(args.version, checksums), encoding="utf-8")


if __name__ == "__main__":
    main()
