import {test, expect} from './fixtures';
import {execFileSync, spawn} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import type {UIElement, UIDumpResponse, ForegroundAppResponse} from './types';
import {coverageEnv} from './coverage';
import {
	expectAgentShape,
	expectAppShape,
	expectFsListingShape,
	expectForegroundAppShape,
	expectOkEnvelope,
	expectUIDumpShape,
} from './shapes';

const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');

// settings is present on every android image, unlike chrome or play store
const SETTINGS_PACKAGE = 'com.android.settings';

const PLAYGROUND_PACKAGE = 'com.mobilenext.playground';

// the settings search ui is a separate package that joins the settings task via
// taskAffinity=com.android.settings.root. force-stopping settings alone leaves its
// activity on top of that task, so `am start` resumes a task settings no longer owns
const SETTINGS_SEARCH_PACKAGE = 'com.google.android.settings.intelligence';

const AGENT_BUNDLE_ID = 'com.mobilenext.devicekit';
const AGENT_MISSING_MESSAGE = 'Agent is not installed on the device';

// google_apis images use com.google.android.apps.nexuslauncher, aosp images use
// com.android.launcher3 — match on the shared substring instead of pinning one
const LAUNCHER_PACKAGE_PATTERN = /launcher/i;

type Device = {
	id: string;
	name: string;
	platform: string;
	type: string;
	version: string;
	state: string;
};

type Dimensions = {
	width: number;
	height: number;
};

type Point = {
	x: number;
	y: number;
};

// this spec is written against an emulator: it force-stops apps, writes to
// /sdcard, and leaves the device on arbitrary screens. a physical android device
// reports type "real" and is left alone.
function getFirstAndroidDevice(deviceType: string): Device | null {
	try {
		const output = execFileSync(mobilecliBinary, ['devices', '--platform', 'android', '--type', deviceType], {
			encoding: 'utf8',
			env: coverageEnv(),
		});
		return JSON.parse(output)?.data?.devices?.[0] ?? null;
	} catch (error) {
		return null;
	}
}

test.describe('Android Tests', () => {
	let device: Device | null;

	test.beforeAll(({deviceType}) => {
		device = getFirstAndroidDevice(deviceType);
		if (!device) {
			console.log(`No Android ${deviceType} device found. See test/README.md for setup instructions.`);
		}
	});

	test('should take screenshot', ({deviceType}) => {
		test.skip(!device, 'No Android device found');
		// a locked or sleeping handset produces a near-empty png well under this floor
		test.skip(deviceType === 'real', 'screenshot size assumes a known screen state');

		const screenshotPath = `/tmp/screenshot-android-${Date.now()}.png`;
		mobilecli(['screenshot', '--device', device!.id, '--format', 'png', '--output', screenshotPath]);

		const fileExists = fs.existsSync(screenshotPath);
		expect(fileExists).toBe(true);

		const stats = fs.statSync(screenshotPath);
		expect(stats.size).toBeGreaterThan(64 * 1024);
	});

	test.describe('screenrecord', () => {
		test('should record with --time-limit 5 and produce a playable mp4', () => {
			test.skip(!device, 'No Android device found');

			const videoPath = path.join(os.tmpdir(), `mobilecli-rec-timelimit-${Date.now()}.mp4`);
			recordScreenWithTimeLimit(device!.id, videoPath, 5);

			assertVideoIsPlayable(videoPath);
			fs.unlinkSync(videoPath);
		});

		test('should record without time limit and finalize a playable mp4 on Ctrl-C', async () => {
			test.skip(!device, 'No Android device found');

			const videoPath = path.join(os.tmpdir(), `mobilecli-rec-ctrlc-${Date.now()}.mp4`);
			await recordThenInterruptWithCtrlC(device!.id, videoPath, 5);

			assertVideoIsPlayable(videoPath);
			fs.unlinkSync(videoPath);
		});
	});

	test('should open URL https://example.com', () => {
		test.skip(!device, 'No Android device found');

		mobilecli(['url', 'https://example.com', '--device', device!.id]);
	});

	test('should get device info', () => {
		test.skip(!device, 'No Android device found');

		mobilecli(['device', 'info', '--device', device!.id]);
	});

	test('should list installed apps', () => {
		test.skip(!device, 'No Android device found');

		const apps = listApps(device!.id);
		apps.forEach(expectAppShape);
		expect(apps.map((a: any) => a.packageName)).toContain(SETTINGS_PACKAGE);
	});

	test('should launch Settings app and verify it is in foreground', async () => {
		test.skip(!device, 'No Android device found');

		launchApp(device!.id, SETTINGS_PACKAGE);
		await sleep(3000);

		expect(getForegroundApp(device!.id).data.packageName).toBe(SETTINGS_PACKAGE);
	});

	test('should terminate Settings app and verify launcher is in foreground', async () => {
		test.skip(!device, 'No Android device found');

		// tear down any existing settings task so the launch below builds a fresh one
		clearSettingsTask(device!.id);

		// force-stop returns to whatever task sits below the app, so start from the
		// launcher — otherwise an app left running by an earlier test surfaces instead
		pressButton(device!.id, 'HOME');
		await sleep(2000);

		launchApp(device!.id, SETTINGS_PACKAGE);
		await sleep(3000);

		terminateApp(device!.id, SETTINGS_PACKAGE);
		await sleep(3000);

		expect(getForegroundApp(device!.id).data.packageName).toMatch(LAUNCHER_PACKAGE_PATTERN);
	});

	test('should handle launching app twice (idempotency)', async () => {
		test.skip(!device, 'No Android device found');

		launchApp(device!.id, SETTINGS_PACKAGE);
		await sleep(3000);

		// launching again should resume the app, not fail
		launchApp(device!.id, SETTINGS_PACKAGE);
		await sleep(3000);

		expect(getForegroundApp(device!.id).data.packageName).toBe(SETTINGS_PACKAGE);
	});

	test('should press HOME button and return to launcher from Settings', async () => {
		test.skip(!device, 'No Android device found');

		launchApp(device!.id, SETTINGS_PACKAGE);
		await sleep(3000);
		expect(getForegroundApp(device!.id).data.packageName).toBe(SETTINGS_PACKAGE);

		pressButton(device!.id, 'HOME');
		await sleep(3000);

		expect(getForegroundApp(device!.id).data.packageName).toMatch(LAUNCHER_PACKAGE_PATTERN);
	});

	test('should tap on Network & internet in Settings and navigate to that screen', async ({deviceType}) => {
		test.skip(!device, 'No Android device found');
		// matches on english settings labels, so it only holds for a known locale
		test.skip(deviceType === 'real', 'asserts english ui labels');

		// `am start` resumes an existing task, so clear it first to guarantee we
		// land on the settings root screen rather than wherever a previous test left it
		clearSettingsTask(device!.id);
		launchApp(device!.id, SETTINGS_PACKAGE);
		await sleep(5000);

		const entry = findElementByText(dumpUI(device!.id), 'Network & internet');
		tap(device!.id, centerOf(entry).x, centerOf(entry).y);
		await sleep(3000);

		verifyElementWithTextExists(dumpUI(device!.id), 'Airplane mode');
	});

	test.describe('fs operations on /sdcard/Download', () => {
		const remoteDir = '/sdcard/Download/mobilecli-test';
		const remoteFile = `${remoteDir}/hello.txt`;

		test('should create a nested directory with mkdir -p', () => {
			test.skip(!device, 'No Android device found');
			fsMkdir(device!.id, remoteDir, true);
		});

		test('should push a file into /sdcard/Download', () => {
			test.skip(!device, 'No Android device found');
			const localFile = writeTempFile('hello from mobilecli');
			fsPush(device!.id, localFile, remoteFile);
			fs.unlinkSync(localFile);
		});

		test('should list the pushed file in /sdcard/Download', () => {
			test.skip(!device, 'No Android device found');
			const entries = fsList(device!.id, remoteDir);
			const names = entries.map((e: any) => e.name);
			expect(names).toContain('hello.txt');
		});

		test('should pull the file back and verify contents match', () => {
			test.skip(!device, 'No Android device found');
			const localDest = path.join(os.tmpdir(), `mobilecli-pull-${Date.now()}.txt`);
			fsPull(device!.id, remoteFile, localDest);
			const contents = fs.readFileSync(localDest, 'utf8');
			expect(contents.trim()).toBe('hello from mobilecli');
			fs.unlinkSync(localDest);
		});

		test('should remove the test directory recursively', () => {
			test.skip(!device, 'No Android device found');
			fsRm(device!.id, remoteDir, true);
			const entries = fsList(device!.id, '/sdcard/Download');
			const names = entries.map((e: any) => e.name);
			expect(names).not.toContain('mobilecli-test');
		});
	});

	// these run in order and share device state: each step depends on what the
	// previous one left installed
	test.describe('agent lifecycle', () => {
		test.beforeAll(() => {
			if (!device) return;
			// the device may arrive with an agent from an earlier run, so start clean.
			// the response is ignored: "not installed" is a perfectly good outcome here
			runAgentCommand(device.id, 'uninstall');
		});

		test('status should report no agent before installation', () => {
			test.skip(!device, 'No Android device found');

			const response = runAgentCommand(device!.id, 'status');
			expect(response.status).toBe('fail');
			expect(response.data.message).toBe(AGENT_MISSING_MESSAGE);
		});

		test('uninstall should report no agent when none is installed', () => {
			test.skip(!device, 'No Android device found');

			const response = runAgentCommand(device!.id, 'uninstall');
			expect(response.status).toBe('fail');
			expect(response.data.message).toBe(AGENT_MISSING_MESSAGE);
		});

		test('install should report a successful installation', () => {
			test.skip(!device, 'No Android device found');

			const response = runAgentCommand(device!.id, 'install');
			expect(response.status).toBe('ok');
			expect(response.data.message).toBe('Agent installed successfully');
			expectAgentShape(response.data.agent);
			expect(response.data.agent.bundleId).toBe(AGENT_BUNDLE_ID);
		});

		test('installing again should report the agent is already installed', () => {
			test.skip(!device, 'No Android device found');

			const response = runAgentCommand(device!.id, 'install');
			expect(response.status).toBe('ok');
			expect(response.data.message).toBe('Agent is already installed');
			expectAgentShape(response.data.agent);
			expect(response.data.agent.bundleId).toBe(AGENT_BUNDLE_ID);
		});

		test('status should report the installed agent', () => {
			test.skip(!device, 'No Android device found');

			const response = runAgentCommand(device!.id, 'status');
			expect(response.status).toBe('ok');
			expectAgentShape(response.data.agent);
			expect(response.data.agent.bundleId).toBe(AGENT_BUNDLE_ID);
			// derived from the reported version rather than a literal, so bumping the
			// pinned agent in cli/agent.go does not require editing this test
			expect(response.data.message).toBe(`Agent version ${response.data.agent.version} is installed on device`);
		});

		test('uninstall should succeed once an agent is installed', () => {
			test.skip(!device, 'No Android device found');

			const response = runAgentCommand(device!.id, 'uninstall');
			expect(response.status).toBe('ok');
			expect(response.data.message).toBe('Agent uninstalled successfully');
		});
	});

	test.describe('fs operations on app container (com.mobilenext.playground)', () => {
		// reading an app sandbox needs a debuggable build installed, which is
		// guaranteed on an emulator image but not on someone's phone
		test.skip(({deviceType}) => deviceType === 'real', 'requires a debuggable playground build');

		const packageName = PLAYGROUND_PACKAGE;
		let containerPath: string;
		let remoteDir: string;
		let remoteFile: string;

		test.beforeAll(() => {
			if (!device) return;
			containerPath = getAppContainerPath(device.id, packageName);
			remoteDir = `${containerPath}/files/mobilecli-test`;
			remoteFile = `${remoteDir}/data.txt`;
		});

		test('should return a valid container path for com.mobilenext.playground', () => {
			test.skip(!device, 'No Android device found');
			expect(containerPath).toMatch(/^\/data\/user\/\d+\/com\.mobilenext\.playground/);
		});

		test('should list the app container root', () => {
			test.skip(!device, 'No Android device found');
			const entries = fsList(device!.id, containerPath);
			expect(Array.isArray(entries)).toBe(true);
		});

		test('should create a directory inside the app container', () => {
			test.skip(!device, 'No Android device found');
			fsMkdir(device!.id, remoteDir, true);
		});

		test('should push a file into the app container', () => {
			test.skip(!device, 'No Android device found');
			const localFile = writeTempFile('app container test');
			fsPush(device!.id, localFile, remoteFile);
			fs.unlinkSync(localFile);
		});

		test('should list the file inside the app container', () => {
			test.skip(!device, 'No Android device found');
			const entries = fsList(device!.id, remoteDir);
			const names = entries.map((e: any) => e.name);
			expect(names).toContain('data.txt');
		});

		test('should pull the file from the app container and verify contents', () => {
			test.skip(!device, 'No Android device found');
			const localDest = path.join(os.tmpdir(), `mobilecli-pull-app-${Date.now()}.txt`);
			fsPull(device!.id, remoteFile, localDest);
			const contents = fs.readFileSync(localDest, 'utf8');
			expect(contents.trim()).toBe('app container test');
			fs.unlinkSync(localDest);
		});

		test('should remove the test directory from the app container', () => {
			test.skip(!device, 'No Android device found');
			fsRm(device!.id, remoteDir, true);
			const entries = fsList(device!.id, `${containerPath}/files`);
			const names = entries.map((e: any) => e.name);
			expect(names).not.toContain('mobilecli-test');
		});
	});
});

// every command routed through here answers with a json envelope; asserting it
// centrally means a change to the output format fails these tests instead of
// silently passing because only the exit code was checked
function mobilecli(args: string[]): any {
	try {
		const output = execFileSync(mobilecliBinary, args, {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
			env: coverageEnv(),
		});
		return expectOkEnvelope(JSON.parse(output));
	} catch (error: any) {
		console.log(`Command failed: ${mobilecliBinary} ${args.join(' ')}`);
		if (error.stderr) console.log(`stderr: ${error.stderr}`);
		if (error.stdout) console.log(`stdout: ${error.stdout}`);
		throw error;
	}
}

function mobilecliJson(args: string[]): any {
	try {
		const result = execFileSync(mobilecliBinary, args, {
			encoding: 'utf8',
			timeout: 60000,
			stdio: ['pipe', 'pipe', 'pipe'],
			env: coverageEnv(),
		});
		const parsed = JSON.parse(result);
		expectOkEnvelope(parsed);
		return parsed;
	} catch (error: any) {
		console.log(`Command failed: ${mobilecliBinary} ${args.join(' ')}`);
		if (error.stderr) console.log(`stderr: ${error.stderr}`);
		if (error.stdout) console.log(`stdout: ${error.stdout}`);
		throw error;
	}
}

// screenrecord streams progress to stderr and prints no json envelope, so it
// bypasses mobilecli() entirely
function recordScreenWithTimeLimit(deviceId: string, videoPath: string, timeLimitSeconds: number): void {
	execFileSync(mobilecliBinary, ['screenrecord', '--device', deviceId, '--time-limit', String(timeLimitSeconds), '--output', videoPath], {
		encoding: 'utf8',
		timeout: 180000,
		stdio: ['pipe', 'pipe', 'pipe'],
		env: coverageEnv(),
	});
}

function sleep(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

function listApps(deviceId: string): any[] {
	return mobilecliJson(['apps', 'list', '--device', deviceId]).data;
}

function launchApp(deviceId: string, packageName: string): void {
	mobilecli(['apps', 'launch', packageName, '--device', deviceId]);
}

function terminateApp(deviceId: string, packageName: string): void {
	mobilecli(['apps', 'terminate', packageName, '--device', deviceId]);
}

// force-stops every package that can hold an activity in the settings task, so the
// next launch starts from a clean root instead of resuming whatever was left open
// agent subcommands report a missing agent with status "fail" and still exit 0, so
// they cannot go through mobilecli(), which asserts an ok envelope
function runAgentCommand(deviceId: string, subcommand: string): any {
	const args = ['agent', subcommand, '--device', deviceId];
	try {
		const output = execFileSync(mobilecliBinary, args, {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
			env: coverageEnv(),
		});
		return JSON.parse(output);
	} catch (error: any) {
		console.log(`Command failed: ${mobilecliBinary} ${args.join(' ')}`);
		if (error.stderr) console.log(`stderr: ${error.stderr}`);
		if (error.stdout) console.log(`stdout: ${error.stdout}`);
		throw error;
	}
}

function clearSettingsTask(deviceId: string): void {
	terminateApp(deviceId, SETTINGS_PACKAGE);
	terminateApp(deviceId, SETTINGS_SEARCH_PACKAGE);
}

function getForegroundApp(deviceId: string): ForegroundAppResponse {
	const response = mobilecliJson(['apps', 'foreground', '--device', deviceId]);
	expectForegroundAppShape(response.data);
	return response;
}

function dumpUI(deviceId: string): UIDumpResponse {
	const response = mobilecliJson(['dump', 'ui', '--device', deviceId]);
	expectUIDumpShape(response.data.elements);
	return response;
}

function tap(deviceId: string, x: number, y: number): void {
	mobilecli(['io', 'tap', `${x},${y}`, '--device', deviceId]);
}

function pressButton(deviceId: string, button: string): void {
	mobilecli(['io', 'button', button, '--device', deviceId]);
}

// android returns the view hierarchy as a nested tree, so flatten it before searching
function flattenElements(elements: UIElement[]): UIElement[] {
	return elements.flatMap(element => [element, ...flattenElements(element.children ?? [])]);
}

function findElementByText(uiDump: UIDumpResponse, text: string): UIElement {
	const element = flattenElements(uiDump.data.elements).find(el => el.text === text);
	if (!element) {
		throw new Error(`Element with text "${text}" not found. Available texts: ${allTextsIn(uiDump).join(', ')}`);
	}

	return element;
}

function verifyElementWithTextExists(uiDump: UIDumpResponse, text: string): void {
	const exists = flattenElements(uiDump.data.elements).some(el => el.text === text);
	expect(exists, `Expected an element with text "${text}". Available texts: ${allTextsIn(uiDump).join(', ')}`).toBe(true);
}

function allTextsIn(uiDump: UIDumpResponse): string[] {
	return flattenElements(uiDump.data.elements).map(el => el.text).filter(Boolean) as string[];
}

function centerOf(element: UIElement): Point {
	return {
		x: element.rect.x + Math.floor(element.rect.width / 2),
		y: element.rect.y + Math.floor(element.rect.height / 2),
	};
}

function getAppContainerPath(deviceId: string, packageName: string): string {
	return mobilecliJson(['apps', 'path', packageName, '--device', deviceId]).data.path;
}

function fsList(deviceId: string, remotePath: string): any[] {
	const response = mobilecliJson(['fs', 'ls', '--device', deviceId, remotePath]);
	expectFsListingShape(response.data);
	return response.data;
}

function fsPush(deviceId: string, localPath: string, remotePath: string): void {
	mobilecli(['fs', 'push', '--device', deviceId, localPath, remotePath]);
}

function fsPull(deviceId: string, remotePath: string, localPath: string): void {
	mobilecli(['fs', 'pull', '--device', deviceId, remotePath, localPath]);
}

function fsMkdir(deviceId: string, remotePath: string, parents: boolean): void {
	mobilecli(['fs', 'mkdir', '--device', deviceId, ...(parents ? ['-p'] : []), remotePath]);
}

function fsRm(deviceId: string, remotePath: string, recursive: boolean): void {
	mobilecli(['fs', 'rm', '--device', deviceId, ...(recursive ? ['-r'] : []), remotePath]);
}

// records the screen with no time limit, lets it run for recordSeconds, then
// sends SIGINT (Ctrl-C). mobilecli is expected to catch the signal, finalize
// the mp4, and exit cleanly. resolves once the process has fully exited.
function recordThenInterruptWithCtrlC(deviceId: string, outputPath: string, recordSeconds: number): Promise<void> {
	return new Promise((resolve, reject) => {
		const child = spawn(mobilecliBinary, ['screenrecord', '--device', deviceId, '--output', outputPath], {
			stdio: ['pipe', 'pipe', 'pipe'],
			env: coverageEnv(),
		});

		child.on('error', reject);
		child.on('close', () => resolve());

		setTimeout(() => child.kill('SIGINT'), recordSeconds * 1000);
	});
}

// verifies the recording is a non-empty, well-formed mp4 that ffprobe can
// decode and report real video dimensions for (a corrupt file makes ffprobe
// exit non-zero, which throws and fails the test).
function assertVideoIsPlayable(videoPath: string): void {
	expect(fs.existsSync(videoPath)).toBe(true);
	expect(fs.statSync(videoPath).size).toBeGreaterThan(0);

	const {width, height} = probeVideoDimensions(videoPath);
	expect(width).toBeGreaterThan(0);
	expect(height).toBeGreaterThan(0);
}

function probeVideoDimensions(videoPath: string): Dimensions {
	const output = execFileSync('ffprobe', [
		'-v', 'error',
		'-select_streams', 'v:0',
		'-show_entries', 'stream=width,height',
		'-of', 'csv=s=x:p=0',
		videoPath,
	], {encoding: 'utf8'}).trim();

	const [width, height] = output.split('x').map(Number);
	return {width, height};
}

function writeTempFile(content: string): string {
	const tmpPath = path.join(os.tmpdir(), `mobilecli-push-${Date.now()}.txt`);
	fs.writeFileSync(tmpPath, content, 'utf8');
	return tmpPath;
}
