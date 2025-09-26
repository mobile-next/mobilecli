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


