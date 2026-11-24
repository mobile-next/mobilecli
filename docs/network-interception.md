# Network capture and interception

mobilecli can observe and intercept the HTTP traffic of a **debuggable app you
own** — an Android app with `android:debuggable="true"`, or an iOS app built with
the `get-task-allow` entitlement (any Debug build). This is the backend for a
Playwright-style `route()` in mobilewright: the host gets a callback when the app
makes a request and decides whether to let it through, change it, fake a
response, or fail it.

It reuses the injected-agent channel that already drives webviews: a JVMTI `.so`
on Android, a dylib on iOS. On iOS the dylib is mapped into the app by the
debugger-assisted Mach-O mapper (`devices/ios/machomap`) over the developer
`debugserver` reached through the go-ios userspace tunnel — no LLDB and no
re-signing, so it runs on real `get-task-allow` devices, not just the simulator.
The network methods are added to that same in-app agent; nothing new is injected.
Only debuggable apps can be attached to at all, so this never applies to App Store
or release builds.

## Model

The contract mirrors the Chrome DevTools "Fetch" domain, because Playwright's
`route()` is a thin layer over it. Porting mobilewright becomes a near-direct
mapping.

- The host **enables** interception with a list of routes (URL glob + optional
  method). The agent pauses only requests that match a route, so unmatched
  traffic never makes a round trip to the host.
- The agent emits a **paused** event per matched request, carrying a request id
  unique within the session.
- The host replies with exactly one decision for that id: **continue**
  (optionally with a changed URL, method, headers or body), **fulfill** (a
  synthetic status, headers and body), or **abort** (fail with an error code).
- Capture-only events (**request**, **response**, **finished**, **failed**)
  never pause; they back `page.on('request')` and friends, and later HAR
  recording. (Phase 2.)

### Fail open

This is the rule that keeps the app usable. A paused request continues
**unchanged** whenever a decision cannot be obtained:

- the host stops polling (it disconnected or crashed),
- no decision arrives within the agent's decision timeout (~60s),
- interception is disabled.

Without this, a crashed test script would hang every request the app makes.

## Transport

The host talks to the in-app agent over the existing forwarded RPC socket
(`agentRequest` in `devices/android_rpc.go`). The methods:

| Method              | Params                                                        | Returns                 |
|---------------------|--------------------------------------------------------------|-------------------------|
| `network.enable`    | `{routes: [{url, method?}]}`                                  | `{status:"ok"}`         |
| `network.poll`      | `{}` — long-polls, returns when an event is ready or ~30s     | `{events: [ ... ]}`     |
| `network.continue`  | `{requestId, url?, method?, headers?, body?}`                 | `{status:"ok"}`         |
| `network.fulfill`   | `{requestId, status, headers?, body?}`                       | `{status:"ok"}`         |
| `network.abort`     | `{requestId, errorCode?}`                                     | `{status:"ok"}`         |
| `network.disable`   | `{}`                                                          | `{status:"ok"}`         |

`network.poll` long-polls so events arrive promptly without the host busy-waiting.
The host runs one poll loop per `listen` and relays each event object to the CLI
as a `stream.data` notification. Decisions are separate, idempotent calls keyed
by `requestId`.

Bodies are base64 in both directions. A paused/response body larger than the
agent's cap (256 KiB) is truncated in the event (with `bodyTruncated: true`) but
never truncated in the app itself.

### Event shapes

```jsonc
// matched request, paused, awaiting a decision
{ "type": "paused", "requestId": "r12", "method": "GET",
  "url": "https://api.example.com/user", "headers": {"accept": "*/*"},
  "body": null, "stack": "okhttp" }
```

## CLI

```
# stream paused requests as NDJSON on stdout; decide from another terminal or a coprocess
mobilecli network listen --device <id> --pause '**/api/**' [--pause 'POST **/login']

# decisions, keyed by the requestId from a paused event
mobilecli network continue --device <id> --id r12 [--url ... --method ... --header K=V --body @file]
mobilecli network fulfill  --device <id> --id r12 --status 200 [--header K=V --body @file]
mobilecli network abort    --device <id> --id r12 [--error failed]
```

`--pause` is a glob matched against the full URL. `PREFIX URL` form (`'POST **/x'`)
restricts to a method. Repeat `--pause` for multiple routes.

## Scope / status

- **Phase 1 (this doc):** pause + continue/fulfill/abort on Android (OkHttp) and
  iOS (URLSession), on the iOS simulator and real `get-task-allow` devices.
- **Phase 2:** capture-only events, response-stage interception, HAR recording,
  `HttpURLConnection`, webviews via CDP Fetch.

The in-app hooks live in `agents/android/java/` and `agents/ios/`. The host side
— daemon stream method, CLI, device plumbing — lives in `devices/network.go`,
`commands/network.go`, `cli/network.go`, and `server/daemon_streams.go`.
