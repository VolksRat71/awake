class Awake < Formula
  desc "macOS CLI + TUI utility to keep your Mac awake"
  homepage "https://github.com/VolksRat71/awake"
  version "1.0.4"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.4/awake-v1.0.4-darwin-arm64.tar.gz"
      sha256 "957b6c9dd60cef8f9270b17d33e9a8a57cb7f6c6c21463b4bcc713c4cece5c4c"
    else
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.4/awake-v1.0.4-darwin-amd64.tar.gz"
      sha256 "db6bc81341475c44611ef1d66393602206337dc1eb59eaa98116d528c798a3e5"
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
