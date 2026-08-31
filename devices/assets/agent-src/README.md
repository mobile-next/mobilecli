# On-device automation agent

A persistent on-device server that replaces per-command `adb` for the hot-path
Android operations. It is launched once via `app_process` (running as the shell
uid), holds a `UiAutomation` connection open for the process lifetime, and
serves requests over a `localabstract` socket reached from the host via
`adb forward`.

Because the process and its `UiAutomation` connection persist, there is no
per-request instrumentation spawn and no fixed idle wait — which is what makes
the adb paths slow.

- **Screenshot** — `UiAutomation.takeScreenshot()`
- **Input** — `InputManager.injectInputEvent()` (real `MotionEvent` / `KeyEvent`),
  text via the on-device `KeyCharacterMap`

All framework-hidden APIs are reached by reflection so the agent compiles
against the public SDK `android.jar`.

## Integration

`devices/android_agent.go` embeds the compiled `mw-agent.dex`, performs one-time
setup (push → launch → `adb forward` → connect), and routes `TakeScreenshot`,
`Tap`, `LongPress`, `Swipe`, `Gesture`, `PressButton`, `PressKeys`, and ASCII
`SendKeys` through the agent — each with **automatic fallback** to the existing
adb path on any error. A dropped connection triggers one transparent reconnect.

Hierarchy dumps (`DumpSource`/`DumpSourceRaw`) are intentionally not handled by
this agent — `upstream/main` already ships a persistent DeviceKit server for
fast UI-tree dumps (`am instrument com.mobilenext.devicekit/.DeviceKitServer`),
so this PR leaves that operation on the existing devicekit/uiautomator path
rather than duplicating it.

Disable entirely with `MOBILECLI_DISABLE_AGENT=1`.

## Rebuilding the dex

The embedded `devices/assets/mw-agent.dex` is regenerated from `Agent.java`:

```sh
./devices/assets/agent-src/build.sh   # writes devices/assets/mw-agent.dex
```

Requires a JDK and the Android SDK (`android.jar` + build-tools `d8`).
