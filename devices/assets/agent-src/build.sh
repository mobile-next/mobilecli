#!/bin/bash
# Builds the on-device automation agent (classes.dex) embedded by
# devices/android_agent.go as mw-agent.dex.
#
# Requirements: a JDK (javac), Android SDK with an android.jar platform and
# build-tools (d8). Override SDK locations with ANDROID_HOME / ANDROID_PLATFORM
# / ANDROID_BUILD_TOOLS if autodetection fails.
set -e
HERE=$(cd "$(dirname "$0")" && pwd)
SDK=${ANDROID_HOME:-$HOME/Library/Android/sdk}
PLATFORM=${ANDROID_PLATFORM:-$SDK/platforms/android-33/android.jar}
BUILD_TOOLS=${ANDROID_BUILD_TOOLS:-$(ls -d "$SDK"/build-tools/* | sort -V | tail -1)}

echo "SDK=$SDK"
echo "PLATFORM=$PLATFORM"
echo "BUILD_TOOLS=$BUILD_TOOLS"

rm -rf "$HERE/out"
mkdir -p "$HERE/out/classes"

echo "==> javac"
javac -source 11 -target 11 -cp "$PLATFORM" -d "$HERE/out/classes" \
  "$HERE/src/dev/mobilewright/agent/Agent.java"

echo "==> d8"
"$BUILD_TOOLS/d8" --min-api 24 --output "$HERE/out" --lib "$PLATFORM" \
  $(find "$HERE/out/classes" -name '*.class')

cp "$HERE/out/classes.dex" "$HERE/../mw-agent.dex"
echo "==> wrote devices/assets/mw-agent.dex"
