# typed: strict
# frozen_string_literal: true

# Homebrew formula for Navia.
class Navia < Formula
  desc "Terminal micro-IDE for project navigation, editing, search, and git review"
  homepage "https://github.com/heidaraliy/navia"
  version "0.1.3"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.3/navia_0.1.3_darwin_arm64.tar.gz"
      sha256 "9031a7de80d25fb7ce8dff2778e43d8741386a04916fb05a5afdfad10d42c9b8"
    else
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.3/navia_0.1.3_darwin_amd64.tar.gz"
      sha256 "5c5bdf66e8f0324fdd790b067c6188544aa0ec5af0d0376267fa5a31d2be642a"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.3/navia_0.1.3_linux_arm64.tar.gz"
      sha256 "ca82fbfa531b53ba38cad6e9e639d6d077b78e3a01abc5a0a77b3b171547d5d4"
    else
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.3/navia_0.1.3_linux_amd64.tar.gz"
      sha256 "1306ceb74535451c2b69a230d6e23383599b1be6c7c8d6387b15eb1d15088f1c"
    end
  end

  def install
    bin.install "navia"
  end

  test do
    assert_match "navia 0.1.3", shell_output("#{bin}/navia --version")
  end
end
