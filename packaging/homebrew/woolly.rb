class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.4.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.4.0/woolly_darwin_arm64.tar.gz"
      sha256 "9dcfb416c6cc6a517fe462d020b9b63bb115b5c558e20a24e11af8e3b3dff1f4"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.4.0/woolly_darwin_amd64.tar.gz"
      sha256 "e612d689602a522d63b88cb0948d44cc69dd5304ca9280fb0b6ae0db5941cad4"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.4.0/woolly_linux_arm64.tar.gz"
      sha256 "538688ec47d3cccb1b4127671fa7c03ab25f2a4b52c896e56f0b2b945a90710d"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.4.0/woolly_linux_amd64.tar.gz"
      sha256 "5f71be58a2d4724b985a27601b80e051b980c11084a7e37b562842d9b81ef6d1"
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
