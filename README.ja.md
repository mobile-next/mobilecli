# mobilecli

[English](README.md) | **日本語** | [简体中文](README.zh-CN.md)

[Mobile Next](https://github.com/mobile-next/) が提供する、iOS / Android のデバイス、シミュレーター、エミュレーター、アプリを管理するためのユニバーサルなコマンドラインツールです。

<p align="left">
  <a href="https://github.com/mobile-next/mobilecli">
    <img src="https://img.shields.io/github/stars/mobile-next/mobilecli" alt="Mobile Next Stars" />
  </a>
  <a href="https://www.npmjs.com/package/mobilecli">
    <img src="https://img.shields.io/npm/dm/mobilecli?logo=npm&style=flat&color=red" alt="npm">
  </a>
  <a href="https://github.com/mobile-next/mobilecli/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/license-FSL--1.1--Apache--2.0-blue.svg" alt="mobilecli is released under the FSL-1.1-Apache-2.0 License">
  </a>
  <a href="http://mobilenext.ai/join-slack">
      <img src="https://img.shields.io/badge/join-Slack-blueviolet?logo=slack&style=flat" alt="Slack community channel" />
  </a>
</p>

## 機能 🚀

- **デバイス管理**: 接続されたモバイルデバイスの一覧表示、管理、操作
- **クロスプラットフォーム対応**: iOS 実機、iOS シミュレーター、Android 実機、Android エミュレーターで動作
- **エミュレーター / シミュレーター制御**: エミュレーターとシミュレーターの起動・終了をプログラムから実行
- **スクリーンショット取得**: 接続された任意のデバイスからフォーマットを指定してスクリーンショットを取得
- **複数の出力形式**: 品質を指定して PNG または JPEG でスクリーンショットを保存
- **画面キャプチャの動画ストリーミング**: デバイスから mjpeg / h264 の動画を直接ストリーミング
- **デバイス制御**: デバイスの再起動、画面座標のタップ、ハードウェアボタンの押下
- **アプリ管理**: アプリの起動、終了、インストール、アンインストール、データ消去、一覧表示、フォアグラウンドアプリの取得
- **ファイルシステム**: デバイス上またはアプリコンテナ内でのファイルの push、pull、一覧表示、mkdir、rm（Android、iOS シミュレーター）
- **位置情報の上書き**: デバイスが報告する GPS 位置情報を偽装
- **クラッシュレポート**: iOS / Android デバイスからクラッシュレポートを一覧表示・取得
- **デバイスログ**: iOS / Android デバイスのログをフィルタリングしながらリアルタイムにストリーミング
- **WebView の検査**: 埋め込み WebView の一覧表示、ページ遷移、DOM クエリ、JavaScript の実行

### 🎯 プラットフォーム対応状況

| プラットフォーム | 対応 |
|----------|:---------:|
| iOS 実機 | ✅ |
| iOS シミュレーター | ✅ |
| Android 実機 | ✅ |
| Android エミュレーター | ✅ |

## インストール 📦

#### 前提条件 📋
- **Android SDK**（`adb` が PATH に含まれていること。Android デバイス対応に必要）
- **Xcode Command Line Tools**（macOS での iOS シミュレーター対応に必要）

#### npm でグローバルにインストール
```bash
npm install -g mobilecli@latest
```

### エージェントのセットアップ 🤖

`mobilecli` をインストールし、コーディングエージェントが使い方を理解できるようにスキルを追加します:

```bash
npm install -g mobilecli@latest
npx skills add https://github.com/mobile-next/mobilecli
```

## CLI リファレンス

### 接続されたデバイスの一覧表示 🔍

```bash
# List all online devices and simulators
mobilecli devices

# List all devices including offline emulators and simulators
mobilecli devices --include-offline
```

**注意**: オフラインのエミュレーターやシミュレーターは `mobilecli device boot` コマンドで起動できます。

### スクリーンショットの取得 📸

```bash
# Take a PNG screenshot (default)
mobilecli screenshot --device <device-id>

# Take a JPEG screenshot with custom quality
mobilecli screenshot --device <device-id> --format jpeg --quality 80

# Scale the screenshot down to half size
mobilecli screenshot --device <device-id> --scale 0.5

# Limit the largest dimension to 800 pixels, keeping aspect ratio
mobilecli screenshot --device <device-id> --max-size 800

# Crop to a region (x,y,width,height in screen points), applied before scaling
mobilecli screenshot --device <device-id> --clip 10,80,300,200

# Save to specific path
mobilecli screenshot --device <device-id> --output screenshot.png

# Output to stdout
mobilecli screenshot --device <device-id> --output -
```

### 画面のストリーミング 🎥

```bash
mobilecli screencapture --device <device-id> --format mjpeg | ffplay -
```

screencapture は一方向であることに注意してください。画面をタップするには `io tap` コマンドを使う必要があります。

### デバイス制御 🎮

```bash
# Boot an offline emulator or simulator
mobilecli device boot --device <device-id>

# Shutdown a running emulator or simulator
mobilecli device shutdown --device <device-id>

# Reboot a device
mobilecli device reboot --device <device-id>

# Tap at coordinates (x,y)
mobilecli io tap --device <device-id> 100,200

# Long press at coordinates (x,y) with optional duration in milliseconds
mobilecli io longpress --device <device-id> 100,200
mobilecli io longpress --device <device-id> 100,200 --duration 2000

# Press hardware buttons
mobilecli io button --device <device-id> HOME
mobilecli io button --device <device-id> VOLUME_UP
mobilecli io button --device <device-id> POWER

# Send text
mobilecli io text --device <device-id> 'hello world'

# Read device clipboard
mobilecli io clipboard get --device <device-id>

# Write device clipboard
mobilecli io clipboard set --device <device-id> 'hello world'

# Send keys combination to paste (cmd+v for iOS, ctrl+v for Android)
mobilecli io keys --device <device-id> "cmd+v"
```

### 位置情報の上書き 📍

```bash
# Fake the device location (latitude,longitude)
mobilecli device location set --device <device-id> 37.7749,-122.4194

# Hold the override until Ctrl-C, then clear it automatically
mobilecli device location set --device <device-id> 37.7749,-122.4194 --wait

# Restore the real location
mobilecli device location clear --device <device-id>
```

プラットフォームごとの注意点:

| プラットフォーム | 備考 |
|----------|-------|
| iOS シミュレーター | そのまま動作します。上書きは mobilecli の終了後も維持されます |
| iOS 実機 | 上書きは、それを設定した mobilecli プロセスが生きている間だけ有効なため、`--wait` が必須です。`clear` も同じプロセスから実行する必要があります |
| Android エミュレーター | エミュレーターコンソールを使用します。位置の固定を取り消す手段がないため、`clear` は実際の位置ではなく、エミュレーター起動時の座標（Googleplex）に位置情報を戻します |
| Android 実機 | デバイス上のエージェントをモック位置情報プロバイダーとして実行し、`com.android.shell` に `mock_location` appop を付与します。一部の OEM ROM はモックプロバイダーを無視し、`Location.isFromMockProvider()` や Play Integrity をチェックするアプリには検知されます |

### 対応ハードウェアボタン

- `HOME` - ホームボタン
- `BACK` - 戻るボタン（Android のみ）
- `POWER` - 電源ボタン
- `VOLUME_UP`, `VOLUME_DOWN` - 音量の上げ下げ
- `DPAD_UP`, `DPAD_DOWN`, `DPAD_LEFT`, `DPAD_RIGHT`, `DPAD_CENTER` - 十字キー操作（Android のみ）

### アプリ管理 📱

```bash
# List installed apps on device
mobilecli apps list --device <device-id>

# Get currently foreground app
mobilecli apps foreground --device <device-id>

# Launch an app
mobilecli apps launch <bundle-id> --device <device-id>

# Terminate an app
mobilecli apps terminate <bundle-id> --device <device-id>

# Install an app (.apk for Android, .ipa for iOS, .zip for iOS Simulator)
mobilecli apps install <path> --device <device-id>

# Uninstall an app
mobilecli apps uninstall <bundle-id> --device <device-id>

# Clear app data (cache, preferences, databases) without uninstalling
# Supported on Android and iOS Simulator
mobilecli apps clear <bundle-id> --device <device-id>
```

### ファイルシステム 📂

デバイス上、またはアプリのデータコンテナ内のファイルにアクセスします。現在は **Android** と **iOS シミュレーター** に対応しています。

```bash
# Get the data container path of an app (Android)
mobilecli apps path <bundle-id> --device <device-id>

# List files at any absolute path (defaults to device root if omitted)
mobilecli fs ls --device <device-id>
mobilecli fs ls --device <device-id> /sdcard
mobilecli fs ls --device <device-id> /sdcard/Download

# List files inside an app's data container
mobilecli fs ls --device <device-id> com.example.app
mobilecli fs ls --device <device-id> com.example.app /Documents

# Pull a file from the device to local disk
mobilecli fs pull --device <device-id> /sdcard/recording.mp4 ./recording.mp4

# Pull a file from an app's private container
mobilecli fs pull --device <device-id> /data/user/0/com.example.app/files/db.sqlite ./db.sqlite

# Push a file to the device
mobilecli fs push --device <device-id> ./config.json /sdcard/config.json

# Push a file into an app's private container
mobilecli fs push --device <device-id> ./config.json /data/user/0/com.example.app/files/config.json

# Create a directory
mobilecli fs mkdir --device <device-id> /sdcard/myfolder

# Create a directory and all parent directories
mobilecli fs mkdir --device <device-id> -p /sdcard/a/b/c
mobilecli fs mkdir --device <device-id> -p /data/user/0/com.example.app/files/cache/v2

# Remove a file
mobilecli fs rm --device <device-id> /sdcard/old_file.txt

# Remove a directory recursively
mobilecli fs rm --device <device-id> -r /sdcard/myfolder
mobilecli fs rm --device <device-id> -r /data/user/0/com.example.app/files/cache
```

**注意:**
- `/data/user/` 配下のパスは `run-as` 経由でアクセスするため、アプリがデバッグ可能（debuggable）である必要があります。
- `/data/user/` への push は、ファイルを一旦 `/data/local/tmp/` に置いてからコンテナ内にコピーします。
- バイナリファイル（画像、データベース、DEX ファイル）の pull は、すべてのプラットフォームで完全に対応しており、バイナリセーフです。

### エージェント管理 🤖

**iOS** では、タッチ入力（タップ、スワイプ、ボタン押下）、画面キャプチャのストリーミング、UI ツリーの検査にデバイス上のエージェントが必要です。これらの機能は、デバイス上でエージェントを実行しない限り、標準の iOS ツールでは利用できません。

**Android** ではエージェントは不要です。Android デバイスで `mobilecli agent status` を実行すると、その旨が報告されるだけです。

```bash
# Check if the agent is installed on a device
mobilecli agent status --device <device-id>

# Install the agent
mobilecli agent install --device <device-id>

# Force reinstall the agent
mobilecli agent install --device <device-id> --force

# Install on a real iOS device (requires provisioning profile)
mobilecli agent install --device <device-id> --provisioning-profile /path/to/profile.mobileprovision
```

`agent status` の出力例:
```json
{
  "status": "ok",
  "data": {
    "message": "Agent version 0.0.12 is installed on device",
    "agent": {
      "version": "0.0.12",
      "bundleId": "com.mobilenext.devicekit-iosUITests.xctrunner"
    }
  }
}
```

### WebView の検査 🌐

ネイティブアプリ内で動作する埋め込み WebView（iOS では `WKWebView`、Android では `android.webkit.WebView`）を検査・操作します。

```bash
# List embedded webviews in the foreground app
mobilecli webview list --device <device-id>

# Navigate a webview to a URL
mobilecli webview goto <id> https://example.com --device <device-id>

# Reload, go back or forward
mobilecli webview reload <id> --device <device-id>
mobilecli webview back <id> --device <device-id>
mobilecli webview forward <id> --device <device-id>

# Get current URL and page title
mobilecli webview url <id> --device <device-id>
mobilecli webview title <id> --device <device-id>

# Dump the full HTML content of the page
mobilecli webview content <id> --device <device-id>

# Query DOM elements by CSS selector
mobilecli webview query <id> "button" --device <device-id>
mobilecli webview query <id> "[data-testid='submit']" --device <device-id>

# Evaluate arbitrary JavaScript
mobilecli webview eval <id> "document.querySelectorAll('a').length" --device <device-id>

# Wait for the page to finish loading
mobilecli webview wait <id> --state load --device <device-id>
mobilecli webview wait <id> --state domcontentloaded --timeout 5000 --device <device-id>
```

`webview list` の出力例:
```json
{
  "status": "ok",
  "data": [
    {
      "id": "1",
      "url": "https://example.com",
      "title": "Example Domain"
    }
  ]
}
```

`webview query <id> "button"` の出力例:
```json
{
  "status": "ok",
  "data": [
    { "tag": "button", "text": "Sign In", "id": "login-btn", "class": "btn-primary", "value": null, "href": null },
    { "tag": "button", "text": "Cancel", "id": null, "class": "btn-secondary", "value": null, "href": null }
  ]
}
```

### クラッシュレポート 💥

```bash
# List crash reports from a device
mobilecli device crashes list --device <device-id>

# Get a specific crash report by ID
mobilecli device crashes get <crash-id> --device <device-id>
```

`crashes list` の出力例:
```json
{
  "status": "ok",
  "data": [
    {
      "processName": "ShareExtension",
      "timestamp": "2026-01-24-195529",
      "id": "ShareExtension-2026-01-24-195529.ips"
    }
  ]
}
```

**注意**: iOS 実機では、クラッシュレポートは Apple の crashreport サービス経由で取得します。iOS シミュレーターでは `~/Library/Logs/DiagnosticReports/` から読み取ります。Android では `adb logcat -b crash` の出力を解析します。

### デバイスログ 📋

```bash
# Stream logs from a device (Ctrl+C to stop)
mobilecli device logs --device <device-id>

# Stop after 100 entries
mobilecli device logs --device <device-id> --limit 100

# Filter by field (exact match)
mobilecli device logs --filter process=SpringBoard
mobilecli device logs --filter tag=ActivityManager

# Exclude by field
mobilecli device logs --filter process!=SpringBoard

# Combine filters (AND logic)
mobilecli device logs --filter level=Error --filter process!=SpringBoard
```

対応しているフィルターキー: `pid`, `process`, `tag`, `level`, `subsystem`, `category`, `message`

各ログエントリは 1 行の JSON として出力されます:
```json
{"timestamp":"2026-04-15 12:17:14.224451+0300","message":"Start proc...","level":"Default","subsystem":"com.apple.UIKit","category":"EventDispatch","pid":54052,"process":"SpringBoard"}
```

ログは [HTTP API](#http-api-) の `device.logs` メソッドからも利用できます。同じ `limit` と `filters` を受け取り、ストリーミング用の URL を返します:

```bash
curl -X POST http://localhost:12000/rpc -H "Content-Type: application/json" -d '{
  "jsonrpc": "2.0", "method": "device.logs", "id": 1,
  "params": { "deviceId": "<device-id>", "limit": 100, "filters": ["level=Error", "process!=SpringBoard"] }
}'
# => {"jsonrpc":"2.0","id":1,"result":{"sessionUrl":"/sessions/<session-id>/logs"}}

curl -N http://localhost:12000/sessions/<session-id>/logs
```

セッションは作成から 1 分で期限切れになり、接続は 1 つだけ受け付けます。切断するとデバイス上のログストリームは停止します。

### リモートデバイス ☁️

```bash
# Allocate a remote iOS device
mobilecli remote allocate --platform ios --version ">=18" --name "iPhone*" --wait

# Allocate a remote Android device (exact OS version)
mobilecli remote allocate --platform android --version 14 --wait

# List available remote devices
mobilecli remote list-devices

# Release an allocated remote device
mobilecli remote release --device <device-id>
```

## Claude Code スキル 🤖

このリポジトリには、Claude Code（または SKILL.md 互換の任意のエージェント）に `mobilecli` の使い方（デバイスの一覧表示、タップと文字入力、UI ツリーのダンプ、アプリ管理、高速な自動化のための JSON-RPC サーバーの利用）を教えるエージェントスキル（[skills/mobilecli/SKILL.md](skills/mobilecli/SKILL.md)）が含まれています。

[skills](https://github.com/vercel-labs/skills) でインストールします:

```bash
# current project only
npx skills add mobile-next/mobilecli

# or globally, for all projects
npx skills add mobile-next/mobilecli -g
```

その後、エージェントに「エミュレーターのスクリーンショットを撮って」や「ログインボタンをタップして」のように頼むと、スキルが自動的に起動します。

## デーモン ⚙️

すべてのデバイスコマンドは、ユーザーごとのバックグラウンドデーモンと通信します。デーモンは検出済みのデバイス、iOS トンネル、デバイス上のエージェントを呼び出し間で維持するため、検出のコストがかかるのは最初のコマンドだけです。デーモンは最初のデバイスコマンドで自動的に起動し（`mobilecli --help` と `--version` では起動しません）、リクエストがないまま 30 分経過すると終了します。

```bash
# Inspect or control it explicitly
mobilecli daemon status
mobilecli daemon stop

# Run it in the foreground (e.g. under a service manager), with a custom idle timeout
mobilecli daemon start --idle-timeout 0
```

CLI とデーモンは `~/.mobilecli/` 内の unix ドメインソケット経由で JSON-RPC 通信を行います（`MOBILECLI_HOME` で変更可能）。デーモンのログは `~/.mobilecli/daemon.log` です。古いバージョンの mobilecli が残したデーモンは自動的に再起動されます。`auth login` と `auth logout` は、新しい認証情報を反映させるために、実行中のデーモンを停止します。

## HTTP API 🔌

***mobilecli*** は、コマンドラインで利用できるすべての機能を HTTP インターフェースとしても提供します。このサーバーは上記デーモンの薄いフロントエンドであるため、HTTP クライアントと CLI は同じデバイスとトンネルを共有します。

利用可能な JSON-RPC メソッドとそのパラメーターの一覧は、[OpenRPC 仕様](https://github.com/mobile-next/mobile-openrpc/blob/main/mobilecli/openrpc.md)を参照してください。

```bash
# Start the server (default port 12000)
mobile server start

curl http://localhost:12000/rpc -XPOST -d '{"jsonrpc":"2.0", "id": 1, "method": "devices", "params": {}}'
curl http://localhost:12000/rpc -XPOST -d '{"jsonrpc":"2.0", "id": 1, "method": "screenshot", "params": {"deviceId": "your-device-id"}}'
```

## WebSocket 対応 🔌

***mobilecli*** には WebSocket サーバーが含まれており、HTTP API と同じ JSON-RPC 2.0 形式で、単一の接続上で複数のリクエストを送ることができます。

```bash
# Start the server (default port 12000)
mobilecli server start

# Connect and send requests using wscat
wscat -c ws://localhost:12000/ws
> {"jsonrpc":"2.0","id":1,"method":"devices","params":{}}
< {"jsonrpc":"2.0","id":1,"result":[...]}
> {"jsonrpc":"2.0","id":2,"method":"screenshot","params":{"deviceId":"your-device-id"}}
< {"jsonrpc":"2.0","id":2,"result":{...}}
```

**注意**: `screencapture` は WebSocket では対応していません。動画ストリーミングには HTTP の `/rpc` エンドポイントを使用してください。

## プラットフォーム固有の注意点

### iOS 実機
- デバイス上のエージェントが必要です。`mobilecli agent install --device <device-id> --provisioning-profile /path/to/profile.mobileprovision` でインストールしてください。お使いのデバイス向けにエージェントを再署名するため、有効な Apple のプロビジョニングプロファイルが必要です。

## 開発 👩‍💻

### ビルド 🛠️

*mobilecli* をローカルでテストする方法の詳細は (docs/TESTING.md) を参照してください。

```bash
make lint
make build
make test
```

## サポート 💬

不具合の報告や機能リクエストは [GitHub Issues](https://github.com/mobile-next/mobilecli/issues) ページをご利用ください。

ぜひ今すぐ <a href="http://mobilenext.ai/join-slack">Slack チャンネルに参加</a>してください 💜

<a href="https://mobilenext.ai/">Mobile Next</a> と私たちが作っているものについて、より詳しく知ることができます。
