#!/bin/sh
# Installs the latest Woolly release.
#   curl -fsSL https://woolly.sh/install.sh | sh
set -eu

REPO="woolly/woolly"
INSTALL_DIR="${WOOLLY_INSTALL_DIR:-/usr/local/bin}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "Unsupported CPU: $arch" >&2; exit 1 ;;
esac
case "$os" in
  darwin | linux) ;;
  *) echo "Unsupported OS: $os. On Windows, download the zip from https://github.com/$REPO/releases" >&2; exit 1 ;;
esac

url="https://github.com/$REPO/releases/latest/download/woolly_${os}_${arch}.tar.gz"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading Woolly for $os/$arch..."
curl -fsSL "$url" -o "$tmp/woolly.tar.gz"
tar -xzf "$tmp/woolly.tar.gz" -C "$tmp"

if [ -w "$INSTALL_DIR" ]; then
  mv "$tmp/woolly" "$INSTALL_DIR/woolly"
else
  echo "Installing to $INSTALL_DIR (needs sudo)..."
  sudo mv "$tmp/woolly" "$INSTALL_DIR/woolly"
fi

echo "Installed: $("$INSTALL_DIR/woolly" version)"
echo "Next: woolly remember \"I prefer dark mode\""
