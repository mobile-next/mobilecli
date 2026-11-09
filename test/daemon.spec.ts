import {test, expect} from '@playwright/test';
import {execSync, spawn} from 'child_process';
import {mkdtempSync, rmSync, existsSync} from 'fs';
import * as os from 'os';
import * as path from 'path';
import {coverageEnv} from './coverage';

const BINARY = path.join(__dirname, '..', 'mobilecli');

// a short, private state dir per run: unix socket paths are length-limited and
// the suite must never touch the developer's real ~/.mobilecli
// (macOS's os.tmpdir() is too long for a unix socket path)
const tempRoot = process.platform === 'darwin' ? '/tmp' : os.tmpdir();
const home = mkdtempSync(path.join(tempRoot, 'mcli-e2e-'));
const env = {...coverageEnv(), MOBILECLI_HOME: home};

function mobilecli(args: string): string {
	return execSync(`${BINARY} ${args}`, {env, encoding: 'utf8', stdio: ['ignore', 'pipe', 'pipe']});
}

function daemonStatus(): {running: boolean; pid?: number; version?: string} {
	return JSON.parse(mobilecli('daemon status'));
}

function socketExists(): boolean {
	return existsSync(path.join(home, 'daemon.sock'));
}

test.describe('daemon lifecycle', () => {
	test.afterAll(() => {
		try {
			mobilecli('daemon stop');
		} catch {
			// already stopped
		}
		rmSync(home, {recursive: true, force: true});
	});

	test('reports not running before any device command', () => {
		expect(daemonStatus().running).toBe(false);
	});

	test('--help and --version do not start the daemon', () => {
		mobilecli('--help');
		mobilecli('--version');
		expect(socketExists()).toBe(false);
	});

	test('a device command starts the daemon automatically', () => {
		const devices = JSON.parse(mobilecli('devices'));
		expect(devices.status).toBe('ok');

		const status = daemonStatus();
		expect(status.running).toBe(true);
		expect(status.pid).toBeGreaterThan(0);
		expect(status.version).toBeTruthy();
	});

	test('a second device command reuses the same daemon', () => {
		const before = daemonStatus().pid;
		mobilecli('devices');
		expect(daemonStatus().pid).toBe(before);
	});

	test('errors from the daemon keep the cli envelope and exit code', () => {
		let stdout = '';
		let exitCode = 0;
		try {
			mobilecli('io tap --device __no_such_device__ 1,1');
		} catch (err: any) {
			stdout = err.stdout;
			exitCode = err.status;
		}
		expect(exitCode).toBe(1);
		expect(JSON.parse(stdout)).toMatchObject({status: 'error'});
		expect(stdout).toContain('__no_such_device__');
	});

	test('daemon stop shuts it down and removes the socket', () => {
		expect(JSON.parse(mobilecli('daemon stop'))).toEqual({status: 'ok'});
		expect(daemonStatus().running).toBe(false);
		expect(socketExists()).toBe(false);
	});

	test('daemon start runs in the foreground and honours --idle-timeout', async () => {
		const child = spawn(BINARY, ['daemon', 'start', '--idle-timeout', '2s'], {env, stdio: 'ignore'});
		const exited = new Promise<number | null>((resolve) => child.on('exit', resolve));

		await expect.poll(() => daemonStatus().running, {timeout: 5000}).toBe(true);

		// no requests for longer than the idle timeout: the daemon exits on its own
		expect(await exited).toBe(0);
		expect(daemonStatus().running).toBe(false);
	});
});
