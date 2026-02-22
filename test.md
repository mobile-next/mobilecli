# mobilecli Test Campaign

Manual test campaign to validate all device interactions as the sole backend for iOS simulators and real devices.

## Setup

Build the binary before running any test:

```bash
go build -o mobilecli .
```

Identify your devices:

```bash
./mobilecli devices -v
```

Export IDs for use in commands below:

```bash
# Replace with your actual IDs from `./mobilecli devices`
export SIM=<simulator-udid>
export DEVICE=<real-device-udid>
```

---

## 1. Device Management

### List devices

```bash
./mobilecli devices
./mobilecli devices --include-offline --platform ios --type simulator
```

### Device info (triggers DeviceKit agent startup)

```bash
./mobilecli device info --device $SIM -v
./mobilecli device info --device $DEVICE -v
```

Expected: verbose output shows "Starting DeviceKit" — no WDA mention.

### Orientation

```bash
./mobilecli device orientation get --device $SIM
./mobilecli device orientation get --device $DEVICE

./mobilecli device orientation set --device $DEVICE landscape
./mobilecli device orientation set --device $DEVICE portrait
```

---

## 2. Screenshots

```bash
# PNG (simulator)
./mobilecli screenshot --device $SIM -o /tmp/sim_screen.png && open /tmp/sim_screen.png

# JPEG (simulator)
./mobilecli screenshot --device $SIM -o /tmp/sim_screen.jpg -f jpeg -q 85 && open /tmp/sim_screen.jpg

# PNG (real device)
./mobilecli screenshot --device $DEVICE -o /tmp/dev_screen.png && open /tmp/dev_screen.png

# JPEG (real device)
./mobilecli screenshot --device $DEVICE -o /tmp/dev_screen.jpg -f jpeg -q 85 && open /tmp/dev_screen.jpg
```

---

## 3. Screen Capture (Streaming)

Requires `ffplay` (`brew install ffmpeg`).

### Simulator

```bash
# MJPEG — DeviceKit /mjpeg endpoint
./mobilecli screencapture --device $SIM -f mjpeg | ffplay -i - -vf scale=390:-1 2>/dev/null

# AVC — DeviceKit /h264 endpoint
./mobilecli screencapture --device $SIM -f avc | ffplay -i - 2>/dev/null

# avc+replay-kit — must print a warning and exit with error (not supported on simulator)
./mobilecli screencapture --device $SIM -f avc+replay-kit
```

Expected for `avc+replay-kit`: log warning + non-zero exit, no panic.

### Real device

```bash
# MJPEG
./mobilecli screencapture --device $DEVICE -f mjpeg | ffplay -i - -vf scale=390:-1 2>/dev/null

# AVC — DeviceKit /h264 endpoint
./mobilecli screencapture --device $DEVICE -f avc | ffplay -i - 2>/dev/null

# avc+replay-kit — BroadcastExtension (requires devicekit-ios app installed on device)
./mobilecli screencapture --device $DEVICE -f avc+replay-kit | ffplay -i - 2>/dev/null
```

---

## 4. Input / Output

```bash
# Tap
./mobilecli io tap --device $SIM 200,400
./mobilecli io tap --device $DEVICE 200,400

# Long press
./mobilecli io longpress --device $SIM 200,400
./mobilecli io longpress --device $DEVICE 200,400

# Swipe (x1,y1,x2,y2)
./mobilecli io swipe --device $SIM 200,600,200,200
./mobilecli io swipe --device $DEVICE 200,600,200,200

# Text input (focus a text field first)
./mobilecli io text --device $SIM "hello world"
./mobilecli io text --device $DEVICE "hello world"

# Hardware buttons: HOME, VOLUME_UP, VOLUME_DOWN, POWER
./mobilecli io button --device $SIM HOME
./mobilecli io button --device $DEVICE HOME
```

---

## 5. App Management

```bash
# List installed apps
./mobilecli apps list --device $SIM
./mobilecli apps list --device $DEVICE

# Foreground app
./mobilecli apps foreground --device $SIM
./mobilecli apps foreground --device $DEVICE

# Launch and verify foreground
./mobilecli apps launch --device $SIM com.apple.Preferences
./mobilecli apps foreground --device $SIM    # expected: com.apple.Preferences

./mobilecli apps launch --device $DEVICE com.apple.Preferences
./mobilecli apps foreground --device $DEVICE

# Terminate
./mobilecli apps terminate --device $SIM com.apple.Preferences
./mobilecli apps terminate --device $DEVICE com.apple.Preferences

# Press HOME to go back to home screen
./mobilecli io button --device $SIM HOME
./mobilecli io button --device $DEVICE HOME
```

---

## 6. UI Dump

```bash
./mobilecli dump ui --device $SIM
./mobilecli dump ui --device $DEVICE
```

Expected: JSON array of screen elements with type, label, and rect fields.

---

## 7. Open URL / Deep Link

```bash
./mobilecli url --device $SIM https://apple.com
./mobilecli url --device $DEVICE https://apple.com
```

---

## Pass Criteria

| Test | Simulator | Real device |
|---|---|---|
| `device info` launches DeviceKit (no WDA in logs) | | |
| `screenshot` produces a valid image | | |
| `screencapture -f mjpeg` streams video | | |
| `screencapture -f avc` streams H.264 | | |
| `screencapture -f avc+replay-kit` warns + errors on simulator | N/A — must error | |
| `io tap/swipe/text/button` execute on screen | | |
| `apps foreground` returns active bundle ID | | |
| `dump ui` returns element tree | | |
| No "wda" or "WebDriverAgent" in `-v` logs | | |
