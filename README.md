# mobilecli 

A universal command-line tool for managing iOS and Android devices, simulators, emulators and apps from [Mobile Next](https://github.com/mobile-next/). 

<h4 align="center">
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

<h4 align="center">
<a href="http://mobilenexthq.com/join-slack">
    <img src="https://img.shields.io/badge/join-Slack-blueviolet?logo=slack&style=flat" alt="Slack community channel" />
</a>	
</p>


## Features 🚀

- **Device Management**: List and manage connected iOS/Android devices and simulators
- **Screenshot Capture**: Take screenshots from any connected device with format options (PNG/JPEG)
- **Device Control**: Reboot devices, tap screen coordinates, press hardware buttons
- **Cross-Platform Support**: Works with iOS physical devices, iOS simulators, Android devices, and Android emulators
- **Multiple Output Formats**: Save screenshots as PNG or JPEG with quality control
- **App management**: Launch app, terminate apps. Install and uninstall coming next ⏭️

## Installation 🪄

### Prerequisites 📋

- **Android SDK** with `adb` in PATH (for Android device support)
- **Xcode Command Line Tools** (for iOS simulator support on macOS)

### Install ⚡

Get started right now with `mobilecli`. 

```bash
npm install -g @mobilenext/mobilecli@latest
```

### Install from Source 🛠️

```bash
git clone https://github.com/mobile-next/mobilecli.git
cd mobilecli
make build
```

### Install Dependencies

#### 🍎 For iOS Simulator Support 

Xcode is required. Make sure you have it installed with the runtimes relevant for you installed. You will have to create Simulators and have them booted before `mobilecli` can use them.

`mobilecli` will automatically install an agent on the device that is required for functionalities such as opening a url, tapping on buttons and streaming screen capture.

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
# List all connected devices and simulators to your local or remote server
mobilecli devices
```

Example output:
```json
[
  {
    "id": "12345678-1234567890ABCDEF",
    "name": "iPhone 15",
    "platform": "ios",
    "type": "real"
  },
  {
    "id": "emulator-5554",
    "name": "Pixel_7_API_34",
    "platform": "android", 
    "type": "emulator"
  }
]
```

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
# Reboot a device
mobilecli device reboot --device <device-id>

# Tap at coordinates (x,y)
mobilecli io tap --device <device-id> 100,200

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

