class Navia < Formula
  desc "Terminal micro-IDE for project navigation, editing, search, and git review"
  homepage "https://github.com/heidaraliy/navia"
  version "0.1.0"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.0/navia_0.1.0_darwin_arm64.tar.gz"
      sha256 "0cc6b735771ab0a3e0cc0cd0d9d551dbc161480c3699ef7f336d9c87638eba77"
    else
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.0/navia_0.1.0_darwin_amd64.tar.gz"
      sha256 "179c08eb02d2cf4dd4fc7bc38c340102ed7c409ad02536909cc477e4457673e3"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.0/navia_0.1.0_linux_arm64.tar.gz"
      sha256 "557e1b626794c9b232ac0e7fb58cd1a971ad1bccb73ed65349708a5921fe1c7b"
    else
      url "https://github.com/heidaraliy/navia/releases/download/v0.1.0/navia_0.1.0_linux_amd64.tar.gz"
      sha256 "8977e6c10fdeb02524e113f82d4cde2a269052ffc03c92b9fe87f34a18665365"
    end
  end

  def install
    bin.install "navia"
  end

  test do
    assert_match "navia 0.1.0", shell_output("#{bin}/navia --version")
  end
end
