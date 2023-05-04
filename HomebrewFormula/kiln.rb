# typed: false
# frozen_string_literal: true

# This file was generated by GoReleaser. DO NOT EDIT.
class Kiln < Formula
  desc ""
  homepage ""
  version "0.83.0"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/pivotal-cf/kiln/releases/download/v0.83.0/kiln-darwin-arm64-0.83.0.tar.gz"
      sha256 "84ca47f6f878d582e11ebf97df61729248f4751140abc2abdc924ea63b074f85"

      def install
        bin.install "kiln"
      end
    end
    if Hardware::CPU.intel?
      url "https://github.com/pivotal-cf/kiln/releases/download/v0.83.0/kiln-darwin-amd64-0.83.0.tar.gz"
      sha256 "d15e2103ca33fa8e63041eaac2c2e77e99053079d92723bf869abcfd44ba34c9"

      def install
        bin.install "kiln"
      end
    end
  end

  on_linux do
    if Hardware::CPU.intel?
      url "https://github.com/pivotal-cf/kiln/releases/download/v0.83.0/kiln-linux-amd64-0.83.0.tar.gz"
      sha256 "4409bae9e3b15384cca187d5dffc6dd3e9585ee543c7ebd841c4e8d481363ce1"

      def install
        bin.install "kiln"
      end
    end
  end

  test do
    system "#{bin}/kiln --version"
  end
end
