#!/bin/bash
# CI bootstrap shared by the rootless, rootful and jailed package jobs.
set -euo pipefail
: "${THEOS:?Set THEOS to the destination for the pinned Theos checkout}"
brew install ldid dpkg
git clone https://github.com/theos/theos.git "$THEOS"
git -C "$THEOS" checkout 5280bd038207e14f8bd76f5417aa2fe641c03228
git -C "$THEOS" submodule update --init --recursive
sdk=$(xcrun --sdk iphoneos --show-sdk-path)
version=$(xcrun --sdk iphoneos --show-sdk-version)
mkdir -p "$THEOS/sdks"
ln -s "$sdk" "$THEOS/sdks/iPhoneOS${version}.sdk"
