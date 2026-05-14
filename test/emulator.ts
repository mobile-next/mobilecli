import { expect } from 'chai';
import { execSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import {
	createAndLaunchEmulator,
	shutdownEmulator,
	deleteEmulator,
	cleanupEmulators,
	findAndroidSystemImage,
	getAvailableEmulators
} from './avdctl';

const TEST_SERVER_URL = 'http://localhost:12001';

const SUPPORTED_VERSIONS = ['31', '36'];

describe('Android Emulator Tests', () => {
	after(() => {
		cleanupEmulators();
	});

	SUPPORTED_VERSIONS.forEach((apiLevel) => {
		describe(`Android API ${apiLevel}`, () => {
			let emulatorName: string;
			let deviceId: string;
			let systemImageAvailable: boolean = false;

			before(function () {
				this.timeout(300000); // 5 minutes for emulator startup

				try {
					findAndroidSystemImage(apiLevel);
					systemImageAvailable = true;

					console.log(`Creating and launching Android API ${apiLevel} emulator...`);
					const result = createAndLaunchEmulator(apiLevel, 'pixel');
					emulatorName = result.name;
					deviceId = result.deviceId;
					console.log(`Emulator ready: ${emulatorName} (${deviceId})`);
				} catch (error) {
					console.log(`Android API ${apiLevel} system image not available, skipping tests: ${error}`);
					systemImageAvailable = false;
				}
			});

			after(() => {
				if (deviceId && emulatorName) {
					console.log(`Cleaning up emulator ${emulatorName} (${deviceId})`);
					shutdownEmulator(deviceId);
					deleteEmulator(emulatorName);
				}
			});

			it('should take screenshot', async function () {
				if (!systemImageAvailable) {
					this.skip();
					return;
				}

				this.timeout(180000);

				const screenshotPath = `/tmp/screenshot-android${apiLevel}-${Date.now()}.png`;

				takeScreenshot(deviceId, screenshotPath);
				verifyScreenshotFileWasCreated(screenshotPath);
				verifyScreenshotFileHasValidContent(screenshotPath);

				// console.log(`Screenshot saved at: ${screenshotPath}`);
			});

			it('should open URL https://example.com', async function () {
				if (!systemImageAvailable) {
					this.skip();
					return;
				}

				this.timeout(180000);

				openUrl(deviceId, 'https://example.com');
			});

			it('should get device info', async function () {
				if (!systemImageAvailable) {
					this.skip();
					return;
				}

				this.timeout(60000);

				getDeviceInfo(deviceId);
			});

			describe('fs operations on /sdcard/Download', () => {
				const remoteDir = '/sdcard/Download/mobilecli-test';
				const remoteFile = `${remoteDir}/hello.txt`;

				it('should create a nested directory with mkdir -p', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					fsMkdir(deviceId, remoteDir, true);
				});

				it('should push a file into /sdcard/Download', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const localFile = writeTempFile('hello from mobilecli');
					fsPush(deviceId, localFile, remoteFile);
					fs.unlinkSync(localFile);
				});

				it('should list the pushed file in /sdcard/Download', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const entries = fsList(deviceId, remoteDir);
					const names = entries.map((e: any) => e.name);
					expect(names).to.include('hello.txt');
				});

				it('should pull the file back and verify contents match', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const localDest = path.join(os.tmpdir(), `mobilecli-pull-${Date.now()}.txt`);
					fsPull(deviceId, remoteFile, localDest);
					const contents = fs.readFileSync(localDest, 'utf8');
					expect(contents.trim()).to.equal('hello from mobilecli');
					fs.unlinkSync(localDest);
				});

				it('should remove the test directory recursively', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					fsRm(deviceId, remoteDir, true);
					const entries = fsList(deviceId, '/sdcard/Download');
					const names = entries.map((e: any) => e.name);
					expect(names).to.not.include('mobilecli-test');
				});
			});

			describe('fs operations on app container (com.mobilenext.mobilewright_demo)', () => {
				const packageName = 'com.mobilenext.mobilewright_demo';
				let containerPath: string;
				let remoteDir: string;
				let remoteFile: string;

				before(function () {
					if (!systemImageAvailable) return;
					containerPath = getAppContainerPath(deviceId, packageName);
					remoteDir = `${containerPath}/files/mobilecli-test`;
					remoteFile = `${remoteDir}/data.txt`;
				});

				it('should return a valid container path for com.mobilenext.mobilewright_demo', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					expect(containerPath).to.match(/^\/data\/user\/\d+\/com\.mobilenext\.mobilewright_demo/);
				});

				it('should list the app container root', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const entries = fsList(deviceId, containerPath);
					expect(entries).to.be.an('array');
				});

				it('should create a directory inside the app container', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					fsMkdir(deviceId, remoteDir, true);
				});

				it('should push a file into the app container', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const localFile = writeTempFile('app container test');
					fsPush(deviceId, localFile, remoteFile);
					fs.unlinkSync(localFile);
				});

				it('should list the file inside the app container', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const entries = fsList(deviceId, remoteDir);
					const names = entries.map((e: any) => e.name);
					expect(names).to.include('data.txt');
				});

				it('should pull the file from the app container and verify contents', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					const localDest = path.join(os.tmpdir(), `mobilecli-pull-app-${Date.now()}.txt`);
					fsPull(deviceId, remoteFile, localDest);
					const contents = fs.readFileSync(localDest, 'utf8');
					expect(contents.trim()).to.equal('app container test');
					fs.unlinkSync(localDest);
				});

				it('should remove the test directory from the app container', function () {
					if (!systemImageAvailable) { this.skip(); return; }
					this.timeout(30000);
					fsRm(deviceId, remoteDir, true);
					const entries = fsList(deviceId, `${containerPath}/files`);
					const names = entries.map((e: any) => e.name);
					expect(names).to.not.include('mobilecli-test');
				});
			});
		});
	});
});

function mobilecli(args: string): void {
	const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
	const command = `${mobilecliBinary} ${args}`;

	try {
		const result = execSync(command, {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
			env: {
				ANDROID_HOME: process.env.ANDROID_HOME || "",
			}
		});
	} catch (error: any) {
		console.log(`Command failed: ${command}`);
		if (error.stderr) {
			console.log(`Error stderr: ${error.stderr}`);
		}
		if (error.stdout) {
			console.log(`Error stdout: ${error.stdout}`);
		}
		if (error.message && !error.stderr && !error.stdout) {
			console.log(`Error message: ${error.message}`);
		}
		throw error;
	}
}

function takeScreenshot(deviceId: string, screenshotPath: string): void {
	execSync('../mobilecli devices', { stdio: 'inherit' });
	mobilecli(`screenshot --device ${deviceId} --format png --output ${screenshotPath}`);
}

function verifyScreenshotFileWasCreated(screenshotPath: string): void {
	const fileExists = fs.existsSync(screenshotPath);
	expect(fileExists).to.be.true;
	// console.log(`✓ Screenshot file was created: ${screenshotPath}`);
}

function verifyScreenshotFileHasValidContent(screenshotPath: string): void {
	const stats = fs.statSync(screenshotPath);
	const fileSizeInBytes = stats.size;
	expect(fileSizeInBytes).to.be.greaterThan(64 * 1024);
}

function openUrl(deviceId: string, url: string): void {
	mobilecli(`url "${url}" --device ${deviceId}`);
}

function getDeviceInfo(deviceId: string): void {
	mobilecli(`device info --device ${deviceId}`);
}

function mobilecliJson(args: string): any {
	const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
	const result = execSync(`${mobilecliBinary} ${args}`, {
		encoding: 'utf8',
		timeout: 60000,
		stdio: ['pipe', 'pipe', 'pipe'],
		env: { ANDROID_HOME: process.env.ANDROID_HOME || '' },
	});
	return JSON.parse(result);
}

function getAppContainerPath(deviceId: string, packageName: string): string {
	const response = mobilecliJson(`apps path ${packageName} --device ${deviceId}`);
	expect(response.status).to.equal('ok');
	return response.data.path;
}

function fsList(deviceId: string, remotePath: string): any[] {
	const response = mobilecliJson(`fs ls --device ${deviceId} "${remotePath}"`);
	expect(response.status).to.equal('ok');
	return response.data;
}

function fsPush(deviceId: string, localPath: string, remotePath: string): void {
	mobilecli(`fs push --device ${deviceId} "${localPath}" "${remotePath}"`);
}

function fsPull(deviceId: string, remotePath: string, localPath: string): void {
	mobilecli(`fs pull --device ${deviceId} "${remotePath}" "${localPath}"`);
}

function fsMkdir(deviceId: string, remotePath: string, parents: boolean): void {
	const flag = parents ? '-p ' : '';
	mobilecli(`fs mkdir --device ${deviceId} ${flag}"${remotePath}"`);
}

function fsRm(deviceId: string, remotePath: string, recursive: boolean): void {
	const flag = recursive ? '-r ' : '';
	mobilecli(`fs rm --device ${deviceId} ${flag}"${remotePath}"`);
}

function writeTempFile(content: string): string {
	const tmpPath = path.join(os.tmpdir(), `mobilecli-push-${Date.now()}.txt`);
	fs.writeFileSync(tmpPath, content, 'utf8');
	return tmpPath;
}
