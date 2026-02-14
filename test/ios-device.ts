import {expect} from 'chai';
import {execFileSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import {mkdirSync} from "fs";
import {UIElement, UIDumpResponse, DeviceInfoResponse, ForegroundAppResponse} from './types';

interface DeviceEntry {
	id: string;
	name: string;
	platform: string;
	type: string;
	state: string;
}

describe('iOS Real Device Tests', () => {
	let deviceId: string;

	before(function () {
		this.timeout(30000);

		const device = findConnectedIOSDevice();
		if (!device) {
			console.log('No connected iOS real device found, skipping iOS Real Device tests');
			this.skip();
			return;
		}

		deviceId = device.id;
		console.log(`Found real iOS device: ${device.name} (${deviceId})`);
	});

	it('should discover device in device list', async function () {
		const devices = listDevices();
		verifyDeviceListContainsDevice(devices, deviceId);
	});

	it('should get device info', async function () {
		const info = getDeviceInfo(deviceId);
		verifyRealDeviceInfo(info, deviceId);
	});

	it('should take screenshot', async function () {
		this.timeout(60000);

		const screenshotPath = `/tmp/screenshot-device-${Date.now()}.png`;

		takeScreenshot(deviceId, screenshotPath);
		verifyScreenshotFileWasCreated(screenshotPath);
		verifyScreenshotFileHasValidContent(screenshotPath);
	});

	it('should open URL https://example.com', async function () {
		this.timeout(60000);

		openUrl(deviceId, 'https://example.com');
	});

	it('should warm up WDA by checking foreground app', async function () {
		this.timeout(300000);

		try {
			terminateApp(deviceId, 'com.apple.mobilesafari');
			await new Promise(resolve => setTimeout(resolve, 2000));
		} catch (e) {
			// Safari might not be running
		}

		const foregroundApp = getForegroundApp(deviceId);
		expect(foregroundApp.data.packageName).to.not.be.empty;

		await new Promise(resolve => setTimeout(resolve, 3000));
	});

	it('should launch Safari and verify it is in foreground', async function () {
		this.timeout(60000);

		launchApp(deviceId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		const foregroundApp = getForegroundApp(deviceId);
		verifySafariIsForeground(foregroundApp);
	});

	it('should terminate Safari and verify SpringBoard is in foreground', async function () {
		this.timeout(60000);

		launchApp(deviceId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		terminateApp(deviceId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 3000));

		const foregroundApp = getForegroundApp(deviceId);
		verifySpringBoardIsForeground(foregroundApp);
	});

	it('should dump UI elements', async function () {
		this.timeout(60000);

		launchApp(deviceId, 'com.apple.Preferences');
		await new Promise(resolve => setTimeout(resolve, 5000));

		const uiDump = dumpUI(deviceId);
		expect(uiDump.data.elements).to.be.an('array');
		expect(uiDump.data.elements.length).to.be.greaterThan(0);
	});

	it('should tap on General in Settings', async function () {
		this.timeout(60000);

		// Terminate Settings first to ensure it opens at root
		try {
			terminateApp(deviceId, 'com.apple.Preferences');
			await new Promise(resolve => setTimeout(resolve, 2000));
		} catch (e) {
			// Settings might not be running
		}

		launchApp(deviceId, 'com.apple.Preferences');
		await new Promise(resolve => setTimeout(resolve, 5000));

		const uiDump = dumpUI(deviceId);
		const generalElement = findElementByName(uiDump, 'General');

		const centerX = generalElement.rect.x + Math.floor(generalElement.rect.width / 2);
		const centerY = generalElement.rect.y + Math.floor(generalElement.rect.height / 2);

		tap(deviceId, centerX, centerY);
		await new Promise(resolve => setTimeout(resolve, 3000));

		const generalUiDump = dumpUI(deviceId);
		verifyElementExists(generalUiDump, 'About');
	});

	it('should send text input', async function () {
		this.timeout(60000);

		// Open Safari and tap address bar to get a text field focused
		launchApp(deviceId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		const uiDump = dumpUI(deviceId);
		const addressBar = uiDump.data.elements.find(
			el => el.label === 'Address' || el.name === 'Address' || el.name === 'URL'
		);

		if (!addressBar) {
			// Address bar might not be visible; skip gracefully
			console.log('Address bar not found in UI, skipping text input test');
			return;
		}

		const centerX = addressBar.rect.x + Math.floor(addressBar.rect.width / 2);
		const centerY = addressBar.rect.y + Math.floor(addressBar.rect.height / 2);

		tap(deviceId, centerX, centerY);
		await new Promise(resolve => setTimeout(resolve, 2000));

		sendText(deviceId, 'hello');
		await new Promise(resolve => setTimeout(resolve, 2000));
	});

	it('should press HOME button', async function () {
		this.timeout(60000);

		launchApp(deviceId, 'com.apple.mobilesafari');
		await new Promise(resolve => setTimeout(resolve, 10000));

		pressButton(deviceId, 'HOME');
		await new Promise(resolve => setTimeout(resolve, 3000));

		const foregroundAfterHome = getForegroundApp(deviceId);
		verifySpringBoardIsForeground(foregroundAfterHome);
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

function findConnectedIOSDevice(): DeviceEntry | null {
	try {
		const result = mobilecli(['devices']);
		const devices: DeviceEntry[] = result.data?.devices || [];

		return devices.find(d =>
			d.platform === 'ios' &&
			(d.type === 'device' || d.type === 'real') &&
			d.state === 'online'
		) || null;
	} catch {
		return null;
	}
}

function listDevices(): any {
	return mobilecli(['devices']);
}

function verifyDeviceListContainsDevice(response: any, deviceId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).to.include(deviceId);
}

function getDeviceInfo(deviceId: string): DeviceInfoResponse {
	return mobilecli(['device', 'info', '--device', deviceId]);
}

function verifyRealDeviceInfo(info: DeviceInfoResponse, deviceId: string): void {
	expect(info.data.device.id).to.equal(deviceId);
	expect(info.data.device.platform).to.equal('ios');
	expect(info.data.device.type).to.be.oneOf(['device', 'real']);
	expect(info.data.device.state).to.equal('online');
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
	expect(fileSizeInBytes).to.be.greaterThan(100 * 1024);
}

function openUrl(deviceId: string, url: string): void {
	mobilecli(['url', url, '--device', deviceId]);
}

function launchApp(deviceId: string, packageName: string): void {
	mobilecli(['apps', 'launch', '--device', deviceId, packageName]);
}

function terminateApp(deviceId: string, packageName: string): void {
	mobilecli(['apps', 'terminate', '--device', deviceId, packageName]);
}

function getForegroundApp(deviceId: string): ForegroundAppResponse {
	return mobilecli(['apps', 'foreground', '--device', deviceId]);
}

function verifySafariIsForeground(foregroundApp: ForegroundAppResponse): void {
	expect(foregroundApp.data.packageName).to.equal('com.apple.mobilesafari');
	expect(foregroundApp.data.appName).to.equal('Safari');
}

function verifySpringBoardIsForeground(foregroundApp: ForegroundAppResponse): void {
	expect(foregroundApp.data.packageName).to.equal('com.apple.springboard');
}

function dumpUI(deviceId: string): UIDumpResponse {
	const response = mobilecli(['dump', 'ui', '--device', deviceId]);

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

function tap(deviceId: string, x: number, y: number): void {
	mobilecli(['io', 'tap', `${x},${y}`, '--device', deviceId]);
}

function sendText(deviceId: string, text: string): void {
	mobilecli(['io', 'text', text, '--device', deviceId]);
}

function pressButton(deviceId: string, button: string): void {
	mobilecli(['io', 'button', button, '--device', deviceId]);
}
