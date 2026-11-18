import { execFileSync } from "node:child_process";

// A single booted simulator device entry from `simctl list devices -j`.
type SimDevice = { udid: string; state: string };

// Bundle id of the XCUITest runner (PRODUCT_BUNDLE_IDENTIFIER
// `com.mobilenext.devicekit-iosUITests` + the `.xctrunner` suffix Xcode adds).
// Launching it on a booted simulator starts the JSON-RPC server (see README).
const RUNNER_BUNDLE_ID = "com.mobilenext.devicekit-iosUITests.xctrunner";

// Must match the port in playwright.config.ts (baseURL http://localhost:12004).
const PORT = 12004;

// How long to wait for /health after launching the runner.
const HEALTH_TIMEOUT_MS = 120_000;
const HEALTH_POLL_INTERVAL_MS = 1_000;

function sleepSync(ms: number): void {
  Atomics.wait(new Int32Array(new SharedArrayBuffer(4)), 0, 0, ms);
}

function findBootedSimulatorUdid(): string {
  const override = process.env.DEVICEKIT_SIMULATOR_UDID;
  if (override) {
    return override;
  }

  const json = execFileSync("xcrun", ["simctl", "list", "devices", "booted", "-j"], {
    encoding: "utf8",
  });
  const devices = JSON.parse(json).devices as Record<string, SimDevice[]>;
  const booted = Object.values(devices)
    .flat()
    .find((device) => device.state === "Booted");

  if (!booted) {
    throw new Error(
      "No booted simulator found. Boot one (e.g. `xcrun simctl boot <udid>`) or set DEVICEKIT_SIMULATOR_UDID.",
    );
  }
  return booted.udid;
}

function killRunnerOnSimulator(udid: string): void {
  try {
    execFileSync("xcrun", ["simctl", "terminate", udid, RUNNER_BUNDLE_ID], { stdio: "ignore" });
  } catch {
    // Runner wasn't running — nothing to kill.
  }
}

function launchRunnerOnSimulator(udid: string): void {
  execFileSync("xcrun", ["simctl", "launch", udid, RUNNER_BUNDLE_ID], {
    stdio: "ignore",
    // simctl forwards SIMCTL_CHILD_* vars to the launched app (minus the prefix),
    // so the server binds the port we expect. Other SIMCTL_CHILD_* vars already in
    // the environment (e.g. LLVM_PROFILE_FILE for coverage) are passed through too.
    env: {
      ...process.env,
      SIMCTL_CHILD_DEVICEKIT_LISTEN_PORT: String(PORT),
    },
  });
}

function waitForServerHealthy(): void {
  const deadline = Date.now() + HEALTH_TIMEOUT_MS;
  while (Date.now() < deadline) {
    try {
      const body = execFileSync("curl", ["-s", `http://localhost:${PORT}/health`], {
        encoding: "utf8",
      });
      if (body.trim() === "OK") {
        return;
      }
    } catch {
      // Server not accepting connections yet.
    }
    sleepSync(HEALTH_POLL_INTERVAL_MS);
  }
  throw new Error(
    `XCUITest server did not become healthy on port ${PORT} within ${HEALTH_TIMEOUT_MS / 1000}s`,
  );
}

function globalSetup(): void {
  const udid = findBootedSimulatorUdid();
  console.log(`[global-setup] using simulator ${udid}`);

  killRunnerOnSimulator(udid);
  console.log(`[global-setup] launching ${RUNNER_BUNDLE_ID} on port ${PORT}`);
  launchRunnerOnSimulator(udid);

  waitForServerHealthy();
  console.log(`[global-setup] server healthy on http://localhost:${PORT}`);
}

export default globalSetup;
