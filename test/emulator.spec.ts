import {test, expect} from '@playwright/test';
import {execFileSync} from 'child_process';
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

function writeTempFile(content: string): string {
	const tmpPath = path.join(os.tmpdir(), `mobilecli-push-${Date.now()}.txt`);
	fs.writeFileSync(tmpPath, content, 'utf8');
	return tmpPath;
}
