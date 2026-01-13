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

## Methods

This section documents the JSON-RPC methods registered by the server and shows example `params` and curl requests.

- `devices`
	- Description: List devices.
	- Params: object (optional)
		- `includeOffline` (bool)
		- `platform` (string)
		- `type` (string)
	- Example:

```json
{"jsonrpc":"2.0","method":"devices","params":{"includeOffline":true},"id":1}
```

- `screenshot`
	- Description: Take a screenshot and return base64 data.
	- Params: object
		- `deviceId` (string)
		- `format` (string) - `png` or `jpeg`
		- `quality` (int) - JPEG quality 1-100
	- Example:

```json
{"jsonrpc":"2.0","method":"screenshot","params":{"deviceId":"<id>","format":"png"},"id":2}
```

- `screencapture` (streaming)
	- Description: Start a screen capture stream (MJPEG or AVC). This writes a streaming response; use the HTTP `/rpc` POST to initiate streaming.
	- Params: object
		- `deviceId` (string)
		- `format` (string) - `mjpeg` or `avc`
		- `quality` (int)
		- `scale` (float)
	- Note: Progress notifications are sent inside the stream for `mjpeg`.

- `io_tap`
	- Description: Tap at coordinates.
	- Params: object
		- `deviceId` (string)
		- `x` (int)
		- `y` (int)
	- Example:

```json
{"jsonrpc":"2.0","method":"io_tap","params":{"deviceId":"<id>","x":100,"y":200},"id":3}
```

- `io_longpress`
	- Description: Long-press at coordinates.
	- Params: object
		- `deviceId` (string)
		- `x` (int)
		- `y` (int)
		- `duration` (int, ms)

- `io_text`
	- Description: Send text input.
	- Params: object
		- `deviceId` (string)
		- `text` (string)

- `io_button`
	- Description: Press a hardware button (HOME, VOLUME_UP, etc.).
	- Params: object
		- `deviceId` (string)
		- `button` (string)

- `io_swipe`
	- Description: Swipe from one point to another.
	- Params: object
		- `deviceId` (string)
		- `x1`,`y1`,`x2`,`y2` (ints)

- `io_gesture`
	- Description: Perform a gesture composed of actions (tap, move, wait, etc.).
	- Params: object
		- `deviceId` (string)
		- `actions` (array of action objects) — see `devices/wda` types in code for schema.

- `url`
	- Description: Open a URL or deep link on device.
	- Params: object
		- `deviceId` (string)
		- `url` (string)

- `device_info`
	- Description: Retrieve detailed device info (screen size, OS, etc.).
	- Params: object
		- `deviceId` (string)
	- Example:

```json
{"jsonrpc":"2.0","method":"device_info","params":{"deviceId":"<id>"},"id":21}
```

- `io_orientation_get`
	- Description: Get device orientation.
	- Params: object
		- `deviceId` (string)

- `io_orientation_set`
	- Description: Set device orientation.
	- Params: object
		- `deviceId` (string)
		- `orientation` (string)

- `device_boot`, `device_shutdown`, `device_reboot`
	- Description: Control device power state (boot, shutdown, reboot).
	- Params: object
		- `deviceId` (string)

- `dump_ui`
	- Description: Dump the UI tree from the device.
	- Params: object
		- `deviceId` (string)
		- `format` (string) - `json` (default) or `raw`

- `apps_launch`
	- Description: Launch an app on a device.
	- Params: object
		- `deviceId` (string)
		- `bundleId` (string) - required

- `apps_terminate`
	- Description: Terminate a running app on a device.
	- Params: object
		- `deviceId` (string)
		- `bundleId` (string) - required

- `apps_list`
	- Description: List installed apps on a device.
	- Params: object (optional)
		- `deviceId` (string)

Common notes:
- For most methods `deviceId` is optional; when omitted the server auto-selects a single online device or returns an error when multiple devices are available.
- Methods that interact with the UI/agent (`io_*`, `dump_ui`, `apps_launch`, `device_info`, etc.) call `StartAgent` which may start/forward WDA for iOS devices. If WDA is unresponsive the server will attempt to relaunch it.

## Curl examples

- List devices:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"devices","params":{},"id":10}'
```

- Take a screenshot:

```bash
curl -s -X POST http://localhost:12000/rpc \
	-H 'Content-Type: application/json' \
	-d '{"jsonrpc":"2.0","method":"screenshot","params":{"deviceId":"<id>","format":"png"},"id":11}'
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
	-d '{"jsonrpc":"2.0","method":"io_tap","params":{"deviceId":"<id>","x":195,"y":422},"id":13}'
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
