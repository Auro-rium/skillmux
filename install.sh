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

if [ "$VERSION" = "latest" ]; then
  url="https://github.com/$REPO/releases/latest/download/skillmux_$os_$arch.tar.gz"
else
  url="https://github.com/$REPO/releases/download/$VERSION/skillmux_$os_$arch.tar.gz"
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT HUP INT TERM
mkdir -p "$BIN_DIR"

echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/skillmux.tar.gz"
tar -xzf "$tmp/skillmux.tar.gz" -C "$tmp"
install -m 0755 "$tmp/skillmux" "$BIN_DIR/skillmux"

echo "Installed skillmux to $BIN_DIR/skillmux"
case ":$PATH:" in
  *":$BIN_DIR:"*) ;;
  *) echo "Add $BIN_DIR to PATH if it is not already there." ;;
esac
