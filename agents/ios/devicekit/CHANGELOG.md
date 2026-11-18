## [0.0.38](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.38) (2026-11-13)
* Feat: device.voiceover.* to drive VoiceOver and read what it speaks; requires iOS 27 or later and an agent built with Xcode 27 or later, calls on older versions return an error ([#109](https://github.com/mobile-next/devicekit-ios/pull/109)), thanks to [@vijaysharm](https://github.com/vijaysharm)
* Feat: per-finger start in device.io.gesture, for double taps ([#110](https://github.com/mobile-next/devicekit-ios/pull/110)), thanks to [@vijaysharm](https://github.com/vijaysharm)
* Fix: lay out screen size, screenshots and the ui tree in the foreground app's orientation ([#107](https://github.com/mobile-next/devicekit-ios/pull/107)), thanks to [@edusperoni](https://github.com/edusperoni)
* Fix: normalize MJPEG frame orientation at every scale ([#111](https://github.com/mobile-next/devicekit-ios/pull/111)), thanks to [@jorahbi](https://github.com/jorahbi)

## [0.0.37](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.37) (2026-11-09)
* Feat: add device.io.keyboard.status ([#60](https://github.com/mobile-next/devicekit-ios/pull/60))

## [0.0.36](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.36) (2026-10-31)
* Perf: make device.dump.ui fast on heavy React Native screens ([#105](https://github.com/mobile-next/devicekit-ios/pull/105)), thanks to [@avilashp-sportsbet](https://github.com/avilashp-sportsbet)

## [0.0.35](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.35) (2026-10-17)
* Fix: report keyboard focus from the snapshot's hasKeyboardFocus, so the Email field reports focus too ([#104](https://github.com/mobile-next/devicekit-ios/pull/104))
* Refactor: update naming conventions ([#103](https://github.com/mobile-next/devicekit-ios/pull/103))
* Docs: read the tunnel address on newer Xcode too ([#99](https://github.com/mobile-next/devicekit-ios/pull/99)), thanks to [@scelza](https://github.com/scelza)

## [0.0.34](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.34) (2026-10-13)
* Feat: clear key in device.io.keys sends the HID keyboard clear to wipe the focused text field ([#101](https://github.com/mobile-next/devicekit-ios/pull/101))
* Fix: bundle XCTest frameworks in the runner when Xcode 27 doesn't, and don't merge stale zips ([#100](https://github.com/mobile-next/devicekit-ios/pull/100))

## [0.0.33](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.33) (2026-10-01)
* Feat: listen on multiple addresses, including IPv6, in DEVICEKIT_LISTEN_HOST ([#92](https://github.com/mobile-next/devicekit-ios/pull/92)), thanks to [@hillct](https://github.com/hillct)
* Fix: report the unfolded screen size in device.info on foldables ([#91](https://github.com/mobile-next/devicekit-ios/pull/91))
* Fix: rotate landscape screenshots in the pixels, not with an exif tag ([#93](https://github.com/mobile-next/devicekit-ios/pull/93))
* Fix: fail the test run when the server can't start, log ready only once bound ([#94](https://github.com/mobile-next/devicekit-ios/pull/94))
* Chore: default deployment target to iOS 15.0, keep 14.0 for release builds ([#95](https://github.com/mobile-next/devicekit-ios/pull/95))

## [0.0.31](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.31) (2026-09-29)
* Fix: taps on folded foldables and unfolded springboard ([#89](https://github.com/mobile-next/devicekit-ios/pull/89))

## [0.0.30](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.30) (2026-09-28)
* Fix: tap, swipe and gestures on unfolded foldable simulators, and ui dump frames on the unfolded screen ([#88](https://github.com/mobile-next/devicekit-ios/pull/88))

## [0.0.29](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.29) (2026-09-28)
* Fix: don't offset ui dump frames when the app is rotated to landscape ([#86](https://github.com/mobile-next/devicekit-ios/pull/86))

## [0.0.28](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.28) (2026-09-28)
* Feat: device.io.hinge.set to fold and unfold foldable simulators ([#81](https://github.com/mobile-next/devicekit-ios/pull/81))
* Fix: background the test runner on iOS 27 simulators ([#84](https://github.com/mobile-next/devicekit-ios/pull/84))
* Fix: use portrait screen size when mapping landscape tap and swipe coordinates ([#83](https://github.com/mobile-next/devicekit-ios/pull/83)), thanks to [@hakanor](https://github.com/hakanor)
* Fix: screenshot the active screen on foldable simulators ([#82](https://github.com/mobile-next/devicekit-ios/pull/82))

## [0.0.27](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.27) (2026-09-15)
* Feat: device.settings.apply with light/dark appearance ([#78](https://github.com/mobile-next/devicekit-ios/pull/78))
* Feat: report the on-screen view controller in device.apps.foreground ([#77](https://github.com/mobile-next/devicekit-ios/pull/77))
* Fix: restore iOS 14 support, mirror swift concurrency runtime into runner below iOS 15 ([#75](https://github.com/mobile-next/devicekit-ios/pull/75), [#76](https://github.com/mobile-next/devicekit-ios/pull/76)), thanks to [@hakanor](https://github.com/hakanor)

## [0.0.26](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.26) (2026-08-31)
* Feat: include enabled, selected and focused state attributes in json ui dump ([#68](https://github.com/mobile-next/devicekit-ios/pull/68), [#69](https://github.com/mobile-next/devicekit-ios/pull/69))

## [0.0.25](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.25) (2026-08-25)
* Feat: device.clipboard.get and device.clipboard.set handlers ([#64](https://github.com/mobile-next/devicekit-ios/pull/64)), thanks to [@hakanor](https://github.com/hakanor)

## [0.0.24](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.24) (2026-08-22)
* Fix: sample the swipe path over its requested duration ([#62](https://github.com/mobile-next/devicekit-ios/pull/62)), thanks to [@hakanor](https://github.com/hakanor)
* Build: package main app (devicekit-ios.ipa) in ipa-unsigned so releases ship it ([#61](https://github.com/mobile-next/devicekit-ios/pull/61))

## [0.0.23](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.23) (2026-08-01)
* Fix: support xcode 26.5 by adding two missing testing frameworks ([#57](https://github.com/mobile-next/devicekit-ios/pull/57))

## [0.0.22](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.22) (2026-07-31)
* Fix: correct element position if it belongs to another window (fixes elements within widgets and such) ([#55](https://github.com/mobile-next/devicekit-ios/pull/55))

## [0.0.20](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.20) (2026-06-15)
* Feat: device.io.keys handler for sending key combos ([#51](https://github.com/mobile-next/devicekit-ios/pull/51))

## [0.0.19](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.19) (2026-06-14)
* Refactor: Split H264 streaming code out into a separate repository ([#47](https://github.com/mobile-next/devicekit-ios/pull/47), [#46](https://github.com/mobile-next/devicekit-ios/pull/46))
* Test: Migrate test suite from Mocha to Playwright ([#49](https://github.com/mobile-next/devicekit-ios/pull/49))

## [0.0.18](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.18) (2026-05-04)
* Fix: Prevent XCTest from resetting shouldHaltWhenReceivesControl back to YES on setup

## [0.0.17](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.17) (2026-05-03)
* iOS: Include placeholderValue in source tree element JSON format
* Fix: Prevent test runner from halting on XCTest internal failures
* General: Improve README copy and add GitHub issue templates

## [0.0.16](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.16) (2026-04-16)
* General: Set app version in Info.plist from git tag at build time

## [0.0.13](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.13) (2026-04-15)
* CI: Parallelize IPA and simulator zip builds
* CI: Fail Trivy scan on HIGH/CRITICAL findings

## [0.0.12](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.12) (2026-04-15)
* General: Bump deployment target from iOS 14 to iOS 16 for smaller swift frameworks overhead
* General: Only package XCUITest runner in the .ipa
* Fix: Prevent outputPath override via CodingKeys, make JSONRPCResponse Encodable only
* CI: Add build provenance attestations for release artifacts
* CI: Remove unnecessary brew install for xcbeautify

## [0.0.10](https://github.com/mobile-next/devicekit-ios/releases/tag/0.0.10) (2026-04-12)
* General: Initial public release of DeviceKit iOS
* General: JSON-RPC 2.0 server over HTTP and WebSocket
* General: Health check and graceful shutdown endpoints
* General: Add MJPEG streaming endpoint tests and test infrastructure
* iOS: Tap, swipe, long press, and multi-finger gesture synthesis
* iOS: Text input via system keyboard
* iOS: Hardware button simulation (home, lock, volume)
* iOS: App launch, terminate, and foreground detection
* iOS: Full accessibility tree inspection (UI hierarchy dump)
* iOS: Screenshot capture (PNG/JPEG with configurable quality)
* iOS: Real-time MJPEG screen streaming with configurable fps, quality, and scale
* iOS: Real-time H264 screen streaming with configurable fps, bitrate, quality, and scale
* iOS: ReplayKit broadcast extension with H264 video and Opus audio
* iOS: Device orientation get/set
* iOS: URL opening
* iOS: Device info (screen size, scale)
