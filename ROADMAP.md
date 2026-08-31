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

## What's coming

| Feature | Description | Status |
|---|---|---|
| **Keyboard dismiss** | Check visibility of on-screen keyboard and dismiss upon request | Planned |
| **Clear app storage** | Clear app cache and documents | Planned |
| **Simulate Shake** | Simulate a shake on the device | Planned |
| **Split npm release** | Smaller npm released based on plantform | Planned |
| **Windows ARM64** | First-class Windows ARM64 builds and support | Planned |
| **Device logs** | Read device system logs iOS `os_log`, Android `logcat` from the CLI and JSON-RPC. | Planned |

