import {expect} from 'chai';
import {execFileSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import {findRunningEmulatorByName} from './avdctl';
import {mkdirSync} from "fs";

const EMULATOR_NAME = 'mobilecli-test-emu';

describe('Android Emulator Tests', () => {
	let deviceId: string;

	before(function () {
		this.timeout(30000);

		const id = findRunningEmulatorByName(EMULATOR_NAME);
		if (!id) {
			console.log(`No running "${EMULATOR_NAME}" emulator found, skipping Android Emulator tests`);
			console.log('Create and launch one with:');
			console.log('  avdmanager create avd -n "mobilecli-test-emu" -k "system-images;android-36;google_apis_playstore;arm64-v8a" -d "pixel_9"');
			console.log('  emulator -avd mobilecli-test-emu &');
			this.skip();
			return;
		}

		deviceId = id;
	});

	it('should take screenshot', async function () {
		this.timeout(180000);

		const screenshotPath = `/tmp/screenshot-emu-${Date.now()}.png`;

		takeScreenshot(deviceId, screenshotPath);
		verifyScreenshotFileWasCreated(screenshotPath);
		verifyScreenshotFileHasValidContent(screenshotPath);
	});

	it('should open URL https://example.com', async function () {
		this.timeout(180000);

		openUrl(deviceId, 'https://example.com');
	});

	it('should get device info', async function () {
		this.timeout(60000);

		const info = getDeviceInfo(deviceId);
		verifyDeviceInfo(info, deviceId);
	});
});

const createCoverageDirectory = (): string => {
	const dir = path.join(__dirname, "coverage");
	mkdirSync(dir, {recursive: true});
	return dir;
}

function mobilecli(args: string[]): any {
	const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');

	try {
		const coverdata = createCoverageDirectory();
		const result = execFileSync(mobilecliBinary, [...args, '--verbose'], {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
			env: {
				...process.env,
				"GOCOVERDIR": coverdata,
			}
		});
		return JSON.parse(result);
	} catch (error: any) {
		console.log(`Command failed: ${mobilecliBinary} ${JSON.stringify(args)}`);
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
	mobilecli(['screenshot', '--device', deviceId, '--format', 'png', '--output', screenshotPath]);
}

function verifyScreenshotFileWasCreated(screenshotPath: string): void {
	const fileExists = fs.existsSync(screenshotPath);
	expect(fileExists).to.be.true;
}

function verifyScreenshotFileHasValidContent(screenshotPath: string): void {
	const stats = fs.statSync(screenshotPath);
	const fileSizeInBytes = stats.size;
	expect(fileSizeInBytes).to.be.greaterThan(64 * 1024);
}

function openUrl(deviceId: string, url: string): void {
	mobilecli(['url', url, '--device', deviceId]);
}

function getDeviceInfo(deviceId: string): any {
	return mobilecli(['device', 'info', '--device', deviceId]);
}

function verifyDeviceInfo(info: any, deviceId: string): void {
	expect(info.data.device.id).to.equal(deviceId);
	expect(info.data.device.platform).to.equal('android');
	expect(info.data.device.type).to.equal('emulator');
	expect(info.data.device.state).to.equal('online');
}
