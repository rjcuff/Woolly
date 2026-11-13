#!/bin/sh
# Builds the Woolly desktop app into app/src-tauri/target. On a Mac it builds
# one app for both Apple Silicon and Intel, and when a Developer ID
# certificate and a notarytool profile named "woolly" are set up, it signs
# and notarizes it and writes dist-app/Woolly-mac.dmg. On Windows it writes
# the installer to dist-app/Woolly-windows.exe.
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

identity=""
if [ "$(uname -s)" = Darwin ]; then
  identity=$(security find-identity -v -p codesigning | sed -n 's/.*"\(Developer ID Application: [^"]*\)".*/\1/p' | head -1)
fi
if [ -n "$identity" ]; then
  export APPLE_SIGNING_IDENTITY="$identity"
  echo "Signing as $identity"
else
  echo "No Developer ID certificate found, so the app won't be signed."
fi

(cd app && { [ -d node_modules ] || npm install --no-audit --no-fund; } && npx tauri build $target)

case "$(uname -s)" in
  MINGW* | MSYS* | CYGWIN*)
    mkdir -p dist-app
    setup=$(ls app/src-tauri/target/release/bundle/nsis/Woolly_*_x64-setup.exe | tail -1)
    cp "$setup" dist-app/Woolly-windows.exe
    echo "Built dist-app/Woolly-windows.exe"
    exit 0 ;;
esac
[ "$(uname -s)" = Darwin ] || exit 0
app=app/src-tauri/target/universal-apple-darwin/release/bundle/macos/Woolly.app
out=dist-app
mkdir -p "$out"
rm -f "$out/Woolly-mac.dmg"

notarize() {
  xcrun notarytool submit "$1" --keychain-profile woolly --wait
}
can_notarize=false
if [ -n "$identity" ] && xcrun notarytool history --keychain-profile woolly >/dev/null 2>&1; then
  can_notarize=true
fi

if $can_notarize; then
  echo "Notarizing the app with Apple…"
  ditto -c -k --keepParent "$app" "$out/Woolly.zip"
  notarize "$out/Woolly.zip"
  rm "$out/Woolly.zip"
  xcrun stapler staple "$app"
fi

# A disk image with the app and a shortcut to Applications, for drag-to-install.
stage=$(mktemp -d)
cp -R "$app" "$stage/"
ln -s /Applications "$stage/Applications"
hdiutil create -volname Woolly -srcfolder "$stage" -ov -format UDZO "$out/Woolly-mac.dmg" >/dev/null
rm -rf "$stage"

if $can_notarize; then
  codesign --sign "$identity" --timestamp "$out/Woolly-mac.dmg"
  notarize "$out/Woolly-mac.dmg"
  xcrun stapler staple "$out/Woolly-mac.dmg"
  spctl -a -t open --context context:primary-signature -v "$out/Woolly-mac.dmg"
  echo "Signed and notarized: $out/Woolly-mac.dmg"
elif [ -n "$identity" ]; then
  echo "Signed but not notarized. Set up notarization with:"
  echo "  xcrun notarytool store-credentials woolly --apple-id <your Apple ID> --team-id <team ID>"
else
  echo "Unsigned: $out/Woolly-mac.dmg"
fi
