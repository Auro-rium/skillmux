#!/bin/sh
set -eu

VERSION="${1:?usage: build_release.sh <version>}"

rm -rf dist build
mkdir -p dist build

build_unix() {
  os="$1"
  arch="$2"
  out="build/$os-$arch"
  mkdir -p "$out"

  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
    -o "$out/skillmux" ./cmd/skillmux

  tar -C "$out" -czf "dist/skillmux_${os}_${arch}.tar.gz" skillmux
}

build_windows() {
  arch="$1"
  out="build/windows-$arch"
  mkdir -p "$out"

  CGO_ENABLED=0 GOOS=windows GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=$VERSION" \
    -o "$out/skillmux.exe" ./cmd/skillmux

  (cd "$out" && zip -q "../../dist/skillmux_windows_${arch}.zip" skillmux.exe)
}

build_unix linux amd64
build_unix linux arm64
build_unix darwin amd64
build_unix darwin arm64
build_windows amd64

cp install.sh install.ps1 dist/

(
  cd dist
  sha256sum skillmux_* install.sh install.ps1 > checksums.txt
)
