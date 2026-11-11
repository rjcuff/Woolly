class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.6.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.6.0/woolly_darwin_arm64.tar.gz"
      sha256 "30e024ba87cd7f48ca1c725b3d656fe1d3f754be6fef3f71ff8336afe03b7af5"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.6.0/woolly_darwin_amd64.tar.gz"
      sha256 "026e83ecbc2e63f0287fd61fa24af9deffbb063eee1990ce3389e046e5dc75cc"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.6.0/woolly_linux_arm64.tar.gz"
      sha256 "b27c140629d134c8dac3417021733a60f03ef9a389d0dc45945791010781fea1"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.6.0/woolly_linux_amd64.tar.gz"
      sha256 "150330a3b4f6ce9e9d9eb5c0c358d3ef4157a1e1718a7d2998a3ba1859cde2b7"
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
