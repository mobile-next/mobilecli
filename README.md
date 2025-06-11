# mobilectl 

A universal command-line tool for managing iOS and Android devices, simulators, emulators and apps from [Mobile Next](https://github.com/mobile-next/). 

<h4 align="center">
<a href="https://github.com/mobile-next/mobilectl">
    <img src="https://img.shields.io/github/stars/mobile-next/mobilectl" alt="Mobile Next Stars" />
  </a>  
 <a href="https://github.com/mobile-next/mobilectl">
    <img src="https://img.shields.io/github/contributors/mobile-next/mobilectl?color=green" alt="Mobile Next Downloads" />
  </a>
  <a href="https://www.npmjs.com/package/@mobilenext/mobilectl">
    <img src="https://img.shields.io/npm/dm/@mobilenext/mobilectl?logo=npm&style=flat&color=red" alt="npm">
  </a>
<a href="https://github.com/mobile-next/mobilectl/releases">
    <img src="https://img.shields.io/github/release/mobile-next/mobilectl">
  </a>
<a href="https://github.com/mobile-next/mobilectl/blob/main/LICENSE">
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

### Prerequisites

- [**go-ios**](https://github.com/danielpaulus/go-ios) (for iOS device management)
- **Android SDK** with `adb` in PATH (for Android device support)
- **Xcode Command Line Tools** (for iOS simulator support on macOS)

### Install from Source

```bash
git clone https://github.com/mobile-next/mobilectl.git
cd mobilectl
make build
```

### Install Dependencies

#### 🍎 For iOS Support 
```bash
# Install go-ios for iOS device management
brew install go-ios
# or
npm install -g go-ios
```

#### 🤖 For Android Support
```bash
# Install Android SDK and ensure adb is in PATH
# Download from: https://developer.android.com/studio/command-line/adb
# or
brew install --cask android-platform-tools
```

## Usage

### List Connected Devices

```bash
# List all connected devices and simulators to your local or remote server
mobilectl devices
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

### Take Screenshots

```bash
# Take a PNG screenshot (default)
mobilectl screenshot --device <device-id>

# Take a JPEG screenshot with custom quality
mobilectl screenshot --device <device-id> --format jpeg --quality 80

# Save to specific path
mobilectl screenshot --device <device-id> --output screenshot.png

# Output to stdout
mobilectl screenshot --device <device-id> --output -
```

### Device Control

```bash
# Reboot a device
mobilectl reboot --device <device-id>

# Tap at coordinates (x,y)
mobilectl tap --device <device-id> 100,200

# Press hardware buttons
mobilectl press-button --device <device-id> HOME
mobilectl press-button --device <device-id> VOLUME_UP
mobilectl press-button --device <device-id> POWER
```

### Supported Hardware Buttons

- `HOME` - Home button
- `BACK` - Back button (Android only)
- `POWER` - Power button
- `VOLUME_UP` - Volume up
- `VOLUME_DOWN` - Volume down

## Platform-Specific Notes

### iOS Real Devices
- Currently requires that you install and run WebDriverAgent manually

### iOS Simulators  
- Currently requires that you install and run WebDriverAgent manually

## Development

### Building

```bash
make build
```

### Testing

```bash
make test
```

### Linting

```bash
make lint
```

### Project Structure

```
mobilectl/
├── main.go              # CLI entry point and commands
├── devices/             # Device management interfaces
│   ├── common.go        # ControllableDevice interface
│   ├── android.go       # Android device implementation
│   ├── ios.go          # iOS real device implementation
│   ├── simulator.go    # iOS simulator implementation
│   └── wda.go          # WebDriverAgent client
└── utils/              # Utility functions
    ├── image.go        # Image conversion utilities
    ├── file.go         # File operations
    └── zipfile.go      # Archive operations
```

## Roadmap

- [ ] Webserver with json-rpc interface
- [ ] Automatically install WebDriverAgent 
- [ ] Automatically create tunnel for iOS17+
- [ ] App installation/management commands (install, removed, update)
- [ ] Video streaming capabilities with WebRTC
- [ ] Remote device management server
- [ ] CI/CD pipeline improvements
- [ ] Package distribution (Homebrew, etc.)

## Support

For issues and feature requests, please use the [GitHub Issues](https://github.com/mobile-next/mobilectl/issues) page. 
