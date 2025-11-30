## [0.0.42](https://github.com/mobile-next/mobilecli/releases/tag/0.0.42) (2025-11-27)

* iOS: Fix mjpeg framerate wda race condition ([#119](https://github.com/mobile-next/mobilecli/pull/119))
* iOS: Fix multiple real devices would override each other's pointers ([#120](https://github.com/mobile-next/mobilecli/pull/120))

## [0.0.41](https://github.com/mobile-next/mobilecli/releases/tag/0.0.41) (2025-11-26)

* General: Cache preflight cors check for a day ([#114](https://github.com/mobile-next/mobilecli/pull/114))
* General: Fix wrong error message when simulator/emulator device is offline ([#107](https://github.com/mobile-next/mobilecli/pull/107))
* iOS: Optimize "simctl list" performance by eliminating it ([#115](https://github.com/mobile-next/mobilecli/pull/115))
* iOS: Add wda session caching for faster input operations ([#111](https://github.com/mobile-next/mobilecli/pull/111))
* iOS: Support scale parameter for mjpeg ([#110](https://github.com/mobile-next/mobilecli/pull/110))
* iOS: Add progress notifications on wda installation ([#109](https://github.com/mobile-next/mobilecli/pull/109))
* Android: Improve "device list" performance by removing dependency on avdmanager ([#116](https://github.com/mobile-next/mobilecli/pull/116))
* Android: Fix screenshot from multi-display android (like Pro Fold 9) ([#113](https://github.com/mobile-next/mobilecli/pull/113))
* Android: Make sure ANDROID_HOME path is valid ([#108](https://github.com/mobile-next/mobilecli/pull/108))
* Android: Faster emulator boot (removed -no-snapshot-load) and without window ([#117](https://github.com/mobile-next/mobilecli/pull/117))

## [0.0.40](https://github.com/mobile-next/mobilecli/releases/tag/0.0.40) (2025-11-20)

* iOS: Start tunnel before apps launch and terminate on iOS 17+ ([#106](https://github.com/mobile-next/mobilecli/pull/106))

## [0.0.39](https://github.com/mobile-next/mobilecli/releases/tag/0.0.39) (2025-11-20)

* General: Bump glob from 10.4.5 to 10.5.0 in /test ([#105](https://github.com/mobile-next/mobilecli/pull/105))
* General: Bump golang.org/x/crypto from 0.41.0 to 0.45.0 ([#104](https://github.com/mobile-next/mobilecli/pull/104))
* General: Bump js-yaml from 4.1.0 to 4.1.1 in /test ([042098c](https://github.com/mobile-next/mobilecli/commit/042098c))
* General: Upgrade golang to 1.25.0 ([#102](https://github.com/mobile-next/mobilecli/pull/102))
* General: Update go-quic to 0.49.1 ([5510c06](https://github.com/mobile-next/mobilecli/commit/5510c06))

## [0.0.38](https://github.com/mobile-next/mobilecli/releases/tag/0.0.38) (2025-11-17)

* Android: Fix broken adb access to emulator-%d ([#100](https://github.com/mobile-next/mobilecli/pull/100))

## [0.0.37](https://github.com/mobile-next/mobilecli/releases/tag/0.0.37) (2025-11-15)

* General: New and more detailed --help ([#96](https://github.com/mobile-next/mobilecli/pull/96))
* General: Make device_reboot available through jsonrpc ([#97](https://github.com/mobile-next/mobilecli/pull/97))
* Android: Translate Android API levels to Android release versions in device list ([#98](https://github.com/mobile-next/mobilecli/pull/98))

## [0.0.36](https://github.com/mobile-next/mobilecli/releases/tag/0.0.36) (2025-11-14)

* General: Progammatically boot and shutdown emulators and simulators ([#95](https://github.com/mobile-next/mobilecli/pull/95))
* General: Offline devices need to be booted manually prior to use ([#95](https://github.com/mobile-next/mobilecli/pull/95))

## [0.0.35](https://github.com/mobile-next/mobilecli/releases/tag/0.0.35) (2025-11-13)

* General: Support "type", "platform" and "includeOffline" in jsonrpc for 'devices' command ([#94](https://github.com/mobile-next/mobilecli/pull/94))
* General: Changed "devices --all" to "devices --include-offline" for consistency and clarity ([#94](https://github.com/mobile-next/mobilecli/pull/94))

## [0.0.34](https://github.com/mobile-next/mobilecli/releases/tag/0.0.34) (2025-11-13)

* General: Support --type and --platform in list devices command ([#81](https://github.com/mobile-next/mobilecli/pull/81))
* iOS: List simulators that have been initialized once (they boot fast) in "devices --all" ([#81](https://github.com/mobile-next/mobilecli/pull/81))
* iOS: Add an icon and a display name for installed webdriveragent ([#93](https://github.com/mobile-next/mobilecli/pull/93))
* Android: List offline emulators in "devices --all" ([#81](https://github.com/mobile-next/mobilecli/pull/81))

## [0.0.33](https://github.com/mobile-next/mobilecli/releases/tag/0.0.33) (2025-11-01)

* General: Fix launching mobilecli using npx on macos after split to arm64/amd64 ([#91](https://github.com/mobile-next/mobilecli/pull/91))

## [0.0.32](https://github.com/mobile-next/mobilecli/releases/tag/0.0.32) (2025-11-01)

* General: Use USERPROFILE and LOCALAPPDATA on Windows to locate adb.exe without relying on PATH ([#88](https://github.com/mobile-next/mobilecli/pull/88))

## [0.0.31](https://github.com/mobile-next/mobilecli/releases/tag/0.0.31) (2025-10-23)

* Android: CRLF was getting in the way of mjpeg on Windows ([#86](https://github.com/mobile-next/mobilecli/pull/86))

## [0.0.30](https://github.com/mobile-next/mobilecli/releases/tag/0.0.30) (2025-10-23)

* General: Do not attempt fetching iOS simulators if not running on macos ([#85](https://github.com/mobile-next/mobilecli/pull/85))
* iOS: Skip tunnel creation if running on iOS 16.x and below ([#84](https://github.com/mobile-next/mobilecli/pull/84))
* Android: CRLF was getting in the way of screenshots on Windows ([#85](https://github.com/mobile-next/mobilecli/pull/85))

## [0.0.29](https://github.com/mobile-next/mobilecli/releases/tag/0.0.29) (2025-10-21)

* General: Split mac binaries into arm64 and amd64 for smaller packages ([06d7e89](https://github.com/mobile-next/mobilecli/commit/06d7e89e5cb94848ed0a12f74c80726b81c15947))

## [0.0.28](https://github.com/mobile-next/mobilecli/releases/tag/0.0.28) (2025-10-07)

* General: Statically linked binaries for linux ([#79](https://github.com/mobile-next/mobilecli/pull/79))

## [0.0.27](https://github.com/mobile-next/mobilecli/releases/tag/0.0.27) (2025-10-05)

* General: Fix potential zip-split security issue when unpacking wda ([#76](https://github.com/mobile-next/mobilecli/pull/76))
* iOS: Set MJPEG server to stream at 30 fps instead of default 10 ([#78](https://github.com/mobile-next/mobilecli/pull/78))
* iOS: Fix buggy 'device info' on an iOS device when wda wasn't running prior ([#77](https://github.com/mobile-next/mobilecli/pull/77))

## [0.0.26](https://github.com/mobile-next/mobilecli/releases/tag/0.0.26) (2025-10-02)

* iOS: Added swipe command, use "mobilecli io swipe" ([#72](https://github.com/mobile-next/mobilecli/pull/72))
* Simulator: Fixed 'mobilecli device info', it requires wda prior ([#73](https://github.com/mobile-next/mobilecli/pull/73))
* Android: Added swipe command, use "mobilecli io swipe" ([#72](https://github.com/mobile-next/mobilecli/pull/72))

## [0.0.25](https://github.com/mobile-next/mobilecli/releases/tag/0.0.25) (2025-09-26)

* General: fixed '--version' on windows and linux distributables ([#69](https://github.com/mobile-next/mobilecli/pull/69))
* General: renamed 'dump source' to 'dump ui' ([#68](https://github.com/mobile-next/mobilecli/pull/68))
* General: upgraded golang to 1.24.7 ([#66](https://github.com/mobile-next/mobilecli/pull/66))
* iOS: support orientation get and set to 'landscape' and 'portrait' ([#67](https://github.com/mobile-next/mobilecli/pull/67))
* iOS: removed a printf in 'dump ui' that was tainting the json output ([#68](https://github.com/mobile-next/mobilecli/pull/68))
* Android: support orientation get and set to 'landscape' and 'portrait' ([#67](https://github.com/mobile-next/mobilecli/pull/67))

## [0.0.24](https://github.com/mobile-next/mobilecli/releases/tag/0.0.24) (2025-09-23)

* iOS: support longpress ([#54](https://github.com/mobile-next/mobilecli/pull/54))
* iOS: dump ui hierarchy using wda's /source ([#53](https://github.com/mobile-next/mobilecli/pull/53))
* iOS: launch wda on different ports, to enable multiple simulators and real devices on the same host ([#52](https://github.com/mobile-next/mobilecli/pull/52))
* Android: support longpress ([#54](https://github.com/mobile-next/mobilecli/pull/54))
* Android: dump ui hierarchy using uiautomator ([#53](https://github.com/mobile-next/mobilecli/pull/53))

## [0.0.23](https://github.com/mobile-next/mobilecli/releases/tag/0.0.23) (2025-09-18)

* General: all logs moved to --verbose ([#48](https://github.com/mobile-next/mobilecli/pull/48))
* iOS: fixed "ENTER" io button command ([#50](https://github.com/mobile-next/mobilecli/pull/50))
* iOS: added version to device list response, for both real devices and simulators ([#51](https://github.com/mobile-next/mobilecli/pull/51))
* Android: added OS version to device list response (eg "16") ([#51](https://github.com/mobile-next/mobilecli/pull/51))

## [0.0.22](https://github.com/mobile-next/mobilecli/releases/tag/0.0.22) (2025-09-10)

* iOS: launch wda on real devices if needed ([#45](https://github.com/mobile-next/mobilecli/pull/45))
* iOS: automatically detect wda installed device, regardless on bundle identifier ([#45](https://github.com/mobile-next/mobilecli/pull/45))

## [0.0.21](https://github.com/mobile-next/mobilecli/releases/tag/0.0.21) (2025-09-08)

* Android: return the avd name if emulator supports (eg 'Pixel 5' instead of 'sdk_gphone64_arm64') ([#42](https://github.com/mobile-next/mobilecli/pull/42))
* Android: support for POWER and APP_SWITCH buttons ([#44](https://github.com/mobile-next/mobilecli/pull/44))

## [0.0.20](https://github.com/mobile-next/mobilecli/releases/tag/0.0.20) (2025-09-04)

* iOS: fixed potential race-condition in waiting for wda installation on simulator ([#41](https://github.com/mobile-next/mobilecli/pull/41))
* Android: try $HOME/Library/Android if $ANDROID_HOME is not configured ([#40](https://github.com/mobile-next/mobilecli/pull/40))
* Android: support --scale and --quality for `screencapture` mjpeg streaming ([#39](https://github.com/mobile-next/mobilecli/pull/39))

## [0.0.19](https://github.com/mobile-next/mobilecli/releases/tag/0.0.19) (2025-09-03)

* General: run tests on iOS 16, 17 and 18 simulators upon pull-request ([#35](https://github.com/mobile-next/mobilecli/pull/35))
* General: upgraded go-quic libraries for security ([5d35293](https://github.com/mobile-next/mobilecli/commit/5d35293d6bd4164c9354b365129c7ae46ceb60a7#diff-33ef32bf6c23acb95f5902d7097b7a1d5128ca061167ec0716715b0b9eeaa5f6R12))
* iOS: embedded go-ios as a library, go-ios is no longer required to be installed before ([#34](https://github.com/mobile-next/mobilecli/pull/34))

## [0.0.18](https://github.com/mobile-next/mobilecli/releases/tag/0.0.18) (2025-08-26)

* iOS: proper handling and forwarding of port 9100 ([#33](https://github.com/mobile-next/mobilecli/pull/33))
* Simulator: use localhost:9100 properly for mjpeg streaming ([#33](https://github.com/mobile-next/mobilecli/pull/33))

## [0.0.17](https://github.com/mobile-next/mobilecli/releases/tag/0.0.17) (2025-08-25)

* iOS: fix locating go-ios executable when creating tunnel ([#32](https://github.com/mobile-next/mobilecli/pull/32))

## [0.0.16](https://github.com/mobile-next/mobilecli/releases/tag/0.0.16) (2025-08-25)

* General: fix zipslip security warning ([c21e7f0](https://github.com/mobile-next/mobilecli/commit/c21e7f0d8ad22eac583ef166a5a4b836e908cf12))
* General: version command showed 'dev' after refactor of cli package ([0bb104f](https://github.com/mobile-next/mobilecli/commit/0bb104f7f078e672bd27c0455274cd2d46066827))
* iOS: check for go-ios path through GO_IOS_PATH env ([1465914](https://github.com/mobile-next/mobilecli/commit/14659146758931d6531f95b603b48fd15fe07ed0))

## [0.0.15](https://github.com/mobile-next/mobilecli/releases/tag/0.0.15) (2025-08-25)

* General: refactored all commands into "cli" package ([8115a787](https://github.com/mobile-next/mobilecli/commit/8115a7873b62b3b66a79680c3b95a3db792fa5fb))
* General: reimplemented all tests into golang, added coverage tests ([4390b11](https://github.com/mobile-next/mobilecli/commit/4390b11b11ac657ee7694298fe0902687e61d0fc))
* General: better error responses for jsonrpc protocol ([b7ca4a8](https://github.com/mobile-next/mobilecli/commit/b7ca418c8b8e31c5c2776a231bfcdae6dbed3b4c))
* iOS: automatically start webdriver agent if installed on device ([02025dd](https://github.com/mobile-next/mobilecli/commit/02025ddd13581edcbf4f932ac46dcc5e33a6e2ec))
* iOS: automatically start port forwarding for port 8100 using a dynamic source port ([02025dd](https://github.com/mobile-next/mobilecli/commit/02025ddd13581edcbf4f932ac46dcc5e33a6e2ec))
* iOS: automatically start userland tunnel for iOS 17 ([02025dd](https://github.com/mobile-next/mobilecli/commit/02025ddd13581edcbf4f932ac46dcc5e33a6e2ec))
* iOS: fixed bad parsing of go-ios output if warning was printed ([2150f27](https://github.com/mobile-next/mobilecli/commit/2150f279bae927c2a19f2558bb81afcc1df03b54))
* iOS: use "ios" and fallback to "go-ios" ([2150f27](https://github.com/mobile-next/mobilecli/commit/2150f279bae927c2a19f2558bb81afcc1df03b54))
* iOS: support multiple custom gestures to be passed to device ([b7ca418](https://github.com/mobile-next/mobilecli/commit/b7ca418c8b8e31c5c2776a231bfcdae6dbed3b4c))
* Simulator: automatically donwload webdriveragent for simulator ([0dbe361](https://github.com/mobile-next/mobilecli/commit/0dbe3612ef5758523028433f1e168ddac98544e0))
* Simulator: automatically install webdriveragent on simulator if needed ([0dbe361](https://github.com/mobile-next/mobilecli/commit/0dbe3612ef5758523028433f1e168ddac98544e0))


