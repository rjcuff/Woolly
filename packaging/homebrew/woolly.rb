class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.8.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.8.0/woolly_darwin_arm64.tar.gz"
      sha256 "274e657dc4dee3351ac8eb7b08fe9a56b9bcef85030b35ad7ace8771db06e196"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.8.0/woolly_darwin_amd64.tar.gz"
      sha256 "196f9fd455918ff0d5c0a35409f768626d98a4bb5f79adb1db938600045a0f2e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.8.0/woolly_linux_arm64.tar.gz"
      sha256 "f57b7c2707706a5a1ec8e8349fda384034993a7071df484a3a6d4b6e5b606940"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.8.0/woolly_linux_amd64.tar.gz"
      sha256 "b1d4fcb4e33e5c80684bd315a95c457feb8cf3081d1551566a5c01f0faed780d"
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
