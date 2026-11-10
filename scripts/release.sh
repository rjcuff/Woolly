#!/bin/sh
# Builds release archives into dist/ for every supported platform.
#   scripts/release.sh 0.1.0
set -eu

version="${1:?usage: scripts/release.sh <version>}"
cd "$(dirname "$0")/.."
rm -rf dist
mkdir -p dist

for target in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do
  os=${target%/*}
  arch=${target#*/}
  name="woolly_${os}_${arch}"
  dir="dist/$name"
  bin="woolly"
  [ "$os" = windows ] && bin="woolly.exe"

  mkdir -p "$dir"
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$dir/$bin" ./cmd/woolly
  cp LICENSE README.md "$dir/"

  if [ "$os" = windows ]; then
    (cd "$dir" && zip -q "../$name.zip" "$bin" LICENSE README.md)
  else
    tar -czf "dist/$name.tar.gz" -C "$dir" "$bin" LICENSE README.md
  fi
  rm -rf "$dir"
done

(cd dist && shasum -a 256 woolly_* > checksums.txt)
echo "Built woolly $version into dist/"
ls -1 dist
