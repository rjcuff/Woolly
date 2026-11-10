#!/bin/sh
# Installs the latest Woolly release.
#   curl -fsSL https://usewoolly.com/install.sh | sh
set -eu

REPO="rjcuff/woolly"
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

file="woolly_${os}_${arch}.tar.gz"
base="https://github.com/$REPO/releases/latest/download"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading Woolly for $os/$arch..."
curl -fsSL "$base/$file" -o "$tmp/$file"
curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt"

# Make sure the download matches the checksum published with the release.
expected=$(grep " $file\$" "$tmp/checksums.txt" | awk '{print $1}')
if command -v shasum >/dev/null 2>&1; then
  actual=$(shasum -a 256 "$tmp/$file" | awk '{print $1}')
elif command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$file" | awk '{print $1}')
else
  echo "Can't verify the download: no shasum or sha256sum found." >&2
  exit 1
fi
if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
  echo "The download doesn't match the published checksum. Nothing was installed." >&2
  exit 1
fi
echo "Verified checksum."

tar -xzf "$tmp/$file" -C "$tmp"

# Use sudo only if the folder (or where it would be created) isn't writable.
target="$INSTALL_DIR"
while [ ! -d "$target" ]; do target=$(dirname "$target"); done
if [ -w "$target" ]; then
  sudo=""
else
  echo "Installing to $INSTALL_DIR (needs your password)..."
  sudo="sudo"
fi
$sudo mkdir -p "$INSTALL_DIR"
$sudo mv "$tmp/woolly" "$INSTALL_DIR/woolly"
$sudo chmod 755 "$INSTALL_DIR/woolly"

echo "Installed: $("$INSTALL_DIR/woolly" version)"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *)
    echo
    echo "$INSTALL_DIR isn't on your PATH yet. Add it with:"
    echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> ~/.zshrc && source ~/.zshrc"
    ;;
esac
echo "Next: woolly remember \"I prefer dark mode\""
