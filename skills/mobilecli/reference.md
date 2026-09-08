# mobilecli reference

Full command reference and the JSON-RPC API. Read this when a command shape is unclear or when driving mobilecli from scripts.

---

## Command Reference (CLI)

All commands support the global `--device <id>` flag to specify the target device, and `-v` / `--verbose` for logging.

### 1. Device Lifecycle & Info
* **List Devices**:
  ```bash
  # List online devices
  mobilecli devices

  # List all devices including offline ones
  mobilecli devices --include-offline
  ```
* **Boot Device** (start an offline simulator or emulator):
  ```bash
  mobilecli device boot --device <device-id>
  ```
* **Shutdown / Reboot**:
  ```bash
  mobilecli device shutdown --device <device-id>
  mobilecli device reboot --device <device-id>
  ```
* **Device Information**:
  ```bash
  mobilecli device info --device <device-id>
  ```
* **Orientation Control**:
  ```bash
  # Get current orientation (portrait/landscape)
  mobilecli device orientation get --device <device-id>

  # Set orientation
  mobilecli device orientation set --device <device-id> landscape
  ```

### 2. App Management
* **List Apps**:
  ```bash
  mobilecli apps list --device <device-id>
  ```
* **Foreground App**:
  ```bash
  mobilecli apps foreground --device <device-id>
  ```
* **Launch / Terminate**:
  ```bash
  mobilecli apps launch <bundle-id> --device <device-id>
  mobilecli apps terminate <bundle-id> --device <device-id>
  ```
* **Install / Uninstall**:
  ```bash
  # Installs .apk (Android), .ipa (iOS Real), or .zip (iOS Simulator)
  mobilecli apps install /path/to/app.apk --device <device-id>

  # Uninstall
  mobilecli apps uninstall <bundle-id> --device <device-id>
  ```

### 3. Screen & Media
* **Take Screenshot**:
  ```bash
  # PNG format (default)
  mobilecli screenshot --device <device-id> --output screenshot.png

  # JPEG with quality control
  mobilecli screenshot --device <device-id> --format jpeg --quality 80 --output screenshot.jpg
  ```
* **Record Screen**:
  ```bash
  # Record screen to MP4 file
  mobilecli screenrecord --device <device-id> --output recording.mp4

  # Record with custom time limit (in seconds) and suppress progress output
  mobilecli screenrecord --device <device-id> --output recording.mp4 --time-limit 15 --silent
  ```

### 4. Input & Gestures
* **Tap** (ref from the latest `dump ui`, or coordinates):
  ```bash
  mobilecli io tap --device <device-id> @e5
  mobilecli io tap --device <device-id> 150,300
  ```
* **Long Press**:
  ```bash
  mobilecli io longpress --device <device-id> 150,300 --duration 2000
  ```
* **Swipe**:
  ```bash
  # Swipe from x1,y1 to x2,y2
  mobilecli io swipe --device <device-id> 100,600,100,200
  ```
* **Send Text**:
  ```bash
  # Types text into the currently focused input field
  mobilecli io text --device <device-id> "John Doe"
  ```
* **Hardware Buttons**:
  ```bash
  # All platforms: HOME, POWER, VOLUME_UP, VOLUME_DOWN
  # Android only: BACK, ENTER, BACKSPACE, APP_SWITCH,
  #               DPAD_UP, DPAD_DOWN, DPAD_LEFT, DPAD_RIGHT, DPAD_CENTER
  mobilecli io button --device <device-id> HOME
  ```
* **Clipboard**:
  ```bash
  # Read the device clipboard
  mobilecli io clipboard get --device <device-id>

  # Write to the device clipboard
  mobilecli io clipboard set --device <device-id> "Hello World"
  ```

### 5. UI Inspection & Webviews
* **Dump UI Tree**:
  ```bash
  # Parsed JSON with rect and @ref per element (default)
  mobilecli dump ui --device <device-id>

  # Indented one-line-per-element text, easiest to read
  mobilecli dump ui --device <device-id> --format text

  # Raw XML/JSON source from agent
  mobilecli dump ui --device <device-id> --format raw

  # Include elements normally left out, such as the on-screen keyboard (Android)
  mobilecli dump ui --device <device-id> --full
  ```
* **List Webviews**:
  ```bash
  mobilecli webview list --device <device-id>
  ```
* **Webview Navigation & Query**:
  ```bash
  # Navigate to a URL
  mobilecli webview goto <webview-id> https://example.com --device <device-id>

  # Query DOM elements via CSS selector
  mobilecli webview query <webview-id> "button.submit-btn" --device <device-id>

  # Dump full outer HTML
  mobilecli webview content <webview-id> --device <device-id>
  ```
* **Evaluate JavaScript**:
  ```bash
  mobilecli webview eval <webview-id> "document.title" --device <device-id>
  ```
* **Wait for Load State**:
  ```bash
  # Wait states: "load" or "domcontentloaded"
  mobilecli webview wait <webview-id> --state domcontentloaded --timeout 5000 --device <device-id>
  ```

### 6. Filesystem Operations
Access files on-device or inside debuggable app private directories (Android and iOS Simulator).
* **List Directory**:
  ```bash
  # Absolute path
  mobilecli fs ls --device <device-id> /sdcard/Download

  # App private container path
  mobilecli fs ls --device <device-id> com.example.app /Documents
  ```
* **Transfer Files**:
  ```bash
  # Push local file to device
  mobilecli fs push --device <device-id> ./config.json /sdcard/config.json

  # Pull remote file to host
  mobilecli fs pull --device <device-id> /sdcard/log.txt ./log.txt
  ```
* **Directories & Deletion**:
  ```bash
  # Create directory
  mobilecli fs mkdir --device <device-id> -p /sdcard/newdir

  # Delete file or directory
  mobilecli fs rm --device <device-id> -r /sdcard/newdir
  ```

### 7. Crash Logs & Deep Linking
* **Deep Links**:
  ```bash
  mobilecli url --device <device-id> "myapp://settings?user=123"
  ```
* **Crash Reports**:
  ```bash
  # List crash logs
  mobilecli device crashes list --device <device-id>

  # Get crash report content
  mobilecli device crashes get <crash-id> --device <device-id>
  ```
* **Device Logs**:
  ```bash
  # Stream live logs, one json entry per line (ctrl+c to stop)
  mobilecli device logs --device <device-id>

  # Stop after 100 entries, filter with key=value / key!=value (repeatable, ANDed)
  mobilecli device logs --device <device-id> --limit 100 --filter level=Error --filter process!=SpringBoard
  ```
  Filter keys: `pid`, `process`, `tag`, `level`, `subsystem`, `category`, `message`

---

## JSON-RPC Server & WebSocket API

For scripts and long-running automation, make HTTP POST requests to the server's endpoint (`http://localhost:12000/rpc`). The JSON-RPC payload format is:
`{"jsonrpc": "2.0", "method": "<method_name>", "params": { ... }, "id": 1}`

### Core JSON-RPC API Examples

* **List Devices**:
  ```bash
  curl http://localhost:12000/rpc -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0", "id": 1, "method": "devices.list", "params": {}}'
  ```
* **Take Crop/Clip Screenshot**:
  ```bash
  curl http://localhost:12000/rpc -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0", "id": 2, "method": "device.screenshot", "params": {"deviceId": "device-id", "format": "png", "clip": {"x": 50, "y": 100, "width": 200, "height": 300}}}'
  ```
* **Stop Server Remotely**:
  ```bash
  curl http://localhost:12000/rpc -X POST -H "Content-Type: application/json" \
    -d '{"jsonrpc":"2.0", "id": 3, "method": "server.shutdown", "params": {}}'
  ```

### Custom Gestures (JSON-RPC only)
For complex multi-action interactions (e.g. dragging, pinching, or specific curves) which are not accessible via standard CLI gestures, use the `device.io.gesture` method. This allows you to chain raw pointer motion events.

Supported actions:
- `pointerDown`: Touches screen at the current x,y coordinate.
- `pointerMove`: Moves coordinates to target `x`, `y`.
- `pointerUp`: Lifts pointer off screen.
- `pause`: Sleeps for `duration` (in milliseconds).

**Example: Drag-and-Drop Action**
```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "device.io.gesture",
  "params": {
    "deviceId": "my-device-id",
    "actions": [
      { "type": "pointerMove", "x": 100, "y": 150 },
      { "type": "pointerDown" },
      { "type": "pause", "duration": 500 },
      { "type": "pointerMove", "x": 300, "y": 450 },
      { "type": "pause", "duration": 200 },
      { "type": "pointerUp" }
    ]
  }
}
```

### Filesystem Limits in JSON-RPC
> [!WARNING]
> **1MB RPC payload limit**: The JSON-RPC calls `device.fs.push` and `device.fs.pull` encode file data using Base64. To maintain server performance, the maximum file size supported by these RPC endpoints is **1 MB**.
>
> If you need to transfer databases, video files, or payloads larger than 1 MB, you must bypass the JSON-RPC endpoints and invoke the CLI commands directly (`mobilecli fs push` / `mobilecli fs pull`), which handle raw streams and are binary-safe for large volumes.

### WebSocket API
You can open a persistent connection to `ws://localhost:12000/ws` using tools like `wscat`:
```bash
wscat -c ws://localhost:12000/ws
> {"jsonrpc":"2.0","id":1,"method":"devices.list","params":{}}
< {"jsonrpc":"2.0","id":1,"result":[...]}
```

---

## Platform-Specific Notes & Troubleshooting

### iOS Real Devices
- **Agent Dependency**: Input gestures, screenshots, and UI dumps require the agent. Check and install it via:
  ```bash
  # Check agent installation status
  mobilecli agent status --device <device-id>

  # Install the agent (real iOS devices require a provisioning profile)
  mobilecli agent install --device <device-id> --provisioning-profile /path/to/profile.mobileprovision
  ```
  A valid Apple Provisioning Profile and signing identity must be present on the host to code-sign the agent.

### Android Real Devices & Emulators
- **ADB Access**: Ensure the device has "USB Debugging" enabled.
- **App Private Container Paths**: Android app containers (`/data/user/0/...`) are accessed using `run-as`, which requires the application to be built as **debuggable** (`android:debuggable="true"` in the manifest).

---
