# DeviceKit iOS

[![Swift](https://img.shields.io/badge/Swift-5.9-orange.svg?style=flat-square)](https://swift.org)
[![Platform](https://img.shields.io/badge/Platform-iOS%2016.0+-blue.svg?style=flat-square)](https://developer.apple.com/ios/)
[![License](https://img.shields.io/badge/License-FSL--1.1--Apache--2.0-lightgrey.svg?style=flat-square)](LICENSE)

Control any iOS device or simulator over a simple JSON-RPC API. Tap, swipe, stream video, inspect the UI, from any language, over localhost.

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Usage](#usage)
  - [Starting the Server](#starting-the-server)
  - [JSON-RPC API](#json-rpc-api)
  - [Streaming Endpoints](#streaming-endpoints)
- [Architecture](#architecture)
- [Building](#building)
- [Testing](#testing)
- [Communication](#communication)
- [Acknowledgements](#acknowledgements)
- [License](#license)

## What You Can Build

- **Interactive automation** — Tap, swipe, long-press, type text, press hardware buttons (home, lock, volume)
- **App control** — Launch, terminate, and detect the foreground app by bundle ID
- **Live screen visibility** — MJPEG and H264 streaming at configurable FPS, bitrate, and quality
- **UI inspection** — Full accessibility tree dumps for element targeting
- **Screenshots** — PNG or JPEG capture with configurable quality
- **System control** — Get/set orientation, open URLs, query screen size and scale
- **Broadcast audio/video** — ReplayKit extension with H264 video and Opus audio over TCP
- **Flexible transport** — JSON-RPC 2.0 over WebSocket or HTTP, from any language

## Requirements

| Platform | Minimum Version |
|----------|----------------|
| iOS      | 14.0           |
| Swift    | 5.9            |
| Xcode    | 15.0+          |

## Installation

### Building from Source

```bash
# Clone the repository
git clone https://github.com/mobile-next/devicekit-ios.git
cd devicekit-ios

# Install dependencies
brew install xcbeautify

# Build unsigned IPA for real devices
make ipa-unsigned

# Build XCUITest runner for simulators
make sim-zip
```

### Build Targets

| Target | Output | Description |
|--------|--------|-------------|
| `make ipa-unsigned` | `build/export/devicekit-ios-unsigned.ipa` | Unsigned IPA for arm64 devices |
| `make sim-zip-arm64` | `build/export/devicekit-ios-Sim-arm64.zip` | Simulator runner (Apple Silicon) |
| `make sim-zip-x86_64` | `build/export/devicekit-ios-Sim-x86_64.zip` | Simulator runner (Intel) |
| `make sim-zip` | Both simulator zips | arm64 + x86_64 |
| `make lint` | — | Run SwiftLint |
| `make clean` | — | Remove build artifacts |

### Building with Newer Xcode

The project targets iOS 15.0, the lowest Xcode 27 accepts, so local builds work out of the box. Release builds still support iOS 14: the build workflow sets `IPHONEOS_DEPLOYMENT_TARGET=14.0`, and the `ipa-unsigned` and `sim-zip*` targets pass it to `xcodebuild`. To reproduce a release build with an Xcode that still accepts 14.0:

```bash
make sim-zip IPHONEOS_DEPLOYMENT_TARGET=14.0
```

## Quick Start

Once the server is running at `127.0.0.1:12004`, make your first call:

```bash
curl -X POST http://127.0.0.1:12004/rpc \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"device.screenshot","params":{ "format": "png" },"id":1}'
```

Returns a base64-encoded PNG of the current screen.

## Usage

### Starting the Server

DeviceKit runs as an XCUITest. Once installed and launched on a device or simulator, it starts a server on `127.0.0.1:12004`.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `DEVICEKIT_LISTEN_PORT` | `12004` | JSON-RPC server port |
| `DEVICEKIT_LISTEN_HOST` | `127.0.0.1` | Bind address for the JSON-RPC server: a comma-separated list of IPv4 or IPv6 literals, one listener each. All TCP servers (video, audio) also bind to `127.0.0.1` by default. |

#### Reaching a real device over Wi-Fi

Xcode keeps a private IPv6 tunnel to each paired device, reachable only from the paired Mac and kept when the device switches from USB to Wi-Fi. Add the device side of that tunnel to the listen list to reach DeviceKit without USB port forwarding, while keeping the loopback listener for existing clients. `xcodebuild` passes `TEST_RUNNER_`-prefixed variables into the test runner:

```bash
TUNNEL=$(xcrun devicectl device info details --device <udid> | awk '/Tunnel IP Address|tunnelIPAddress/{print $NF}')
# Keeps running until the server is stopped with POST /shutdown
TEST_RUNNER_DEVICEKIT_LISTEN_HOST="127.0.0.1,$TUNNEL" xcodebuild test-without-building \
  -project devicekit-ios.xcodeproj -scheme devicekit-ios -destination "id=<udid>" \
  -collect-test-diagnostics never
```

Then, from a second terminal while it runs:

```bash
TUNNEL=$(xcrun devicectl device info details --device <udid> | awk '/Tunnel IP Address|tunnelIPAddress/{print $NF}')
curl -g -X POST "http://[$TUNNEL]:12004/rpc" -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","method":"device.info","params":{},"id":1}'
```

The tunnel address can change when the tunnel reconnects (for example during a long build), and a stale address fails to bind. Read it right before starting the runner, and if the tunnel reconnects while the runner is up, stop the runner (`POST /shutdown`) and start it again with the new address, since it does not rebind on its own.

Avoid binding `0.0.0.0`, `::` or the device's Wi-Fi address instead: the server has no authentication, so that exposes device control to the whole network.

If the server can't start (for example, an address that can't be bound), the test fails. `-collect-test-diagnostics never` stops `xcodebuild` from collecting a sysdiagnose on that failure, which otherwise can stall it for minutes.

**Endpoints:**

| Endpoint | Protocol | Description |
|----------|----------|-------------|
| `GET /ws` | WebSocket | JSON-RPC 2.0 |
| `POST /rpc` | HTTP | JSON-RPC 2.0 |
| `GET /health` | HTTP | Health check |
| `GET /mjpeg` | HTTP | MJPEG screen stream |
| `GET /h264` | HTTP | H264 screen stream |

### JSON-RPC API

All methods follow the [JSON-RPC 2.0](https://www.jsonrpc.org/specification) specification.

```json
{
  "jsonrpc": "2.0",
  "method": "device.io.tap",
  "params": { "x": 100, "y": 200, "deviceId": "any" },
  "id": 1
}
```

#### Input

| Method | Description |
|--------|-------------|
| `device.io.tap` | Tap at (x, y) coordinates |
| `device.io.swipe` | Swipe from (x1, y1) to (x2, y2) with an optional duration in seconds (default 0.1, max 60) |
| `device.io.longpress` | Long press at (x, y) for a duration |
| `device.io.gesture` | Multi-finger gesture with press/move/release actions |
| `device.io.text` | Type text into the focused field |
| `device.io.button` | Press a hardware button (`home`, `lock`, `volumeUp`, `volumeDown`) |

#### Device

| Method | Description |
|--------|-------------|
| `device.info` | Get screen size, laid out in the foreground app's orientation, and scale factor |
| `device.io.orientation.get` | Get the orientation the foreground app is drawn in (`PORTRAIT` / `LANDSCAPE`) |
| `device.io.orientation.set` | Set orientation to `PORTRAIT` or `LANDSCAPE` |
| `device.url` | Open a URL |
| `device.clipboard.get` | Read the clipboard text |
| `device.clipboard.set` | Replace the clipboard text |

#### Apps

| Method | Description |
|--------|-------------|
| `device.apps.launch` | Launch an app by bundle ID |
| `device.apps.terminate` | Terminate an app by bundle ID |
| `device.apps.foreground` | Get the foreground app's bundle ID, name, and PID |

#### Inspection

| Method | Description |
|--------|-------------|
| `device.dump.ui` | Return the full accessibility view hierarchy |
| `device.screenshot` | Capture a screenshot (base64 PNG/JPEG) |

### Streaming Endpoints

#### MJPEG

```
GET /mjpeg?fps=10&quality=25&scale=100
```

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `fps` | 10 | 1–60 | Frames per second |
| `quality` | 25 | 1–100 | JPEG quality (%) |
| `scale` | 100 | 10–100 | Scale factor (%) |

#### H264

```
GET /h264?fps=30&bitrate=4000000&quality=60&scale=50
```

| Parameter | Default | Range | Description |
|-----------|---------|-------|-------------|
| `fps` | 30 | 1–60 | Frames per second |
| `bitrate` | 4000000 | 100000–10000000 | Target bitrate (bps) |
| `quality` | 60 | 1–100 | Encoder quality (%) |
| `scale` | 50 | 10–100 | Scale factor (%) |

### Broadcast Extension

The ReplayKit broadcast extension provides system-level screen and audio capture over raw TCP, independent of the JSON-RPC server. These are **not** HTTP endpoints — connect with `nc` or any raw TCP client.

| Port  | Stream      | Format |
|-------|-------------|--------|
| 12005 | H264 video  | Raw NAL units |
| 12006 | Opus audio  | Length-prefixed frames (4-byte big-endian uint32) |

```bash
ios tunnel start --userspace &
ios forward 12005 12005 &

nc localhost 12005 | ffplay \
  -fflags nobuffer \
  -flags low_delay \
  -probesize 32 \
  -analyzeduration 0 \
  -framedrop \
  -sync ext \
  -f h264 -framerate 60 -
```

`-framerate` matters: a raw H264 stream carries no timestamps ffplay can read, so
ffplay assumes 25 fps. The broadcast sends up to 60 fps, and without the flag
playback falls further behind every second. Any value at or above the real rate
works, since ffplay simply waits for the next frame.

## Testing

Tests run against a live XCUITest server on a booted simulator. You need one simulator running — the test harness picks the first booted device automatically.

```bash
# Install test dependencies (one-time)
cd tests && npm install && cd ..

# Run tests with code coverage
make test-coverage

# View coverage report as HTML
make coverage-html
```

## Architecture

```
devicekit-ios/
  DeviceKit/                    # Host app (SwiftUI, triggers broadcast picker)
  DeviceKitTests/               # XCUITest runner (automation server)
    JSONRPC/                    #   JSON-RPC protocol + 15 method handlers
    Streamer/                   #   MJPEG and H264 HTTP streaming
    XCTest/                     #   Private API wrappers (touch synthesis, accessibility)
    H264Stream/                 #   Screenshot-based H264 streaming
  BroadcastUploadExtension/     # ReplayKit extension (H264 + Opus over TCP)
  h264-codec/                   # Swift package: H264 encoder (VideoToolbox)
  opus-codec/                   # Swift package: Opus encoder (wraps libopus)
```

## Dependencies

- [FlyingFox](https://github.com/swhitty/FlyingFox) — Lightweight HTTP and WebSocket server
- [libopus](https://opus-codec.org/) — Audio codec (vendored as C source in `opus-codec/`)

## Communication

Questions, issues, or ideas? [Open an issue](https://github.com/mobile-next/devicekit-ios/issues) — all feedback is welcome.

Want to contribute? PRs are appreciated. Check open issues for good first tasks.

## Acknowledgements

DeviceKit iOS began as a fork of the iOS XCTest runner in
[Maestro](https://github.com/mobile-dev-inc/maestro) and has been substantially
rewritten and extended since. Portions of that code remain in this repository,
including parts that Maestro derived from
[WebDriverAgent](https://github.com/facebookarchive/WebDriverAgent) by Facebook,
Inc., later maintained by [Appium](https://github.com/appium/WebDriverAgent).

We are grateful to those projects and their contributors. DeviceKit iOS is not
affiliated with, or endorsed by, any of them.

## License

Code written for DeviceKit iOS is released under the
[Functional Source License 1.1, Apache 2.0 Future License](LICENSE).

Portions derived from third-party projects remain under their original licenses.
See [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md).
