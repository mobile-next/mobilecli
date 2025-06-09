#!/usr/bin/env node

const { join } = require("node:path");
const { spawn } = require("node:child_process");

let binary;

switch (process.platform) {
	case "darwin":
		binary = "mobilectl-darwin";
		break;

	case "linux":
		switch (process.arch) {
			case "arm64":
				binary = "mobilectl-linux-arm64";
				break;
			case "x64":
				binary = "mobilectl-linux-amd64";
				break;
		}
		break;

	default:
		console.error(`Unsupported platform: ${process.platform}-${process.arch}`);
		process.exit(1);
}

const binaryPath = join(__dirname, "bin", binary);

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

child.on("close", (code) => {
	process.exit(code);
});