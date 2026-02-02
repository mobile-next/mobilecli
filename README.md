# mobilecli

A universal command-line tool for managing iOS and Android devices, simulators, emulators and apps from [Mobile Next](https://github.com/mobile-next/).

<p align="center">
  <a href="https://github.com/mobile-next/mobilecli">
    <img src="https://img.shields.io/github/stars/mobile-next/mobilecli" alt="Mobile Next Stars" />
  </a>
  <a href="https://github.com/mobile-next/mobilecli">
    <img src="https://img.shields.io/github/contributors/mobile-next/mobilecli?color=green" alt="Mobile Next Downloads" />
  </a>
  <a href="https://www.npmjs.com/package/@mobilenext/mobilecli">
    <img src="https://img.shields.io/npm/dm/@mobilenext/mobilecli?logo=npm&style=flat&color=red" alt="npm">
  </a>
  <a href="https://github.com/mobile-next/mobilecli/releases">
    <img src="https://img.shields.io/github/release/mobile-next/mobilecli">
  </a>
  <a href="https://github.com/mobile-next/mobilecli/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-AGPL v3.0-blue.svg" alt="Mobile MCP is released under the AGPL v3.0 License">
  </a>
</p>

<p align="center">
  <a href="http://mobilenexthq.com/join-slack">
      <img src="https://img.shields.io/badge/join-Slack-blueviolet?logo=slack&style=flat" alt="Slack community channel" />
  </a>
</p>

## Features 🚀

- **Device Management**: List, manage, interactive with connected mobile devices
- **Cross-Platform Support**: Works with iOS physical devices, iOS simulators, Android devices, and Android emulators
- **Emulator/Simulator Control**: Boot and shutdown emulators and simulators programmatically
- **Screenshot Capture**: Take screenshots from any connected device with format options
- **Multiple Output Formats**: Save screenshots as PNG or JPEG with quality control
- **Screencapture video streaming**: Stream mjpeg/h264 video directly from device
- **Device Control**: Reboot devices, tap screen coordinates, press hardware buttons
- **App Management**: Launch, terminate, install, uninstall, list, and get foreground apps

### 🎯 Platform Support

| Platform | Supported |
|----------|:---------:|
| iOS Real Device | ✅ |
| iOS Simulator | ✅ |
| Android Real Device | ✅ |
| Android Emulator | ✅ |

## Installation 📦

#### Prerequisites 📋
- **Android SDK** with `adb` in PATH (for Android device support)
- **Xcode Command Line Tools** (for iOS simulator support on macOS)

#### Run instantly with npx
```bash
npx @mobilenext/mobilecli@latest
```

#### Install globally with npm
```bash
npm install -g @mobilenext/mobilecli@latest
```

#### Install from Source 🛠️
```bash
git clone https://github.com/mobile-next/mobilecli.git
cd mobilecli
make build
```

### Install Dependencies

#### 🍎 For iOS Simulator Support

Xcode is required. Make sure you have it installed with the runtimes relevant for you installed. You will have to create Simulators and have them booted before `mobilecli` can use them.

`mobilecli` will automatically install an agent on the device that is required for functions such as tapping on elements, pressing buttons and streaming screen capture.

#### 🤖 For Android Support
```bash
# Install Android SDK and ensure adb is in PATH
# Download from: https://developer.android.com/studio/command-line/adb
# or
brew install --cask android-platform-tools
```

## Usage

### List Connected Devices 🔍

```bash
# List all online devices and simulators
mobilecli devices

# List all devices including offline emulators and simulators
mobilecli devices --include-offline
```

Example output:
```json
[
  {
    "id": "12345678-1234567890ABCDEF",
    "name": "iPhone 15",
    "platform": "ios",
    "type": "real",
    "state": "online"
  },
  {
    "id": "Pixel_6",
    "name": "Pixel 6",
    "platform": "android",
    "type": "emulator",
    "state": "online"
  },
  {
    "id": "iPhone_13",
    "name": "iPhone 13",
    "platform": "ios",
    "type": "simulator",
    "state": "offline"
  }
]
```

**Note**: Offline emulators and simulators can be booted using the `mobilecli device boot` command.

### Take Screenshots 📸

```bash
# Take a PNG screenshot (default)
mobilecli screenshot --device <device-id>

# Take a JPEG screenshot with custom quality
mobilecli screenshot --device <device-id> --format jpeg --quality 80

# Save to specific path
mobilecli screenshot --device <device-id> --output screenshot.png

# Output to stdout
mobilecli screenshot --device <device-id> --output -
```

### Stream Screen 🎥

```bash
mobilecli screencapture --device <device-id> --format mjpeg | ffplay -
```

Note that screencapture is one way. You will have to use `io tap` commands to tap on the screen.

### Device Control 🎮

```bash
# Boot an offline emulator or simulator
mobilecli device boot --device <device-id>

# Shutdown a running emulator or simulator
mobilecli device shutdown --device <device-id>

# Reboot a device
mobilecli device reboot --device <device-id>

# Tap at coordinates (x,y)
mobilecli io tap --device <device-id> 100,200

# Long press at coordinates (x,y) with optional duration in milliseconds
mobilecli io longpress --device <device-id> 100,200
mobilecli io longpress --device <device-id> 100,200 --duration 2000

# Press hardware buttons
mobilecli io button --device <device-id> HOME
mobilecli io button --device <device-id> VOLUME_UP
mobilecli io button --device <device-id> POWER

# Send text
mobilecli io text --device <device-id> 'hello world'
```

### Supported Hardware Buttons

- `HOME` - Home button
- `BACK` - Back button (Android only)
- `POWER` - Power button
- `VOLUME_UP`, `VOLUME_DOWN` - Volume up and down
- `DPAD_UP`, `DPAD_DOWN`, `DPAD_LEFT`, `DPAD_RIGHT`, `DPAD_CENTER` - D-pad controls (Android only)

### App Management 📱

```bash
# List installed apps on device
mobilecli apps list --device <device-id>

# Get currently foreground app
mobilecli apps foreground --device <device-id>

# Launch an app
mobilecli apps launch <bundle-id> --device <device-id>

# Terminate an app
mobilecli apps terminate <bundle-id> --device <device-id>

# Install an app (.apk for Android, .ipa for iOS, .zip for iOS Simulator)
mobilecli apps install <path> --device <device-id>

# Uninstall an app
mobilecli apps uninstall <bundle-id> --device <device-id>
```

Example output for `apps foreground`:
```json
{
  "status": "ok",
  "data": {
    "packageName": "com.example.app",
    "appName": "Example App",
    "version": "1.0.0"
  }
}
```

## HTTP API 🔌

***mobilecli*** provides an http interface for all the functionality that is available through command line. As a matter of fact, it is preferable to
use mobilecli as a webserver, so it can cache and keep tunnels alive, speeding up your interactions with the mobile device or simulator/emulator.

```bash
# Start the server (default port 12000)
mobile server start

curl http://localhost:12000/rpc -XPOST -d '{"jsonrpc":"2.0", "id": 1, "method": "devices", "params": {}}'
curl http://localhost:12000/rpc -XPOST -d '{"jsonrpc":"2.0", "id": 1, "method": "screenshot", "params": {"deviceId": "your-device-id"}}'

## WebSocket Support 🔌

***mobilecli*** includes a WebSocket server that allows multiple requests over a single connection using the same JSON-RPC 2.0 format as the HTTP API.

```bash
# Start the server (default port 12000)
mobilecli server start

# Connect and send requests using wscat
wscat -c ws://localhost:12000/ws
> {"jsonrpc":"2.0","id":1,"method":"devices","params":{}}
< {"jsonrpc":"2.0","id":1,"result":[...]}
> {"jsonrpc":"2.0","id":2,"method":"screenshot","params":{"deviceId":"your-device-id"}}
< {"jsonrpc":"2.0","id":2,"result":{...}}
```

**Note**: `screencapture` is not supported over WebSocket - use the HTTP `/rpc` endpoint for video streaming.

## Platform-Specific Notes

### iOS Real Devices
- Currently requires that you install and run WebDriverAgent manually. You may change the BUNDLE IDENTIFIER, and *mobilecli* will be able to launch it if needed, as long as the identifier ends with `*.WebDriverAgent`.

## Development 👩‍💻

### Building 🛠️

Please refer to (docs/TESTING.md) for further instructions regarding testing *mobilecli* locally.

```bash
make lint
make build
make test
```

## Support 💬

For issues and feature requests, please use the [GitHub Issues](https://github.com/mobile-next/mobilecli/issues) page.

Be sure to <a href="http://mobilenexthq.com/join-slack">join our slack channel</a> today 💜

To learn more about <a href="https://mobilenexthq.com/">Mobile Next</a> and what we're building, <a href="https://mobilenexthq.com/#newsletter">subscribe to our newsletter</a>.

