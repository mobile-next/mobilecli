# Test Setup

## iOS

The iOS simulator tests look up an existing simulator by name — no simulator is created automatically. You must create and boot the simulator before running the tests.

The expected simulator name for iOS 26 is `Test-iOS-26`. To create it:

**1. Find the iOS 26 runtime identifier**

```sh
xcrun simctl list runtimes
```

Look for a line containing `iOS 26.` and note its identifier (e.g. `com.apple.CoreSimulator.SimRuntime.iOS-26-0`).

**2. Create the simulator**

```sh
xcrun simctl create "Test-iOS-26" "iPhone 14" com.apple.CoreSimulator.SimRuntime.iOS-26-0
```

**3. Boot it**

```sh
xcrun simctl boot "Test-iOS-26"
```

**4. Verify it is visible to mobilecli**

```sh
mobilecli devices
```

You should see the simulator listed with `"platform": "ios"` and `"type": "simulator"`. Once it's there, run the tests.

---

## Android

The Android tests pick the first available device reported by `mobilecli devices`. No emulator is created automatically — you need one already running before executing the tests.

### Setting up an Android emulator

**Prerequisites**

- Android SDK installed and `ANDROID_HOME` set (e.g. `~/Library/Android/sdk`)
- `cmdline-tools` installed via Android Studio SDK Manager or `sdkmanager`

**1. Download a system image**

```sh
$ANDROID_HOME/cmdline-tools/latest/bin/sdkmanager "system-images;android-36;google_apis_playstore;x86_64"
```

Replace `android-36` and `x86_64` with your target API level and architecture. On Apple Silicon use `arm64-v8a`.

**2. Create an AVD**

```sh
echo "no" | $ANDROID_HOME/cmdline-tools/latest/bin/avdmanager create avd \
  -n "test-android-36" \
  -k "system-images;android-36;google_apis_playstore;x86_64" \
  -d "pixel"
```

**3. Launch the emulator**

```sh
$ANDROID_HOME/emulator/emulator -avd test-android-36 -no-snapshot-save &
```

**4. Verify it's visible to mobilecli**

```sh
mobilecli devices
```

You should see the emulator listed with `"platform": "android"`. Once it's there, run the tests.
