#!/bin/sh
# Builds the Woolly desktop app into app/src-tauri/target. On a Mac it builds
# one app for both Apple Silicon and Intel.
#   scripts/app.sh 0.10.0
set -eu

version="${1:-0.1.0-dev}"
cd "$(dirname "$0")/.."
export PATH="$HOME/.cargo/bin:$PATH"

(cd ui && npm run build >/dev/null)
bin=app/src-tauri/binaries
build() { CGO_ENABLED=0 GOOS=$1 GOARCH=$2 go build -trimpath -ldflags "-s -w -X main.version=$version" -o "$bin/$3" ./cmd/woolly; }

if [ "$(uname -s)" = Darwin ]; then
  build darwin arm64 woolly-aarch64-apple-darwin
  build darwin amd64 woolly-x86_64-apple-darwin
  lipo -create -output "$bin/woolly-universal-apple-darwin" "$bin/woolly-aarch64-apple-darwin" "$bin/woolly-x86_64-apple-darwin"
  target="--target universal-apple-darwin"
else
  triple=$(rustc -vV | sed -n 's/^host: //p')
  ext=""
  case "$triple" in *windows*) ext=".exe" ;; esac
  build "$(go env GOOS)" "$(go env GOARCH)" "woolly-$triple$ext"
  target=""
fi

cd app
[ -d node_modules ] || npm install --no-audit --no-fund
npx tauri build $target
