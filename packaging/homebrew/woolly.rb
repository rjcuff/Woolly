class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.7.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.7.0/woolly_darwin_arm64.tar.gz"
      sha256 "e9858dfb3e6c2c4364ee623dfded4798199db21e709c582e3c4322b9e8b95a96"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.7.0/woolly_darwin_amd64.tar.gz"
      sha256 "3d4ab96cfe882a6d122b3ab927871609c2341c6d6b7f7a93986dc4ae27f17e57"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.7.0/woolly_linux_arm64.tar.gz"
      sha256 "f54c5a4a77c97b805208c5854bbbe68ff42310cb0766993e6f7eb73eeb8aec7b"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.7.0/woolly_linux_amd64.tar.gz"
      sha256 "5b3b758221a759019b011959660ed46b4d7cf08d29fca003509f0040a681bde9"
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
