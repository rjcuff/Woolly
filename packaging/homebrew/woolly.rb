class Woolly < Formula
  desc "Local memory for your AI tools"
  homepage "https://usewoolly.com"
  version "0.5.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.5.0/woolly_darwin_arm64.tar.gz"
      sha256 "07500b2c5ce0b30ab5bb265a7a2868ad096d31caa54fd385f31c76c647ae9305"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.5.0/woolly_darwin_amd64.tar.gz"
      sha256 "5d69eac15e74f8e62310a49d21b4f6272e7be23be28f4a8fa24ec1f7c06e8a27"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/rjcuff/woolly/releases/download/v0.5.0/woolly_linux_arm64.tar.gz"
      sha256 "1983d63ad6a15bd3782649c81eecc94907f9e109fe252b55eec1ff5a02e62455"
    end
    on_intel do
      url "https://github.com/rjcuff/woolly/releases/download/v0.5.0/woolly_linux_amd64.tar.gz"
      sha256 "e2e473c17ef763d032cb6f9836444fc798cf952bfec0252d0a9f49dcba010260"
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
