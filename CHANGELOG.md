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


