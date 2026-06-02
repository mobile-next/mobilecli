import {test, expect} from '@playwright/test';
import {execFileSync, spawn} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';

const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');

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

function getFirstAndroidDevice(): Device | null {
	try {
		const output = execFileSync(mobilecliBinary, ['devices'], {encoding: 'utf8'});
		const result = JSON.parse(output);
		return result?.data?.devices?.find((d: Device) => d.platform === 'android') ?? null;
	} catch (error) {
		return null;
	}
}

test.describe('Android Emulator Tests', () => {
	let device: Device | null;

	test.beforeAll(() => {
		device = getFirstAndroidDevice();
		if (!device) {
			console.log('No Android device found. See test/README.md for setup instructions.');
		}
	});

	test('should take screenshot', () => {
		test.skip(!device, 'No Android device found');

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
			mobilecli(['screenrecord', '--device', device!.id, '--time-limit', '5', '--output', videoPath]);

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

	test.describe('fs operations on app container (com.mobilenext.playground)', () => {
		const packageName = 'com.mobilenext.playground';
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

function mobilecli(args: string[]): void {
	try {
		execFileSync(mobilecliBinary, args, {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
		});
	} catch (error: any) {
		console.log(`Command failed: ${mobilecliBinary} ${args.join(' ')}`);
		if (error.stderr) console.log(`stderr: ${error.stderr}`);
		if (error.stdout) console.log(`stdout: ${error.stdout}`);
		throw error;
	}
}

function mobilecliJson(args: string[]): any {
	const result = execFileSync(mobilecliBinary, args, {
		encoding: 'utf8',
		timeout: 60000,
		stdio: ['pipe', 'pipe', 'pipe'],
		env: { ANDROID_HOME: process.env.ANDROID_HOME || '' },
	});
	return JSON.parse(result);
}

function getAppContainerPath(deviceId: string, packageName: string): string {
	const response = mobilecliJson(['apps', 'path', packageName, '--device', deviceId]);
	expect(response.status).toBe('ok');
	return response.data.path;
}

function fsList(deviceId: string, remotePath: string): any[] {
	const response = mobilecliJson(['fs', 'ls', '--device', deviceId, remotePath]);
	expect(response.status).toBe('ok');
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
