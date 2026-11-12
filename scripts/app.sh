#!/bin/sh
# Builds the Woolly desktop app for this computer into app/src-tauri/target.
#   scripts/app.sh 0.10.0
set -eu

version="${1:-0.1.0-dev}"
cd "$(dirname "$0")/.."
export PATH="$HOME/.cargo/bin:$PATH"
triple=$(rustc -vV | sed -n 's/^host: //p')
ext=""
case "$triple" in *windows*) ext=".exe" ;; esac

(cd ui && npm run build >/dev/null)
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$version" \
  -o "app/src-tauri/binaries/woolly-$triple$ext" ./cmd/woolly

cd app
[ -d node_modules ] || npm install --no-audit --no-fund
npx tauri build
