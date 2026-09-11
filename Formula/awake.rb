class Awake < Formula
  desc "macOS CLI + TUI utility to keep your Mac awake"
  homepage "https://github.com/VolksRat71/awake"
  version "1.0.6"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.6/awake-v1.0.6-darwin-arm64.tar.gz"
      sha256 "09a586c2a846e7086cda59e1e25a5b358b2d78ffa27c77aefdd20e767b531a93"
    else
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.6/awake-v1.0.6-darwin-amd64.tar.gz"
      sha256 "38f51b7a980eb715ef2052f8f944c31deae6f55bf886decb3e8bed0fd10d2265"
    end
  end

  depends_on :macos
  depends_on "terminal-notifier" => :recommended

  def install
    bin.install "awake"
  end

  def caveats
    <<~EOS
      Run the setup to configure the daemon and notifications:
        awake install

      Then start a session:
        awake 60            # 60 minutes
        awake until 17:00   # until 5 PM
        awake               # open the TUI
    EOS
  end

  test do
    assert_match "Keep your Mac awake", shell_output("#{bin}/awake --help")
  end
end
