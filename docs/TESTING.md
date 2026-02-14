# Testing

This document describes how to run tests for mobilecli.

## Unit Tests

Run Go unit tests:

```bash
make test
```

## E2E Tests

E2E tests exercise the mobilecli binary against real simulators, emulators, and physical devices. Tests use **persistent, named test devices** that you create once and reuse across runs.

### Prerequisites

1. **Build the mobilecli binary**
   ```bash
   make build
   ```

2. **Install Node.js dependencies**
   ```bash
   cd test
   npm install
   ```

### Device Setup

Tests look for specific named devices. Create them once; they persist across test runs.

#### iOS Simulator: `mobilecli-test-sim`

```bash
xcrun simctl create "mobilecli-test-sim" "iPhone 16" com.apple.CoreSimulator.SimRuntime.iOS-26-0
```

Boot it before running tests:
```bash
xcrun simctl boot "mobilecli-test-sim"
```

#### Android Emulator: `mobilecli-test-emu`

```bash
avdmanager create avd -n "mobilecli-test-emu" -k "system-images;android-36;google_apis_playstore;arm64-v8a" -d "pixel_9"
```

Launch it before running tests:
```bash
emulator -avd mobilecli-test-emu &
```

#### iOS Real Device

Connect a real iPhone via USB. No naming required — the tests auto-detect any connected iOS device.

### Running Tests

Run all tests (unavailable devices are skipped automatically):

```bash
cd test
npm test
```

Run specific test suites:

```bash
# Server protocol tests only (no devices needed)
npm test -- --grep "server"

# iOS Simulator tests only
npm test -- --grep "iOS Simulator"

# Android Emulator tests only
npm test -- --grep "Android Emulator"

# iOS Real Device tests only
npm test -- --grep "iOS Real Device"
```

### Skip Behavior

Each test suite checks for its required device at startup:

| Suite | Looks for | If missing |
|---|---|---|
| iOS Simulator | Simulator named `mobilecli-test-sim` | Skips all simulator tests |
| Android Emulator | Running emulator named `mobilecli-test-emu` | Skips all emulator tests |
| iOS Real Device | Any connected iOS device | Skips all real device tests |
| Server | Nothing (starts its own server) | Always runs |

This means `npm test` always succeeds — it runs whatever is available and skips the rest.

### CI

E2E tests run on a self-hosted macOS ARM64 runner where the test devices are pre-configured. Server tests run on ubuntu-latest (no devices needed).
