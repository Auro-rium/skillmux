#!/bin/sh
set -eu

REPO="Auro-rium/skillmux"
VERSION="${SKILLMUX_VERSION:-latest}"
BIN_DIR="${SKILLMUX_BIN_DIR:-$HOME/.local/bin}"

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) echo "skillmux: unsupported OS for this installer" >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "skillmux: unsupported architecture" >&2; exit 1 ;;
esac

archive="skillmux_${os}_${arch}.tar.gz"

if [ "$VERSION" = "latest" ]; then
  base="https://github.com/$REPO/releases/latest/download"
else
  case "$VERSION" in
    v*) tag="$VERSION" ;;
    *) tag="v$VERSION" ;;
  esac
  base="https://github.com/$REPO/releases/download/$tag"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

expected="$(awk -v f="$archive" '$2 == f || $2 == "*" f { print $1; exit }' "$tmp/checksums.txt")"
if [ -z "$expected" ]; then
  echo "skillmux: checksum entry for $archive was not found" >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$archive" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp/$archive" | awk '{print $1}')"
else
  echo "skillmux: neither sha256sum nor shasum is available" >&2
  exit 1
fi

if [ "$expected" != "$actual" ]; then
  echo "skillmux: checksum verification failed for $archive" >&2
  exit 1
fi

mkdir -p "$tmp/extract" "$BIN_DIR"
tar -xzf "$tmp/$archive" -C "$tmp/extract"
install -m 0755 "$tmp/extract/skillmux" "$BIN_DIR/skillmux"

echo "Installed skillmux to $BIN_DIR/skillmux"
"$BIN_DIR/skillmux" version

case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *)
    echo
    echo "$BIN_DIR is not currently on PATH."
    echo "Add it to PATH, or run: $BIN_DIR/skillmux"
    ;;
esac
