#!/bin/sh
# Prints the Homebrew formula for a release, using the checksums in dist/.
#   scripts/homebrew.sh 0.3.0 > Formula/woolly.rb
set -eu

version="${1:?usage: scripts/homebrew.sh <version>}"
cd "$(dirname "$0")/.."
sums=dist/checksums.txt
[ -f "$sums" ] || { echo "No $sums. Run: make release VERSION=$version" >&2; exit 1; }

sha() { grep " woolly_$1.tar.gz\$" "$sums" | awk '{print $1}'; }
base="https://github.com/rjcuff/woolly/releases/download/v$version"

cat <<RUBY
class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "$version"
  license "MIT"

  on_macos do
    on_arm do
      url "$base/woolly_darwin_arm64.tar.gz"
      sha256 "$(sha darwin_arm64)"
    end
    on_intel do
      url "$base/woolly_darwin_amd64.tar.gz"
      sha256 "$(sha darwin_amd64)"
    end
  end

  on_linux do
    on_arm do
      url "$base/woolly_linux_arm64.tar.gz"
      sha256 "$(sha linux_arm64)"
    end
    on_intel do
      url "$base/woolly_linux_amd64.tar.gz"
      sha256 "$(sha linux_amd64)"
    end
  end

  def install
    bin.install "woolly"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/woolly version")
    ENV["WOOLLY_HOME"] = testpath/"woolly"
    ENV["WOOLLY_NO_EMBED"] = "1"
    system bin/"woolly", "remember", "Homebrew test"
    assert_match "Homebrew test", shell_output("#{bin}/woolly list")
  end
end
RUBY
