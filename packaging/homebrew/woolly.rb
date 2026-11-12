class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.10.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.10.0/woolly_darwin_arm64.tar.gz"
      sha256 "a19224d2e03654e0878b7a4ff10cbaa9b45407d8e7e2c3ed91f0538845415a62"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.10.0/woolly_darwin_amd64.tar.gz"
      sha256 "b8dc62ce9517eff7bbe07272a063d9fc90bfd71ba9ea34d82d80dc2f40f4341a"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.10.0/woolly_linux_arm64.tar.gz"
      sha256 "aba2844f1dbef3d75047ad1458d9ae3e3fd1018e47aaafda40891a2010d887d5"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.10.0/woolly_linux_amd64.tar.gz"
      sha256 "22d027deb829834063e8d70e2ed84306b7f0296c2fe5af8bb0db5970af8509e4"
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
