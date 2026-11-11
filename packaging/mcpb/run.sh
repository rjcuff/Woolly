#!/bin/sh
# Starts the right Woolly binary for this computer.
dir=$(cd "$(dirname "$0")" && pwd)
os=$(uname -s | tr '[:upper:]' '[:lower:]')
case $(uname -m) in
  arm64 | aarch64) arch=arm64 ;;
  *) arch=amd64 ;;
esac
bin="$dir/woolly-$os-$arch"
[ -x "$bin" ] || chmod +x "$bin" 2>/dev/null
exec "$bin" "$@"
