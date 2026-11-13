class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.12.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.12.0/woolly_darwin_arm64.tar.gz"
      sha256 "eb55927d36a62c0b7c0c7b1eb063dde06a5ac49a561e2f084447246a21a74d4e"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.12.0/woolly_darwin_amd64.tar.gz"
      sha256 "e77e97801ec763761d8637f423bf9b22e64c7ee1a07a8f5e06779ebb6c9d9f82"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.12.0/woolly_linux_arm64.tar.gz"
      sha256 "5e51db2a4c83fe8e5c90a4b658083d2b2d6735b42485e719bc33fca5df680d19"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.12.0/woolly_linux_amd64.tar.gz"
      sha256 "05e852132e00881ab875fd527161613895049c0e7b6588c5b77d45d451b4556d"
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
