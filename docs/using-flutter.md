# Flutter apps

Flutter draws its whole UI into one native view. The OS accessibility tree only
sees what Flutter publishes to it: merged, typeless nodes, and nothing at all for
widgets without semantics (icon-only buttons, `CustomPaint`, app bar titles on
iOS). So for Flutter apps `mobilecli dump ui` reads the live render tree from the
Dart VM service instead, and falls back to the accessibility dump when it can't.

## Requirements

- A **debug or profile** build. Release builds have no Dart VM service.
- The app must be in the **foreground**.
- Nothing else: no `flutter run`, no extra flags, no app changes.

## How it works per platform

All three share the same walk: connect to the VM service over WebSocket, find the
root `RenderView`, walk render objects with `invoke` (no expression compiler
needed, so a normally-launched app works), read each node's bounds via
`localToGlobal`, and read labels/roles from its `debugSemantics`. Semantics are
enabled for the duration of the dump and disposed afterwards. The only
platform-specific part is getting the VM service URI (port + auth token).

| Platform | Flutter detection | VM service URI | Reaching it |
|---|---|---|---|
| Android | app is debuggable and the injected agent finds `FlutterJNI` | agent calls `FlutterJNI.getVMServiceUri()` in-process | `adb forward` to the device port |
| iOS simulator | app bundle contains `Frameworks/Flutter.framework` | mDNS `_dartVmService._tcp` (what `flutter attach` uses); simulator log as fallback | direct, the app runs on the Mac's loopback |
| iOS device | injected agent finds a `FlutterViewController` on screen | agent reads `engine.publisher.url` in-process | port forward over the go-ios tunnel |

Coordinates: Android is scaled from logical to physical pixels using the device
density; iOS reports logical points, so no scaling.

## What you get

Elements carry the Flutter render type (`Text`, `CustomPaint`, `Image`...) and,
where semantics exist, `label`, `value`, `identifier` (from `Semantics(identifier:)`),
`placeholder`, `checked`, `selected`, `focused`, `enabled`. Types are refined to
`TextField`, `Button`, `Checkbox`, `Radio`, `Switch`, `Slider`, `Link`, `Header`
so `getByRole` works. A control such as a `CheckboxListTile` is one element, not
its inner label and box.

## Is it working?

Run with `-v`:

```sh
mobilecli dump ui -v
```

Look for one of these lines:

```
flutter: render-tree dump produced 42 elements in 1.2s     # VM service path used
flutter: vmServiceUri for com.example.app: not a flutter app  # not Flutter, a11y dump used
flutter: no Dart VM service URI found for ... (release build, ...)  # sim: release build
flutter: render-tree dump failed, falling back: ...        # VM path broke, a11y dump used
```

Without `-v`, the tell is in the output: the accessibility dump reports generic
types (`android.view.View`, `Other`) and no `CustomPaint` or `Text` types.

## Demo app

[mobilewright-examples/flutter](https://github.com/mobile-next/mobilewright-examples/tree/main/flutter)
is a small app built to expose the gaps. Build it debug, open it, and dump:

```sh
cd mobilewright-examples/flutter
flutter build apk --debug              # or: flutter build ios --simulator --debug
mobilecli apps install build/app/outputs/flutter-apk/app-debug.apk
mobilecli apps launch com.example.flutter_demo
mobilecli dump ui --format text
```

With the VM service path these show up; with the accessibility dump they are
missing or degraded:

| Element | Accessibility dump | VM service |
|---|---|---|
| Icon-only "+" button | unlabeled view on Android, missing on iOS | `Button` with bounds |
| "Premium package" row, app bar title | missing on iOS | present |
| `ListTile` with trailing button | merged into one node | separate elements |
| `CustomPaint` chart | invisible | `CustomPaint` with bounds |
| Empty `TextField` with `labelText` | hint only, or nothing on Android <= 14 | `TextField` with label |
| Widget types, `Semantics` identifiers | lost | present |
