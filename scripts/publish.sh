#!/bin/sh
# Tags a version, creates the GitHub release and uploads everything in dist/.
# Uses the GitHub login git already has saved (from VS Code or gh).
#   scripts/publish.sh 0.1.1
set -eu

version="${1:?usage: scripts/publish.sh <version>}"
tag="v$version"
repo="rjcuff/woolly"
cd "$(dirname "$0")/.."

[ -n "$(ls dist 2>/dev/null)" ] || { echo "dist/ is empty. Run: make release VERSION=$version" >&2; exit 1; }
[ -z "$(git status --porcelain)" ] || { echo "Commit your changes first." >&2; exit 1; }

token=$(printf "protocol=https\nhost=github.com\n\n" | git credential fill 2>/dev/null | sed -n 's/^password=//p')
[ -n "$token" ] || { echo "No saved GitHub login. Push once from VS Code, or run: gh auth login" >&2; exit 1; }

if ! git rev-parse "$tag" >/dev/null 2>&1; then
  git tag "$tag"
fi
git push origin main "$tag"

body=$(python3 -c '
import json, sys
v = sys.argv[1]
notes = f"Woolly {v}\n\n```sh\ncurl -fsSL https://usewoolly.com/install.sh | sh\n```\n\nDocs: https://usewoolly.com/docs"
print(json.dumps({"tag_name": "v" + v, "name": "v" + v, "body": notes, "draft": False, "prerelease": False, "make_latest": "true"}))
' "$version")

id=$(curl -fsS -X POST \
  -H "Authorization: Bearer $token" -H "Accept: application/vnd.github+json" \
  "https://api.github.com/repos/$repo/releases" -d "$body" |
  python3 -c 'import json, sys; print(json.loads(sys.stdin.read(), strict=False)["id"])')

for f in dist/*; do
  name=$(basename "$f")
  case "$name" in
    *.zip) type=application/zip ;;
    *.txt) type=text/plain ;;
    *) type=application/gzip ;;
  esac
  curl -fsS -o /dev/null -X POST \
    -H "Authorization: Bearer $token" -H "Content-Type: $type" \
    --data-binary @"$f" "https://uploads.github.com/repos/$repo/releases/$id/assets?name=$name"
  echo "uploaded $name"
done

echo "Published https://github.com/$repo/releases/tag/$tag"

# Update the Homebrew tap (github.com/rjcuff/homebrew-tap) if it exists.
tap="rjcuff/homebrew-tap"
if curl -fsS -o /dev/null -H "Authorization: Bearer $token" "https://api.github.com/repos/$tap"; then
  formula=$(scripts/homebrew.sh "$version")
  printf '%s\n' "$formula" > packaging/homebrew/woolly.rb
  sha=$(curl -fsS -H "Authorization: Bearer $token" "https://api.github.com/repos/$tap/contents/Formula/woolly.rb" 2>/dev/null |
    python3 -c 'import json, sys; print(json.loads(sys.stdin.read() or "{}", strict=False).get("sha", ""))' 2>/dev/null || true)
  python3 -c '
import base64, json, sys
body = {"message": "woolly " + sys.argv[1], "content": base64.b64encode(sys.stdin.buffer.read()).decode()}
if sys.argv[2]:
    body["sha"] = sys.argv[2]
print(json.dumps(body))
' "$version" "$sha" < packaging/homebrew/woolly.rb |
    curl -fsS -o /dev/null -X PUT -H "Authorization: Bearer $token" \
      "https://api.github.com/repos/$tap/contents/Formula/woolly.rb" -d @-
  echo "Updated Homebrew: brew install rjcuff/tap/woolly"
  echo "Commit packaging/homebrew/woolly.rb to keep the repo copy in sync."
else
  echo "No $tap repo yet, so Homebrew wasn't updated."
fi
