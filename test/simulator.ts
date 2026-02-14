import {expect} from 'chai';
import {execFileSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import {
	findSimulatorByName,
	bootSimulator,
	waitForSimulatorReady,
} from './simctl';
import {mkdirSync} from "fs";
import {UIElement, UIDumpResponse, DeviceInfoResponse, ForegroundAppResponse} from './types';

const SIMULATOR_NAME = 'mobilecli-test-sim';

describe('iOS Simulator Tests', () => {
	let simulatorId: string;

	before(function () {
		this.timeout(180000);

		const sim = findSimulatorByName(SIMULATOR_NAME);
		if (!sim) {
			console.log(`No "${SIMULATOR_NAME}" simulator found, skipping iOS Simulator tests`);
			console.log('Create one with: xcrun simctl create "mobilecli-test-sim" "iPhone 16" com.apple.CoreSimulator.SimRuntime.iOS-26-0');
			this.skip();
			return;
		}

		simulatorId = sim.udid;

		if (sim.state !== 'Booted') {
			try {
				bootSimulator(simulatorId);
				waitForSimulatorReady(simulatorId);
			} catch (error) {
				console.log(`Failed to boot simulator "${SIMULATOR_NAME}": ${error}`);
				this.skip();
				return;
			}
		}
	});

	it('should take screenshot', async function () {
		const screenshotPath = `/tmp/screenshot-sim-${Date.now()}.png`;

		takeScreenshot(simulatorId, screenshotPath);
		verifyScreenshotFileWasCreated(screenshotPath);
		verifyScreenshotFileHasValidContent(screenshotPath);
	});

	it('should open URL https://example.com', async function () {
		openUrl(simulatorId, 'https://example.com');
	});

	it('should list all devices', async function () {
		const devices = listDevices(false);
		verifyDeviceListContainsSimulator(devices, simulatorId);
	});

	it('should get device info', async function () {
		const info = getDeviceInfo(simulatorId);
		verifyDeviceInfo(info, simulatorId);
	});

	it('should list installed apps', async function () {
		const apps = listApps(simulatorId);
		verifyAppsListContainsSafari(apps);
	});

	it('should warm up WDA by checking foreground app', async function () {
		this.timeout(300000);

		try {
			terminateApp(simulatorId, 'com.apple.mobilesafari');
			await new Promise(resolve => setTimeout(resolve, 2000));
		} catch (e) {
			// Safari might not be running, that's fine
		}

		const foregroundApp = getForegroundApp(simulatorId);
		verifySpringBoardIsForeground(foregroundApp);

		await new Promise(resolve => setTimeout(resolve, 3000));
	});

	it('should launch Safari app and verify it is in foreground', async function () {
		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		const foregroundApp = getForegroundApp(simulatorId);
		verifySafariIsForeground(foregroundApp);
	});

	it('should terminate Safari app and verify SpringBoard is in foreground', async function () {
		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		terminateApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 3000));

		const foregroundApp = getForegroundApp(simulatorId);
		verifySpringBoardIsForeground(foregroundApp);
	});

	it('should handle launching app twice (idempotency)', async function () {
		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 3000));

		const foregroundApp = getForegroundApp(simulatorId);
		verifySafariIsForeground(foregroundApp);
	});

	it('should handle launch-terminate-launch cycle', async function () {
		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		terminateApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 3000));

		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		const foregroundApp = getForegroundApp(simulatorId);
		verifySafariIsForeground(foregroundApp);
	});

	it('should tap on General button in Settings and navigate to General settings', async function () {
		launchApp(simulatorId, 'com.apple.Preferences');
		await new Promise(resolve => setTimeout(resolve, 5000));

		const uiDump = dumpUI(simulatorId);
		const generalElement = findElementByName(uiDump, 'General');

		const centerX = generalElement.rect.x + Math.floor(generalElement.rect.width / 2);
		const centerY = generalElement.rect.y + Math.floor(generalElement.rect.height / 2);

		tap(simulatorId, centerX, centerY);
		await new Promise(resolve => setTimeout(resolve, 3000));

		const generalUiDump = dumpUI(simulatorId);
		verifyElementExists(generalUiDump, 'About');
	});

	it('should press HOME button and return to home screen from Safari', async function () {
		launchApp(simulatorId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		const foregroundApp = getForegroundApp(simulatorId);
		verifySafariIsForeground(foregroundApp);

		pressButton(simulatorId, 'HOME');
		await new Promise(resolve => setTimeout(resolve, 3000));

		const foregroundAfterHome = getForegroundApp(simulatorId);
		verifySpringBoardIsForeground(foregroundAfterHome);
	});

	it('should test device lifecycle: boot, reboot, shutdown', async function () {
		this.timeout(180000);

		shutdownDevice(simulatorId);
		await new Promise(resolve => setTimeout(resolve, 3000));

		const offlineDevices = listDevices(true);
		verifyDeviceIsOffline(offlineDevices, simulatorId);

		bootDevice(simulatorId);
		await new Promise(resolve => setTimeout(resolve, 5000));

		const devicesAfterBoot = listDevices(false);
		verifyDeviceIsOnline(devicesAfterBoot, simulatorId);

		rebootDevice(simulatorId);
		await new Promise(resolve => setTimeout(resolve, 2000));

		const devicesDuringReboot = listDevices(true);
		verifyDeviceExists(devicesDuringReboot, simulatorId);

		await new Promise(resolve => setTimeout(resolve, 15000));

		const devicesAfterReboot = listDevices(false);
		verifyDeviceIsOnline(devicesAfterReboot, simulatorId);

		shutdownDevice(simulatorId);
		await new Promise(resolve => setTimeout(resolve, 3000));

		const devicesAfterShutdown = listDevices(true);
		verifyDeviceIsOffline(devicesAfterShutdown, simulatorId);

		bootDevice(simulatorId);
		await new Promise(resolve => setTimeout(resolve, 5000));
	});

	it('should dump UI source in raw format', async function () {
		this.timeout(60000);

		const foregroundApp = getForegroundApp(simulatorId);
		expect(foregroundApp.data.packageName).to.not.be.empty;

		const rawDump = dumpUIRaw(simulatorId);
		verifyRawWDADump(rawDump);
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

function takeScreenshot(simulatorId: string, screenshotPath: string): void {
	mobilecli(['screenshot', '--device', simulatorId, '--format', 'png', '--output', screenshotPath]);
}

function verifyScreenshotFileWasCreated(screenshotPath: string): void {
	const fileExists = fs.existsSync(screenshotPath);
	expect(fileExists).to.be.true;
}

function verifyScreenshotFileHasValidContent(screenshotPath: string): void {
	const stats = fs.statSync(screenshotPath);
	const fileSizeInBytes = stats.size;

	expect(fileSizeInBytes).to.be.greaterThan(100 * 1024);
}

function openUrl(simulatorId: string, url: string): void {
	mobilecli(['url', url, '--device', simulatorId]);
}

function listDevices(includeOffline: boolean): any {
	const args = ['devices'];
	if (includeOffline) {
		args.push('--include-offline');
	}

	return mobilecli(args);
}

function verifyDeviceListContainsSimulator(response: any, simulatorId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).to.include(simulatorId);
}

function getDeviceInfo(simulatorId: string): DeviceInfoResponse {
	return mobilecli(['device', 'info', '--device', simulatorId]);
}

function verifyDeviceInfo(info: DeviceInfoResponse, simulatorId: string): void {
	expect(info.data.device.id).to.equal(simulatorId);
	expect(info.data.device.platform).to.equal('ios');
	expect(info.data.device.type).to.equal('simulator');
	expect(info.data.device.state).to.equal('online');
}

function listApps(simulatorId: string): any {
	return mobilecli(['apps', 'list', '--device', simulatorId]);
}

function verifyAppsListContainsSafari(response: any): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).to.include('com.apple.mobilesafari');
}

function launchApp(simulatorId: string, packageName: string): void {
	mobilecli(['apps', 'launch', '--device', simulatorId, packageName]);
}

function terminateApp(simulatorId: string, packageName: string): void {
	mobilecli(['apps', 'terminate', '--device', simulatorId, packageName]);
}

function getForegroundApp(simulatorId: string): ForegroundAppResponse {
	return mobilecli(['apps', 'foreground', '--device', simulatorId]);
}

function verifySafariIsForeground(foregroundApp: ForegroundAppResponse): void {
	expect(foregroundApp.data.packageName).to.equal('com.apple.mobilesafari');
	expect(foregroundApp.data.appName).to.equal('Safari');
}

function verifySpringBoardIsForeground(foregroundApp: ForegroundAppResponse): void {
	expect(foregroundApp.data.packageName).to.equal('com.apple.springboard');
}

function dumpUI(simulatorId: string): UIDumpResponse {
	const response = mobilecli(['dump', 'ui', '--device', simulatorId]);

	if (!response?.data?.elements) {
		console.log('Unexpected dump UI response:', JSON.stringify(response, null, 2));
	}

	return response;
}

function findElementByName(uiDump: UIDumpResponse, name: string): UIElement {
	const elements = uiDump?.data?.elements;

	if (!elements) {
		throw new Error(`No UI elements found in response. Status: ${uiDump?.status}`);
	}

	const element = elements.find(el => el.name === name || el.label === name);

	if (!element) {
		const availableNames = elements.map(el => el.name || el.label).filter(Boolean).slice(0, 20);
		throw new Error(`Element with name "${name}" not found. Available elements: ${availableNames.join(', ')}`);
	}

	return element;
}

function tap(simulatorId: string, x: number, y: number): void {
	mobilecli(['io', 'tap', `${x},${y}`, '--device', simulatorId]);
}

function pressButton(simulatorId: string, button: string): void {
	mobilecli(['io', 'button', button, '--device', simulatorId]);
}

function verifyElementExists(uiDump: UIDumpResponse, name: string): void {
	const elements = uiDump?.data?.elements;

	if (!elements) {
		throw new Error(`No UI elements found in response. Status: ${uiDump?.status}`);
	}

	const exists = elements.some(el => el.name === name || el.label === name);

	if (!exists) {
		const availableNames = elements.map(el => el.name || el.label).filter(Boolean).slice(0, 20);
		throw new Error(`Element with name "${name}" not found. Available elements: ${availableNames.join(', ')}`);
	}
}

function bootDevice(simulatorId: string): void {
	mobilecli(['device', 'boot', '--device', simulatorId]);
}

function rebootDevice(simulatorId: string): void {
	mobilecli(['device', 'reboot', '--device', simulatorId]);
}

function shutdownDevice(simulatorId: string): void {
	mobilecli(['device', 'shutdown', '--device', simulatorId]);
}

function verifyDeviceIsOnline(response: any, simulatorId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).to.include(simulatorId);

	const devices = response.data?.devices || [];
	const device = devices.find((d: any) => d.id === simulatorId);
	expect(device).to.not.be.undefined;
	expect(device.state).to.equal('online');
}

function verifyDeviceIsOffline(response: any, simulatorId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).to.include(simulatorId);

	const devices = response.data?.devices || [];
	const device = devices.find((d: any) => d.id === simulatorId);
	expect(device).to.not.be.undefined;
	expect(device.state).to.equal('offline');
}

function verifyDeviceExists(response: any, simulatorId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).to.include(simulatorId);

	const devices = response.data?.devices || [];
	const device = devices.find((d: any) => d.id === simulatorId);
	expect(device).to.not.be.undefined;
}

function dumpUIRaw(simulatorId: string): any {
	return mobilecli(['dump', 'ui', '--device', simulatorId, '--format', 'raw']);
}

function verifyRawWDADump(response: any): void {
	expect(response).to.not.be.undefined;
	expect(response.status).to.equal('ok');

	const data = response.data;
	expect(data).to.not.be.undefined;
	expect(data.rawData).to.not.be.undefined;

	const rawData = data.rawData;
	expect(rawData.type).to.be.a('string');
	expect(rawData.type).to.not.be.empty;

	expect(rawData.children).to.be.an('array');
}
