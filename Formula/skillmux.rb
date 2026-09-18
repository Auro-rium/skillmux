class Skillmux < Formula
  desc "Local-first SKILL.md manager across agent harnesses"
  homepage "https://github.com/Auro-rium/skillmux"
  version "0.1.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.1.0/skillmux_darwin_arm64.tar.gz"
      sha256 "c777ac6a43e8822771b01032271ebc03bbf7df2ef886834c6ad354572aee62a9"
    else
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.1.0/skillmux_darwin_amd64.tar.gz"
      sha256 "8ca1dff9ebb7898a574ccc0fcfe059489ebcf471a7b33154b67b592aa182ba11"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.1.0/skillmux_linux_arm64.tar.gz"
      sha256 "069b2a898066078fcd5b043a98bba4c430854a6581bd1f1b9056541d4fef0f1b"
    else
      url "https://github.com/Auro-rium/skillmux/releases/download/v0.1.0/skillmux_linux_amd64.tar.gz"
      sha256 "d210b28ba264252d855192fed2573d18f4cc1b3ee8c37cdf57a775c649612c06"
    end
  end

  def install
    bin.install "skillmux"
  end

  test do
    assert_match "skillmux v#0.1.0", shell_output("#<built-in function bin>/skillmux version")
  end
end
