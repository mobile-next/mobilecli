import { expect } from 'chai';
import { execSync } from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
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
