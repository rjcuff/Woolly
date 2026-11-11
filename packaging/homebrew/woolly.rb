class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.9.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.9.0/woolly_darwin_arm64.tar.gz"
      sha256 "95e7a95b83de215ace7d70249c94ea67f1dfc590bdb755b401ed80316e2ebc43"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.9.0/woolly_darwin_amd64.tar.gz"
      sha256 "c48bde7c526283f05ae3ce426f854bb33a42120ca08eb403ecb18ccf9d32ed2b"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.9.0/woolly_linux_arm64.tar.gz"
      sha256 "c7ba719418f0fbecf8aaf8e3e902a8039f8b38c964e46cda2e630355dd3fb75f"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.9.0/woolly_linux_amd64.tar.gz"
      sha256 "83a20da1e4b9668a660aabaf09402503b035804b075950ec82cec62d197d62ff"
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
