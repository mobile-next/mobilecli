# PR Review: #366 — feat: gps location override

**Reviewed**: 2026-08-30
**Author**: gmegidish
**Branch**: feat/location-override → main
**Decision**: COMMENT — 2 HIGH findings to fix before merge, no blockers

## Summary

Location override across iOS simulator/real and Android emulator/real, added through an
optional `LocationSettable` interface so `ControllableDevice` is untouched. Structure,
naming and comments are in good shape; the findings are about failure paths (what state is
left behind when a step fails) and about the json-rpc path skipping the validation the cli
does.

## Findings

### CRITICAL
None.

### HIGH

**H1 — json-rpc can set the device to 0,0, and skips coordinate validation entirely**
`server/server.go` (`LocationSetParams`), `commands/location.go:60` (`LocationSetCommand`)

`latitude`/`longitude` are non-pointer `float64`, so `{"deviceId":"x"}` unmarshals to 0,0 and
silently moves the device to Null Island. `LocationSetCommand` also passes coordinates
straight to `SetLocation`: the range check lives in `ParseLatLon`, which only the cli calls,
so out-of-range and non-finite values reach the platform implementations over rpc.
`ParseLatLon` itself accepts `NaN`, because `NaN < -90` and `NaN > 90` are both false.

Fix: one `validateCoordinates(lat, lon)` rejecting non-finite and out-of-range values,
called from both `ParseLatLon` and `LocationSetCommand`; pointer fields in
`LocationSetParams` to reject omitted coordinates.

**H2 — a failed `SetLocation` leaves the mock_location appop granted on the device**
`devices/android_location.go:71` (`startMockLocation`)

The appop is granted first, then the dex push, launch and readiness poll can each fail and
return without revoking it. Worse, when readiness polling times out but the agent is in fact
running, `SetLocation` reports an error while the device keeps the fake location. Same shape
in `ClearLocation`: if `runDexClass(--clear)` fails, the appop is never revoked.

Fix: clean up (stop agent, revoke appop) on every failure path after the grant, and always
revoke during clear while preserving the original error.

### MEDIUM

**M1 — iOS 17+ override can get stuck with no way to clear it**
`devices/ios_location.go:56`

`ClearLocation` uses `LoadAndDelete` before calling `StopSimulateLocation`. In go-ios
v1.0.211 `StopSimulateLocation` returns on a `MethodCall` error before it reaches `Close`, so
a failed stop drops the handle and every later clear answers "no location override is held by
this process" while the device is still simulating. Related: once a handle goes stale (device
unplugged), every later `SetLocation` reuses it and fails with no recovery.

Fix: delete only after a successful stop; on a failed `StartSimulateLocation` through a
cached handle, drop it and build a fresh service once.

**M2 — the held connection should be a field on IOSDevice, not a package-level map**
`devices/ios_location.go:18`

`locationSimulationServices sync.Map` is a second mechanism for something the codebase
already has one for: `tunnelManager`, `deviceKitClient` and the port forwarders are all
per-device fields guarded by `d.mu`, and `commands.FindDevice` caches device instances by id,
so the same `*IOSDevice` comes back for a udid. Moving it to a field under `d.mu` removes the
global and, as a side effect, serializes create/start/stop so two concurrent `SetLocation`
calls can no longer both build a service and leak one.

**M3 — the on-device dex path is defined twice**
`devices/android_location.go:14` vs `devices/android.go:1006`

`mockLocationDEX` and the local `tmpDEX` in `runDexClass` are the same
`/data/local/tmp/mobilecli.dex`. One package-level constant, used by both.

### LOW

- **L1** `devices/android_location.go:47` — `strings.Contains(string(out), "KO")` matches
  anywhere in the output; the console prefixes failures with `KO:`, so check the prefix.
- **L2** `devices/android_location.go:96` — the local variable is named `log`, which reads as
  the logrus package used elsewhere in this package. `agentLog` says what it holds.
- **L3** `devices/android_location.go:33` — `SetLocation` mixes two abstraction levels: the
  emulator console call inline, the real-device path behind a helper. Extract
  `setEmulatorLocation` so the body is two branches of the same size.
- **L4** `commands/location.go:29` — `ParseLatLon` is exported from `commands` but only the
  cli calls it; the rpc layer takes numbers. It belongs in `cli` unexported, with its test.
- **L5** `cli/location.go:47` — with `--wait` and no `--device`, auto-select runs again at
  Ctrl-C. It cannot clear the wrong device (auto-select refuses when more than one device is
  online) but it can fail to clear if another device came online while holding.
- **L6** `devices/android_location.go:92` — readiness polls `adb shell cat` every 200ms for up
  to 10s, so a failing start costs 50 adb round trips.

## Validation Results

| Check | Result |
|---|---|
| Build (`go build ./...`) | Pass |
| Vet (`go vet ./...`) | Pass |
| Tests (`go test ./...`) | Pass |
| Lint (golangci-lint) | Pass (advisory in CI; 3 new advisory hits, 2 errcheck false positives on `return x.Method()` and one `goconst` on "emulator") |
| CI checks | Pass (e2e_test pending) |

## Clean Code pass

Naming is intention-revealing, functions are all under 30 lines, files are small and cohesive,
comments explain why rather than what, no dead code, no commented-out blocks, no magic numbers
(emulator defaults are named constants), tests read as English sentences. The clean-code
findings are L2, L3, L4 and M2 above.

## Files Reviewed

| File | Change |
|---|---|
| `cli/location.go` | Added |
| `commands/location.go` | Added |
| `commands/location_test.go` | Added |
| `devices/android_location.go` | Added |
| `devices/ios_location.go` | Added |
| `devices/simulator_location.go` | Added |
| `devices/location_test.go` | Added |
| `agents/android/java/MockLocation.java` | Added |
| `devices/common.go` | Modified |
| `server/server.go`, `server/dispatch.go` | Modified |
| `docs/openrpc.json`, `docs/openrpc.md`, `README.md` | Modified |
