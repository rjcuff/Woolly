class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.13.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.1/woolly_darwin_arm64.tar.gz"
      sha256 "9bd4242ae34f44eff834454b427684d21a365bda42c5a510d1fd157a52373ce3"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.1/woolly_darwin_amd64.tar.gz"
      sha256 "b1d559a4e6c690c2ac11e1bb513bcfe64669f2db0fd9a2554be823d96a99722a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.1/woolly_linux_arm64.tar.gz"
      sha256 "15f445193eb504190b3038356c11a504251e4e2e9e863fe73e18cecacf74e1f3"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.13.1/woolly_linux_amd64.tar.gz"
      sha256 "3bd43d8f67f9943bf7cc112c75f9af7f3ce8faf4ebd27755645740ea5e2676f2"
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
