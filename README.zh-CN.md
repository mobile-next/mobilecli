# mobilecli

[English](README.md) | [日本語](README.ja.md) | **简体中文**

由 [Mobile Next](https://github.com/mobile-next/) 出品的通用命令行工具，用于管理 iOS 和 Android 设备、模拟器、仿真器以及应用。

<p align="left">
  <a href="https://github.com/mobile-next/mobilecli">
    <img src="https://img.shields.io/github/stars/mobile-next/mobilecli" alt="Mobile Next Stars" />
  </a>
  <a href="https://www.npmjs.com/package/mobilecli">
    <img src="https://img.shields.io/npm/dm/mobilecli?logo=npm&style=flat&color=red" alt="npm">
  </a>
  <a href="https://github.com/mobile-next/mobilecli/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-FSL--1.1--Apache--2.0-blue.svg" alt="mobilecli is released under the FSL-1.1-Apache-2.0 License">
  </a>
  <a href="http://mobilenext.ai/join-slack">
      <img src="https://img.shields.io/badge/join-Slack-blueviolet?logo=slack&style=flat" alt="Slack community channel" />
  </a>
</p>

## 功能特性 🚀

- **设备管理**：列出、管理并操作已连接的移动设备
- **跨平台支持**：支持 iOS 真机、iOS 模拟器、Android 真机和 Android 仿真器
- **仿真器 / 模拟器控制**：通过编程方式启动和关闭仿真器与模拟器
- **截图**：从任意已连接设备截图，并可选择输出格式
- **多种输出格式**：以 PNG 或 JPEG 保存截图，并可控制画质
- **屏幕录制视频流**：直接从设备推送 mjpeg / h264 视频流
- **设备控制**：重启设备、点击屏幕坐标、按下硬件按键
- **应用管理**：启动、终止、安装、卸载、清除数据、列出应用以及获取前台应用
- **文件系统**：在设备上或应用容器内 push、pull、列出文件、mkdir 和 rm（Android、iOS 模拟器）
- **位置伪装**：伪造设备上报的 GPS 位置
- **崩溃报告**：列出并获取 iOS 和 Android 设备上的崩溃报告
- **设备日志**：实时流式输出 iOS 和 Android 设备日志，并支持过滤
- **WebView 检查**：列出内嵌 WebView、页面跳转、查询 DOM 以及执行 JavaScript

### 🎯 平台支持

| 平台 | 支持 |
|----------|:---------:|
| iOS 真机 | ✅ |
| iOS 模拟器 | ✅ |
| Android 真机 | ✅ |
| Android 仿真器 | ✅ |

## 安装 📦

#### 前置条件 📋
- **Android SDK**，且 `adb` 已加入 PATH（用于支持 Android 设备）
- **Xcode Command Line Tools**（用于在 macOS 上支持 iOS 模拟器）

#### 使用 npm 全局安装
```bash
npm install -g mobilecli@latest
```

### Agent 设置 🤖

安装 `mobilecli` 并添加 skill，让你的编码 agent 知道如何使用它：

```bash
npm install -g mobilecli@latest
npx skills add https://github.com/mobile-next/mobilecli
```

## CLI 参考

### 列出已连接的设备 🔍

```bash
# List all online devices and simulators
mobilecli devices

# List all devices including offline emulators and simulators
mobilecli devices --include-offline
```

**注意**：离线的仿真器和模拟器可以通过 `mobilecli device boot` 命令启动。

### 截图 📸

```bash
# Take a PNG screenshot (default)
mobilecli screenshot --device <device-id>

# Take a JPEG screenshot with custom quality
mobilecli screenshot --device <device-id> --format jpeg --quality 80

# Scale the screenshot down to half size
mobilecli screenshot --device <device-id> --scale 0.5

# Limit the largest dimension to 800 pixels, keeping aspect ratio
mobilecli screenshot --device <device-id> --max-size 800

# Crop to a region (x,y,width,height in screen points), applied before scaling
mobilecli screenshot --device <device-id> --clip 10,80,300,200

# Save to specific path
mobilecli screenshot --device <device-id> --output screenshot.png

# Output to stdout
mobilecli screenshot --device <device-id> --output -
```

### 屏幕推流 🎥

```bash
mobilecli screencapture --device <device-id> --format mjpeg | ffplay -
```

注意，screencapture 是单向的。要点击屏幕，需要使用 `io tap` 命令。

### 设备控制 🎮

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

# Read device clipboard
mobilecli io clipboard get --device <device-id>

# Write device clipboard
mobilecli io clipboard set --device <device-id> 'hello world'

# Send keys combination to paste (cmd+v for iOS, ctrl+v for Android)
mobilecli io keys --device <device-id> "cmd+v"
```

### 位置伪装 📍

```bash
# Fake the device location (latitude,longitude)
mobilecli device location set --device <device-id> 37.7749,-122.4194

# Hold the override until Ctrl-C, then clear it automatically
mobilecli device location set --device <device-id> 37.7749,-122.4194 --wait

# Restore the real location
mobilecli device location clear --device <device-id>
```

各平台注意事项：

| 平台 | 说明 |
|----------|-------|
| iOS 模拟器 | 开箱即用，mobilecli 退出后伪装位置依然保留 |
| iOS 真机 | 伪装位置只在设置它的那个 mobilecli 进程存活期间有效，因此必须使用 `--wait`。`clear` 也必须由同一个进程执行 |
| Android 仿真器 | 使用仿真器控制台。它没有撤销定位的方法，因此 `clear` 会把位置重置为仿真器启动时的坐标（Googleplex），而不是真实位置 |
| Android 真机 | 在设备上运行一个代理作为模拟位置提供者，并向 `com.android.shell` 授予 `mock_location` appop。部分 OEM ROM 会忽略模拟位置提供者，检查 `Location.isFromMockProvider()` 或 Play Integrity 的应用能够识别出来 |

### 支持的硬件按键

- `HOME` - Home 键
- `BACK` - 返回键（仅 Android）
- `POWER` - 电源键
- `VOLUME_UP`, `VOLUME_DOWN` - 音量加 / 减
- `DPAD_UP`, `DPAD_DOWN`, `DPAD_LEFT`, `DPAD_RIGHT`, `DPAD_CENTER` - 方向键控制（仅 Android）

### 应用管理 📱

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

# Clear app data (cache, preferences, databases) without uninstalling
# Supported on Android and iOS Simulator
mobilecli apps clear <bundle-id> --device <device-id>
```

### 文件系统 📂

访问设备上或应用数据容器内的文件。目前支持 **Android** 和 **iOS 模拟器**。

```bash
# Get the data container path of an app (Android)
mobilecli apps path <bundle-id> --device <device-id>

# List files at any absolute path (defaults to device root if omitted)
mobilecli fs ls --device <device-id>
mobilecli fs ls --device <device-id> /sdcard
mobilecli fs ls --device <device-id> /sdcard/Download

# List files inside an app's data container
mobilecli fs ls --device <device-id> com.example.app
mobilecli fs ls --device <device-id> com.example.app /Documents

# Pull a file from the device to local disk
mobilecli fs pull --device <device-id> /sdcard/recording.mp4 ./recording.mp4

# Pull a file from an app's private container
mobilecli fs pull --device <device-id> /data/user/0/com.example.app/files/db.sqlite ./db.sqlite

# Push a file to the device
mobilecli fs push --device <device-id> ./config.json /sdcard/config.json

# Push a file into an app's private container
mobilecli fs push --device <device-id> ./config.json /data/user/0/com.example.app/files/config.json

# Create a directory
mobilecli fs mkdir --device <device-id> /sdcard/myfolder

# Create a directory and all parent directories
mobilecli fs mkdir --device <device-id> -p /sdcard/a/b/c
mobilecli fs mkdir --device <device-id> -p /data/user/0/com.example.app/files/cache/v2

# Remove a file
mobilecli fs rm --device <device-id> /sdcard/old_file.txt

# Remove a directory recursively
mobilecli fs rm --device <device-id> -r /sdcard/myfolder
mobilecli fs rm --device <device-id> -r /data/user/0/com.example.app/files/cache
```

**注意：**
- `/data/user/` 下的路径通过 `run-as` 访问，因此应用必须是可调试（debuggable）的。
- 向 `/data/user/` push 文件时，会先把文件暂存到 `/data/local/tmp/`，再复制进容器。
- 在所有平台上都完整支持 pull 二进制文件（图片、数据库、DEX 文件），并保证二进制安全。

### 代理管理 🤖

在 **iOS** 上，触控输入（点击、滑动、按键）、屏幕录制推流和 UI 树检查都需要设备上的代理。如果设备上没有运行代理，标准 iOS 工具链无法提供这些能力。

在 **Android** 上不需要代理。在 Android 设备上运行 `mobilecli agent status` 只会报告这一点。

```bash
# Check if the agent is installed on a device
mobilecli agent status --device <device-id>

# Install the agent
mobilecli agent install --device <device-id>

# Force reinstall the agent
mobilecli agent install --device <device-id> --force

# Install on a real iOS device (requires provisioning profile)
mobilecli agent install --device <device-id> --provisioning-profile /path/to/profile.mobileprovision
```

`agent status` 的输出示例：
```json
{
  "status": "ok",
  "data": {
    "message": "Agent version 0.0.12 is installed on device",
    "agent": {
      "version": "0.0.12",
      "bundleId": "com.mobilenext.devicekit-iosUITests.xctrunner"
    }
  }
}
```

### WebView 检查 🌐

检查并操作原生应用中运行的内嵌 WebView（iOS 上为 `WKWebView`，Android 上为 `android.webkit.WebView`）。

```bash
# List embedded webviews in the foreground app
mobilecli webview list --device <device-id>

# Navigate a webview to a URL
mobilecli webview goto <id> https://example.com --device <device-id>

# Reload, go back or forward
mobilecli webview reload <id> --device <device-id>
mobilecli webview back <id> --device <device-id>
mobilecli webview forward <id> --device <device-id>

# Get current URL and page title
mobilecli webview url <id> --device <device-id>
mobilecli webview title <id> --device <device-id>

# Dump the full HTML content of the page
mobilecli webview content <id> --device <device-id>

# Query DOM elements by CSS selector
mobilecli webview query <id> "button" --device <device-id>
mobilecli webview query <id> "[data-testid='submit']" --device <device-id>

# Evaluate arbitrary JavaScript
mobilecli webview eval <id> "document.querySelectorAll('a').length" --device <device-id>

# Wait for the page to finish loading
mobilecli webview wait <id> --state load --device <device-id>
mobilecli webview wait <id> --state domcontentloaded --timeout 5000 --device <device-id>
```

`webview list` 的输出示例：
```json
{
  "status": "ok",
  "data": [
    {
      "id": "1",
      "url": "https://example.com",
      "title": "Example Domain"
    }
  ]
}
```

`webview query <id> "button"` 的输出示例：
```json
{
  "status": "ok",
  "data": [
    { "tag": "button", "text": "Sign In", "id": "login-btn", "class": "btn-primary", "value": null, "href": null },
    { "tag": "button", "text": "Cancel", "id": null, "class": "btn-secondary", "value": null, "href": null }
  ]
}
```

### 崩溃报告 💥

```bash
# List crash reports from a device
mobilecli device crashes list --device <device-id>

# Get a specific crash report by ID
mobilecli device crashes get <crash-id> --device <device-id>
```

`crashes list` 的输出示例：
```json
{
  "status": "ok",
  "data": [
    {
      "processName": "ShareExtension",
      "timestamp": "2026-01-24-195529",
      "id": "ShareExtension-2026-01-24-195529.ips"
    }
  ]
}
```

**注意**：在 iOS 真机上，崩溃报告通过 Apple 的 crashreport 服务获取。在 iOS 模拟器上，从 `~/Library/Logs/DiagnosticReports/` 读取。在 Android 上，从 `adb logcat -b crash` 的输出中解析。

### 设备日志 📋

```bash
# Stream logs from a device (Ctrl+C to stop)
mobilecli device logs --device <device-id>

# Stop after 100 entries
mobilecli device logs --device <device-id> --limit 100

# Filter by field (exact match)
mobilecli device logs --filter process=SpringBoard
mobilecli device logs --filter tag=ActivityManager

# Exclude by field
mobilecli device logs --filter process!=SpringBoard

# Combine filters (AND logic)
mobilecli device logs --filter level=Error --filter process!=SpringBoard
```

支持的过滤键：`pid`、`process`、`tag`、`level`、`subsystem`、`category`、`message`

每条日志以一行 JSON 输出：
```json
{"timestamp":"2026-04-15 12:17:14.224451+0300","message":"Start proc...","level":"Default","subsystem":"com.apple.UIKit","category":"EventDispatch","pid":54052,"process":"SpringBoard"}
```

日志也可以通过 [HTTP API](#http-api-) 的 `device.logs` 方法获取。它接受同样的 `limit` 和 `filters` 参数，并返回一个用于流式读取的 URL：

```bash
curl -X POST http://localhost:12000/rpc -H "Content-Type: application/json" -d '{
  "jsonrpc": "2.0", "method": "device.logs", "id": 1,
  "params": { "deviceId": "<device-id>", "limit": 100, "filters": ["level=Error", "process!=SpringBoard"] }
}'
# => {"jsonrpc":"2.0","id":1,"result":{"sessionUrl":"/sessions/<session-id>/logs"}}

curl -N http://localhost:12000/sessions/<session-id>/logs
```

会话在创建一分钟后过期，且只接受一个连接。断开连接会停止设备上的日志流。

### 远程设备 ☁️

```bash
# Allocate a remote iOS device
mobilecli remote allocate --platform ios --version ">=18" --name "iPhone*" --wait

# Allocate a remote Android device (exact OS version)
mobilecli remote allocate --platform android --version 14 --wait

# List available remote devices
mobilecli remote list-devices

# Release an allocated remote device
mobilecli remote release --device <device-id>
```

## Claude Code 技能 🤖

本仓库包含一个智能体技能（[skills/mobilecli/SKILL.md](skills/mobilecli/SKILL.md)），用于教 Claude Code（或任何兼容 SKILL.md 的智能体）如何驱动 `mobilecli`：列出设备、点击和输入文字、导出 UI 树、管理应用，以及使用 JSON-RPC 服务器进行快速自动化。

使用 [skills](https://github.com/vercel-labs/skills) 安装：

```bash
# current project only
npx skills add mobile-next/mobilecli

# or globally, for all projects
npx skills add mobile-next/mobilecli -g
```

之后，向你的智能体提出诸如“给我的仿真器截个图”或“点击登录按钮”之类的请求，技能就会自动触发。

## 守护进程 ⚙️

每个设备命令都会与一个按用户运行的后台守护进程通信。守护进程在多次调用之间保持已发现的设备、iOS 隧道和设备上的代理处于活动状态，因此只有第一个命令需要承担设备发现的开销。守护进程会在第一个设备命令执行时自动启动（`mobilecli --help` 和 `--version` 绝不会启动它），并在 30 分钟没有请求后退出。

```bash
# Inspect or control it explicitly
mobilecli daemon status
mobilecli daemon stop

# Run it in the foreground (e.g. under a service manager), with a custom idle timeout
mobilecli daemon start --idle-timeout 0
```

CLI 与守护进程通过 `~/.mobilecli/` 下的 unix 域套接字进行 JSON-RPC 通信（可通过 `MOBILECLI_HOME` 覆盖）；守护进程的日志位于 `~/.mobilecli/daemon.log`。旧版本 mobilecli 遗留的守护进程会被自动重启。`auth login` 和 `auth logout` 会停止正在运行的守护进程，以便它加载新的凭据。

## HTTP API 🔌

***mobilecli*** 为命令行中可用的全部功能提供了 HTTP 接口。该服务器是上述守护进程的一层轻量前端，因此 HTTP 客户端和 CLI 共享同一组设备和隧道。

完整的 JSON-RPC 方法列表及其参数，请参阅 [OpenRPC 规范](https://github.com/mobile-next/mobile-openrpc/blob/main/mobilecli/openrpc.md)。

```bash
# Start the server (default port 12000)
mobile server start

curl http://localhost:12000/rpc -XPOST -d '{"jsonrpc":"2.0", "id": 1, "method": "devices", "params": {}}'
curl http://localhost:12000/rpc -XPOST -d '{"jsonrpc":"2.0", "id": 1, "method": "screenshot", "params": {"deviceId": "your-device-id"}}'
```

## WebSocket 支持 🔌

***mobilecli*** 内置了一个 WebSocket 服务器，允许在单个连接上发送多个请求，使用与 HTTP API 相同的 JSON-RPC 2.0 格式。

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

**注意**：WebSocket 不支持 `screencapture`，视频推流请使用 HTTP 的 `/rpc` 端点。

## 平台相关说明

### iOS 真机
- 需要设备上的代理。使用 `mobilecli agent install --device <device-id> --provisioning-profile /path/to/profile.mobileprovision` 安装。需要有效的 Apple 描述文件（provisioning profile），以便为你的设备重新签名代理。

## 开发 👩‍💻

### 构建 🛠️

关于在本地测试 *mobilecli* 的更多说明，请参阅 (docs/TESTING.md)。

```bash
make lint
make build
make test
```

## 支持 💬

如有问题或功能需求，请使用 [GitHub Issues](https://github.com/mobile-next/mobilecli/issues) 页面。

欢迎立即<a href="http://mobilenext.ai/join-slack">加入我们的 Slack 频道</a> 💜

了解更多关于 <a href="https://mobilenext.ai/">Mobile Next</a> 以及我们正在构建的产品。
