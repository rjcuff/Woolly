class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.13.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.0/woolly_darwin_arm64.tar.gz"
      sha256 "525a9639045f4eaa656c81564d735ac397085ddc37b034871fac3e9b9ab139af"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.0/woolly_darwin_amd64.tar.gz"
      sha256 "a0675b3a79dc0066c3fe23fdac555d7312ee8f330279318f24473c1eb1215ec0"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.0/woolly_linux_arm64.tar.gz"
      sha256 "c280b969ca2c85d2f21cb1b16fa2d0e80d1c2e102560ffe2bd9ba684eacee571"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.0/woolly_linux_amd64.tar.gz"
      sha256 "b8be25ba4660d0fd63f88df5a4756e523ef706edcc4649f86101fb40f5e37e6d"
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
