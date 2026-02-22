# mobilecli JSON-RPC API

This document describes the JSON-RPC methods exposed by the mobilecli HTTP server at the `/rpc` endpoint.

Base URL: `http://<host>:<port>/rpc`

All requests must be JSON-RPC 2.0 objects with `jsonrpc` set to "2.0", a `method` string, an `id`, and optionally `params`.

Common response format:

```json
{
	"jsonrpc": "2.0",
	"result": ... ,
	"error": { "code": ..., "message": ..., "data": ... },
	"id": <id>
}
```

Errors use standard JSON-RPC error codes and include a `data` field with additional context.

## BREAKING CHANGE: Method Names Updated

**All JSON-RPC methods now use a `device.*` naming scheme. Old method names are no longer supported.**

### Migration Table

| Old Method Name | New Method Name |
|-----------------|-----------------|
| devices | devices.list |
| screenshot | device.screenshot |
| screencapture | device.screencapture |
| io_tap | device.io.tap |
| io_longpress | device.io.longpress |
| io_text | device.io.text |
| io_button | device.io.button |
| io_swipe | device.io.swipe |
| io_gesture | device.io.gesture |
| url | device.url |
| device_info | device.info |
| io_orientation_get | device.io.orientation.get |
| io_orientation_set | device.io.orientation.set |
| device_boot | device.boot |
| device_shutdown | device.shutdown |
| device_reboot | device.reboot |
| dump_ui | device.dump.ui |
| apps_launch | device.apps.launch |
| apps_terminate | device.apps.terminate |
| apps_list | device.apps.list |
| apps_foreground | device.apps.foreground |

## Methods

This section documents the JSON-RPC methods registered by the server and shows example `params` and curl requests.

- `devices.list`
	- Description: List devices.
	- Params: object (optional)
		- `includeOffline` (bool)
		- `platform` (string)
		- `type` (string)
	- Example:

```json
{"jsonrpc":"2.0","method":"devices.list","params":{"includeOffline":true},"id":1}
```

- `device.screenshot`
	- Description: Take a screenshot and return base64 data.
	- Params: object
		- `deviceId` (string)
		- `format` (string) - `png` or `jpeg`
		- `quality` (int) - JPEG quality 1-100
	- Example:

```json
{"jsonrpc":"2.0","method":"device.screenshot","params":{"deviceId":"<id>","format":"png"},"id":2}
```

- `device.screencapture` (streaming)
	- Description: Start a screen capture stream (MJPEG or AVC). This writes a streaming response; use the HTTP `/rpc` POST to initiate streaming.
	- Params: object
		- `deviceId` (string)
		- `format` (string) - `mjpeg` or `avc`
		- `quality` (int)
		- `scale` (float)
	- Note: Progress notifications are sent inside the stream for `mjpeg`.

- `device.io.tap`
	- Description: Tap at coordinates.
	- Params: object
		- `deviceId` (string)
		- `x` (int)
		- `y` (int)
	- Example:

```json
{"jsonrpc":"2.0","method":"device.io.tap","params":{"deviceId":"<id>","x":100,"y":200},"id":3}
```

- `device.io.longpress`
	- Description: Long-press at coordinates.
	- Params: object
		- `deviceId` (string)
		- `x` (int)
		- `y` (int)
		- `duration` (int, ms)

- `device.io.text`
	- Description: Send text input.
	- Params: object
		- `deviceId` (string)
		- `text` (string)

- `device.io.button`
	- Description: Press a hardware button (HOME, VOLUME_UP, etc.).
	- Params: object
		- `deviceId` (string)
		- `button` (string)

- `device.io.swipe`
	- Description: Swipe from one point to another.
	- Params: object
		- `deviceId` (string)
		- `x1`,`y1`,`x2`,`y2` (ints)

- `device.io.gesture`
	- Description: Perform a gesture composed of actions (tap, move, wait, etc.).
	- Params: object
		- `deviceId` (string)
		- `actions` (array of action objects) — see `types.TapAction` in code for schema.

- `device.url`
	- Description: Open a URL or deep link on device.
	- Params: object
		- `deviceId` (string)
		- `url` (string)

- `device.info`
	- Description: Retrieve detailed device info (screen size, OS, etc.).
	- Params: object
		- `deviceId` (string)
	- Example:

```json
{"jsonrpc":"2.0","method":"device_info","params":{"deviceId":"<id>"},"id":21}
```

- `device.io.orientation.get`
	- Description: Get device orientation.
	- Params: object
		- `deviceId` (string)

- `device.io.orientation.set`
	- Description: Set device orientation.
	- Params: object
		- `deviceId` (string)
		- `orientation` (string)

- `device.boot`, `device.shutdown`, `device.reboot`
	- Description: Control device power state (boot, shutdown, reboot).
	- Params: object
		- `deviceId` (string)

- `device.dump.ui`
	- Description: Dump the UI tree from the device.
	- Params: object
		- `deviceId` (string)
		- `format` (string) - `json` (default) or `raw`

- `device.apps.launch`
	- Description: Launch an app on a device.
	- Params: object
		- `deviceId` (string)
		- `bundleId` (string) - required

- `device.apps.terminate`
	- Description: Terminate a running app on a device.
	- Params: object
		- `deviceId` (string)
		- `bundleId` (string) - required

- `device.apps.list`
	- Description: List installed apps on a device.
	- Params: object (optional)
		- `deviceId` (string)

- `device.apps.foreground`
	- Description: Get the currently foreground (active) app on a device.
	- Params: object (optional)
		- `deviceId` (string)

Common notes:
- For most methods `deviceId` is optional; when omitted the server auto-selects a single online device or returns an error when multiple devices are available.
- Methods that interact with the UI/agent (`device.io.*`, `device.dump.ui`, `device.apps.launch`, `device.info`, etc.) call `StartAgent` which starts the DeviceKit XCUITest runner on iOS devices.

## Curl examples

- List devices:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"devices.list","params":{},"id":10}'
```

- Take a screenshot:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"device.screenshot","params":{"deviceId":"<id>","format":"png"},"id":11}'
```

- Start MJPEG screen capture (streaming):

```bash
curl -v -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"screencapture","params":{"deviceId":"<id>","format":"mjpeg"},"id":12}'
```

- Tap coordinates:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"device.io.tap","params":{"deviceId":"<id>","x":195,"y":422},"id":13}'
```

- Long press:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"io_longpress","params":{"deviceId":"<id>","x":100,"y":200,"duration":750},"id":14}'
```

- Send text:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"io_text","params":{"deviceId":"<id>","text":"Hello"},"id":15}'
```

- Press button:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"io_button","params":{"deviceId":"<id>","button":"HOME"},"id":16}'
```

- Swipe:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"io_swipe","params":{"deviceId":"<id>","x1":100,"y1":200,"x2":300,"y2":400},"id":17}'
```

- Open URL:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"url","params":{"deviceId":"<id>","url":"https://example.com"},"id":18}'
```

- Get device info:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"device_info","params":{"deviceId":"<id>"},"id":21}'
```

- Dump UI (json):

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"dump_ui","params":{"deviceId":"<id>","format":"json"},"id":19}'
```

- Launch app:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"apps_launch","params":{"deviceId":"<id>","bundleId":"com.example.app"},"id":20}'
```

- Terminate app:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"apps_terminate","params":{"deviceId":"<id>","bundleId":"com.example.app"},"id":21}'
```

- List apps:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"apps_list","params":{"deviceId":"<id>"},"id":22}'
```
