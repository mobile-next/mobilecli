# Test Setup

## iOS

The iOS simulator tests use the first booted simulator reported by `mobilecli devices --platform ios --type simulator` — no simulator is created or booted automatically. You must create and boot one before running the tests, and a physical iPhone is never selected.

Any simulator will do; the examples below use `Test-iOS-26`. To create it:

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

The Android tests live in a single spec that runs against either an emulator or a
physical handset. The Playwright project decides which, through the `deviceType`
option, so there is one copy of the tests:

```sh
npm run test:emulator   # --type emulator (what CI runs)
npm run test:android    # --type real
```

Each picks the first device reported by `mobilecli devices --platform android --type <type>`.
Nothing is created or booted automatically — have the emulator running, or the handset
plugged in with USB debugging enabled, before running the tests.

**A real device must stay awake for the whole run.** Most tests do not generate input
events, so the screen-off timer keeps running; once the screen is off there is no focused
window and `apps foreground` fails. Enable *Developer options → Stay awake*, or:

```sh
adb shell svc power stayon usb     # revert with: adb shell svc power stayon false
```

Three tests are skipped on a real device: the screenshot size check (it assumes a known
screen state), the Settings tap test (it matches English UI labels), and the app-container
fs group (it needs a debuggable build of `com.mobilenext.playground` installed).

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
