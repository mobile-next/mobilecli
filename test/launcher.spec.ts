import {test, expect} from '@playwright/test';
import {spawn, type ChildProcess} from 'child_process';
import {chmodSync, copyFileSync, mkdirSync, mkdtempSync, rmSync, writeFileSync} from 'fs';
import * as os from 'os';
import * as path from 'path';

// the npm package's entry point: it finds the native binary for this platform
// and runs it, so `mobilecli` installed from npm behaves like the binary itself
const LAUNCHER = path.join(__dirname, '..', 'publish', 'npm', 'index.js');

// what the launcher's platform switch resolves to on this machine
const NPM_ARCH: Record<string, string> = {x64: 'amd64', arm64: 'arm64'};
const NPM_PLATFORM: Record<string, string> = {darwin: 'darwin', linux: 'linux', win32: 'windows'};
const PLATFORM_PACKAGE = `@mobilenext/mobilecli-${NPM_PLATFORM[process.platform]}-${NPM_ARCH[process.arch]}`;
const BINARY_NAME = path.basename(PLATFORM_PACKAGE.replace('@mobilenext/', '')) + (process.platform === 'win32' ? '.exe' : '');

// exit code the stand-in binary uses once it has been told to terminate
const BINARY_EXIT_CODE_AFTER_SIGTERM = 3;

// stands in for the native binary. it stays alive until it is told to terminate,
// and says so before exiting with a code of its own. the pid is announced only
// once the handler is in place, because the announcement is what the tests take
// as permission to send signals
const STAND_IN_BINARY = `#!/usr/bin/env node
process.on('SIGTERM', () => {
	process.stdout.write('binary got SIGTERM\\n');
	process.exit(${BINARY_EXIT_CODE_AFTER_SIGTERM});
});
process.stdout.write('pid ' + process.pid + '\\n');
setInterval(() => {}, 1000);
`;

type ExitStatus = {code: number | null; signal: NodeJS.Signals | null};

// the launcher next to a node_modules holding only the stand-in platform package
type InstalledLauncher = {
	launcherPath: string;
	root: string;
};

type RunningLauncher = {
	launcher: ChildProcess;
	binaryPid: number;
	output: () => string;
	exited: Promise<ExitStatus>;
};

test.describe('npm launcher', () => {
	// windows has no signals to forward: kill() there ends the launcher outright
	test.skip(process.platform === 'win32', 'posix signal semantics');

	let installed: InstalledLauncher;
	let running: RunningLauncher | undefined;

	test.beforeAll(() => {
		installed = installLauncherWithStandInBinary();
	});

	test.afterAll(() => {
		rmSync(installed.root, {recursive: true, force: true});
	});

	test.afterEach(() => {
		if (running) {
			killIfStillRunning(running.binaryPid);
			killIfStillRunning(running.launcher.pid!);
			running = undefined;
		}
	});

	test('passes a termination signal on to the binary and waits for it to finish', async () => {
		running = await startLauncher(installed);

		running.launcher.kill('SIGTERM');
		const status = await running.exited;

		expect(running.output()).toContain('binary got SIGTERM');
		expect(isRunning(running.binaryPid), 'the binary outlived the launcher').toBe(false);
		expect(status).toEqual({code: BINARY_EXIT_CODE_AFTER_SIGTERM, signal: null});
	});

	test('dies the way the binary died when the binary is killed by a signal', async () => {
		running = await startLauncher(installed);

		process.kill(running.binaryPid, 'SIGKILL');
		const status = await running.exited;

		expect(status).toEqual({code: null, signal: 'SIGKILL'});
	});
});

// lays the launcher out the way npm installs it: index.js next to a node_modules
// that holds the platform package, here with the stand-in as its binary. a copy
// in a fresh directory keeps the launcher's require.resolve away from any real
// platform package installed further up the tree
function installLauncherWithStandInBinary(): InstalledLauncher {
	const root = mkdtempSync(path.join(os.tmpdir(), 'mobilecli-launcher-'));
	const packageDir = path.join(root, 'node_modules', ...PLATFORM_PACKAGE.split('/'));
	mkdirSync(packageDir, {recursive: true});
	writeFileSync(path.join(packageDir, 'package.json'), JSON.stringify({name: PLATFORM_PACKAGE, version: '0.0.0'}));

	const binaryPath = path.join(packageDir, BINARY_NAME);
	writeFileSync(binaryPath, STAND_IN_BINARY);
	chmodSync(binaryPath, 0o755);

	const launcherPath = path.join(root, 'index.js');
	copyFileSync(LAUNCHER, launcherPath);
	return {launcherPath, root};
}

async function startLauncher(installed: InstalledLauncher): Promise<RunningLauncher> {
	const launcher = spawn(process.execPath, [installed.launcherPath], {
		stdio: ['ignore', 'pipe', 'pipe'],
	});

	let output = '';
	launcher.stdout!.on('data', (chunk: Buffer) => {
		output += chunk.toString();
	});

	// 'close' rather than 'exit': it also waits for the shared stdout pipe to drain,
	// so everything the binary wrote has been read before the tests look at it
	const exited = new Promise<ExitStatus>((resolve) => launcher.once('close', (code, signal) => resolve({code, signal})));

	const binaryPid = await pidAnnouncedBy(launcher);
	return {launcher, binaryPid, output: () => output, exited};
}

// the stand-in binary's first line of output is its pid
function pidAnnouncedBy(launcher: ChildProcess): Promise<number> {
	return new Promise((resolve, reject) => {
		let seen = '';
		const onData = (chunk: Buffer) => {
			seen += chunk.toString();
			const match = seen.match(/^pid (\d+)$/m);
			if (match) {
				launcher.stdout!.off('data', onData);
				resolve(Number(match[1]));
			}
		};
		launcher.stdout!.on('data', onData);
		launcher.on('exit', () => reject(new Error(`launcher exited before the binary announced its pid: ${seen}`)));
	});
}

function isRunning(pid: number): boolean {
	try {
		process.kill(pid, 0);
		return true;
	} catch {
		return false;
	}
}

function killIfStillRunning(pid: number): void {
	if (isRunning(pid)) {
		process.kill(pid, 'SIGKILL');
	}
}
