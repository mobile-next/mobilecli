#!/usr/bin/env node

const { join } = require("node:path");
const { spawn } = require("node:child_process");
const { constants } = require("node:os");

let packageName;
let binaryName;

switch (process.platform) {
	case "darwin":
		switch (process.arch) {
			case "arm64":
				packageName = "@mobilenext/mobilecli-darwin-arm64";
				binaryName = "mobilecli-darwin-arm64";
				break;
			case "x64":
				packageName = "@mobilenext/mobilecli-darwin-amd64";
				binaryName = "mobilecli-darwin-amd64";
				break;
		}
		break;

	case "linux":
	case "android":
		switch (process.arch) {
			case "arm64":
				packageName = "@mobilenext/mobilecli-linux-arm64";
				binaryName = "mobilecli-linux-arm64";
				break;
			case "x64":
				packageName = "@mobilenext/mobilecli-linux-amd64";
				binaryName = "mobilecli-linux-amd64";
				break;
		}
		break;

	case "win32":
		switch (process.arch) {
			case "arm64":
				packageName = "@mobilenext/mobilecli-windows-arm64";
				binaryName = "mobilecli-windows-arm64.exe";
				break;
			case "x64":
				packageName = "@mobilenext/mobilecli-windows-amd64";
				binaryName = "mobilecli-windows-amd64.exe";
				break;
		}
		break;
}

if (!packageName) {
	console.error(`Unsupported platform: ${process.platform}-${process.arch}`);
	process.exit(1);
}

let binaryPath;
try {
	const packagePath = require.resolve(`${packageName}/package.json`);
	binaryPath = join(packagePath, "..", binaryName);
} catch (error) {
	console.error(`Failed to find ${packageName}. Please reinstall @mobilenext/mobilecli.`);
	process.exit(1);
}

const args = process.argv.slice(2);
const child = spawn(binaryPath, args, {
	env: process.env,
	cwd: process.cwd(),
	stdio: [process.stdin, process.stdout, process.stderr],
});

child.on("error", (error) => {
	console.error(error);
	process.exit(1);
});

// a signal sent to this launcher is meant for the binary: pass it on and keep
// waiting, so the binary can finish what the signal asks of it (a recording
// being finalized, a stream being closed) instead of being left behind as an
// orphan when the launcher exits first
const forwardedSignals = ["SIGINT", "SIGTERM", "SIGHUP", "SIGQUIT"].filter((signal) => signal in constants.signals);
for (const signal of forwardedSignals) {
	process.on(signal, () => child.kill(signal));
}

child.on("exit", (code, signal) => {
	if (signal === null) {
		process.exit(code);
	}

	// the binary was killed by a signal, so end this process the same way, and
	// whoever is waiting for it sees the status they would see for the binary
	// itself. the fallback is the shell's convention for the same thing
	process.removeAllListeners(signal);
	process.kill(process.pid, signal);
	process.exit(128 + (constants.signals[signal] ?? 0));
});
