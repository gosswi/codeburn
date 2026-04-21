class Codeburn < Formula
  desc "CLI tool showing where AI coding tokens go - by task, tool, model, and project"
  homepage "https://github.com/agentseal/codeburn"
  version "0.5.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/agentseal/codeburn/releases/download/v#{version}/codeburn_#{version}_darwin_arm64.tar.gz"
      sha256 "PLACEHOLDER_ARM64_SHA256"
    else
      url "https://github.com/agentseal/codeburn/releases/download/v#{version}/codeburn_#{version}_darwin_amd64.tar.gz"
      sha256 "PLACEHOLDER_AMD64_SHA256"
    end
  end

  def install
    bin.install "codeburn"
  end

  test do
    system "#{bin}/codeburn", "--version"
  end
end
