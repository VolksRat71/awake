class Awake < Formula
  desc "macOS CLI + TUI utility to keep your Mac awake"
  homepage "https://github.com/VolksRat71/awake"
  version "1.0.5"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.5/awake-v1.0.5-darwin-arm64.tar.gz"
      sha256 "ee3dc0d1374207dca5c252ca957b877a6852672fe35bfd4afbfc5b32b3ac6ec9"
    else
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.5/awake-v1.0.5-darwin-amd64.tar.gz"
      sha256 "1ddf64caa047e29f71a759c386a00b617bb4f6986d25b7244d98847994fd6cfd"
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
