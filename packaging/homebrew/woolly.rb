class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.3.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.3.0/woolly_darwin_arm64.tar.gz"
      sha256 "d4d69167a3538c022372613c8de0224111ee8bb7a49b52003ebdc308ef9e00f6"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.3.0/woolly_darwin_amd64.tar.gz"
      sha256 "bfb22909a62e0314c7a41c9f40013dcb49b6983bfec452a593f5ce5caf5d3afd"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.3.0/woolly_linux_arm64.tar.gz"
      sha256 "568be1b24533f662abbb6cd8e0295d5a7dccfec76007c2269e66919b230b3e6f"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.3.0/woolly_linux_amd64.tar.gz"
      sha256 "9feda8bbb9b1329f6a4d225a28e8ef3769ff83374aec88badab1178780671861"
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
