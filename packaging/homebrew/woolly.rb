class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.11.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.11.0/woolly_darwin_arm64.tar.gz"
      sha256 "f5a6e7619cbe1429c99b3b08270ea87a15fc9a59044b2e279238cc692975433a"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.11.0/woolly_darwin_amd64.tar.gz"
      sha256 "58036fa41796dd51f0b06dd3ec3d7a15586917f500d76456274aebf1bb6126df"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.11.0/woolly_linux_arm64.tar.gz"
      sha256 "33fde0018c9b06ddb29766168bbbb97637092746f364182885e0bda003e9aefa"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.11.0/woolly_linux_amd64.tar.gz"
      sha256 "4bf76328580d5b43648b704d850c63159abe594c2e7311350f6190fa5cf2aa68"
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
