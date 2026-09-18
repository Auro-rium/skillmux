class Skillmux < Formula
  desc "Local-first SKILL.md manager across agent harnesses"
  homepage "https://github.com/Auro-rium/skillmux"
  version "0.2.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.2.0/skillmux_darwin_arm64.tar.gz"
      sha256 "cbbe08c9184fd631efcb96b00ce2adc279f54233fc1c7da01961eb98e8475aab"
    else
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.2.0/skillmux_darwin_amd64.tar.gz"
      sha256 "502777f6ec9cf0b242514d0b7a17c77db32e707af7b6dc8cf146b3bef63c494b"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.2.0/skillmux_linux_arm64.tar.gz"
      sha256 "41537d50404ddcb7fe848eacdd51cc87aae3cdf27f5decea74a189cf64e767bd"
    else
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.2.0/skillmux_linux_amd64.tar.gz"
      sha256 "6cee96439d42f38e147c1e6b74b539c3361b9fabb542a4a75a13c99130a0c6e5"
    end
  end

  def install
    bin.install "skillmux"
  end

  test do
    assert_match "skillmux v#{version}", shell_output("#{bin}/skillmux version")
  end
end
