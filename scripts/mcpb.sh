#!/bin/sh
# Builds dist/woolly.mcpb, a bundle with Woolly for every platform that
# Claude Desktop installs with a double-click, and fills in server.json for
# the MCP Registry. Run after scripts/release.sh.
#   scripts/mcpb.sh 0.7.0
set -eu

version="${1:?usage: scripts/mcpb.sh <version>}"
cd "$(dirname "$0")/.."
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
mkdir -p "$work/bundle/server"

for target in darwin_amd64 darwin_arm64 linux_amd64 linux_arm64; do
  tar -xzf "dist/woolly_$target.tar.gz" -C "$work" woolly
  mv "$work/woolly" "$work/bundle/server/woolly-$(echo "$target" | tr _ -)"
done
unzip -q -o "dist/woolly_windows_amd64.zip" woolly.exe -d "$work/bundle/server"
cp packaging/mcpb/run.sh "$work/bundle/server/run.sh"
chmod +x "$work/bundle/server/"*
cp packaging/mcpb/icon.png LICENSE "$work/bundle/"
sed "s/\"VERSION\"/\"$version\"/" packaging/mcpb/manifest.json > "$work/bundle/manifest.json"

rm -f dist/woolly.mcpb
(cd "$work/bundle" && zip -q -r -X "$OLDPWD/dist/woolly.mcpb" .)

sha=$(shasum -a 256 dist/woolly.mcpb | cut -d' ' -f1)
(cd dist && shasum -a 256 woolly.mcpb >> checksums.txt)
python3 - "$version" "$sha" <<'PY'
import json, sys
version, sha = sys.argv[1], sys.argv[2]
with open("server.json") as f:
    s = json.load(f)
s["version"] = version
p = s["packages"][0]
p["identifier"] = f"https://github.com/rjcuff/woolly/releases/download/v{version}/woolly.mcpb"
p["fileSha256"] = sha
with open("server.json", "w") as f:
    json.dump(s, f, indent=2)
    f.write("\n")
PY
echo "Built dist/woolly.mcpb ($sha)"
