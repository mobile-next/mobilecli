# Roadmap

This is a living document of planned and in-progress features. Items are roughly prioritized top-to-bottom. Have a feature request? [Open an issue](https://github.com/mobile-next/mobilecli/issues/new/choose).

`mobilecli` is the single device layer underneath the whole Mobile Next stack. Everything above it, Mobilewright for tests, Mobile MCP for agents, your own scripts and CI, talks to devices through this one API. So the bar here is stability first: the surface other tools depend on should not move under them.

## What's included today

| Feature | Example |
|---|---|
| ✅ Device discovery | `mobilecli devices`, `--include-offline` |
| ✅ iOS + Android, real + virtual | iOS real device, iOS Simulator, Android real device, Android emulator |
| ✅ Boot & shutdown | `mobilecli device boot`, `mobilecli device shutdown`, `mobilecli device reboot` |
| ✅ Screenshots | `mobilecli screenshot --device <id>` (PNG/JPEG, quality control) |
| ✅ Screen streaming | `mobilecli screencapture --device <id>` (MJPEG / H.264) |
| ✅ Input | `mobilecli io tap`, `io longpress`, `io button`, `io text` |
| ✅ App lifecycle | `mobilecli apps list`, `launch`, `terminate`, `install`, `uninstall`, `foreground`, `path` |
| ✅ Filesystem | `mobilecli fs ls`, `push`, `pull`, `mkdir`, `rm` (Android, iOS Simulator, app containers) |
| ✅ Crash reports | `mobilecli device crashes` (iOS + Android) |
| ✅ WebView inspection | `mobilecli webview list`, `query`, `goto`, `eval`, `content`, `back`, `forward` |
| ✅ Server mode | `mobilecli server start`, JSON-RPC over HTTP + WebSocket |
| ✅ Remote device allocation | `mobilecli remote allocate`, `mobilecli auth login` |
| ✅ Device logs | `mobilecli device logs` (iOS `os_log`, Android `logcat`) |
| ✅ Split npm release | Per-platform npm packages, smaller install |
| ✅ Clipboard | `mobilecli io clipboard get`, `io clipboard set` |
| ✅ GPS location override | `mobilecli device location set <lat,lon>`, `device location clear` |
| ✅ Screenshot scaling & clipping | `mobilecli screenshot --scale`, `--max-size`, `--clip x,y,w,h` |
| ✅ Flutter UI dump | `mobilecli dump ui` reads Flutter widget trees via the Dart VM service |
| ✅ Element references | `mobilecli dump ui` assigns `@ref` ids, `io tap @ref` acts on elements without coordinates |
| ✅ Windows ARM64 | Native `windows-arm64` builds published with every release |

## What's coming

| Feature | Description | Status |
|---|---|---|
| **Keyboard dismiss** | Check visibility of on-screen keyboard and dismiss upon request | Planned |
| **Clear app storage** | Clear app cache and documents | Planned |
| **Simulate Shake** | Simulate a shake on the device | Planned |
| **Push notifications** | Deliver a notification payload to an app without a real APNs/FCM round-trip | Planned |
| **Biometrics** | Simulate Face ID, Touch ID and fingerprint match/non-match (virtual devices only) | Planned |
| **Image injection** | Add images and videos to the device photo library | Planned |
| **Dark mode** | Switch the device between light and dark appearance | Planned |
| **Wifi & airplane mode** | Toggle wifi and airplane mode to test offline behaviour | Planned |
| **Network capture** | Record HTTP(S) traffic from an app for inspection | Planned |
| **Network interception** | Stub, rewrite or fail matching requests to drive app behaviour | Planned |
| **App profiling** | Sample CPU, memory and frame timing of a running app | Planned |
| **Timezone** | Override the device timezone | Planned |
| **Apple Pay / Google Pay** | Simulate a payment sheet approval or decline | Planned |
| **Camera injection** | Feed an image or video into the device camera | Planned |
