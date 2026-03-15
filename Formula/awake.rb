class Awake < Formula
  desc "macOS CLI + TUI utility to keep your Mac awake"
  homepage "https://github.com/VolksRat71/awake"
  version "1.0.3"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.3/awake-v1.0.3-darwin-arm64.tar.gz"
      sha256 "8f058c9a171ba0cec9c0f57949ae1bd85f590d694354d9d9c8f4c8fb45ad4970"
    else
      url "https://github.com/VolksRat71/awake/releases/download/v1.0.3/awake-v1.0.3-darwin-amd64.tar.gz"
      sha256 "add26f643a583619f662934272de1f205082739e1f9b0337c964672d25833b43"
    end
  end

  depends_on :macos
  depends_on "terminal-notifier" => :recommended

  def install
    bin.install "awake"
  end

  def post_install
    system bin/"awake", "install"
  end

  def caveats
    <<~EOS
      The daemon and notifications were set up automatically.

      To start a session:
        awake 60            # 60 minutes
        awake until 17:00   # until 5 PM
        awake               # open the TUI
    EOS
  end

  test do
    assert_match "Keep your Mac awake", shell_output("#{bin}/awake --help")
  end
end
