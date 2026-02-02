import {expect} from 'chai';
import {execFileSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import {
	createAndLaunchSimulator,
	printAllLogsFromSimulator,
	shutdownSimulator,
	deleteSimulator,
	cleanupSimulators,
	findIOSRuntime
} from './simctl';
import {randomUUID} from "node:crypto";
import {mkdirSync} from "fs";
import {UIElement, UIDumpResponse, DeviceInfoResponse, ForegroundAppResponse} from './types';

const TEST_SERVER_URL = 'http://localhost:12001';

describe('iOS Simulator Tests', () => {
	after(() => {
		cleanupSimulators();
	});

	[/*'16',*/ /*'17', '18',*/ '26'].forEach((iosVersion) => {
		describe(`iOS ${iosVersion}`, () => {
			let simulatorId: string;

			before(function () {
				this.timeout(180000);

				// Check if runtime is available
				try {
					findIOSRuntime(iosVersion);
					simulatorId = createAndLaunchSimulator(iosVersion);
				} catch (error) {
					console.log(`iOS ${iosVersion} runtime not available, skipping tests: ${error}`);
					this.skip();
				}
			});

			after(() => {
				if (simulatorId) {
					printAllLogsFromSimulator(simulatorId);
					shutdownSimulator(simulatorId);
					deleteSimulator(simulatorId);
				}
			});

			it('should take screenshot', async function () {
				const screenshotPath = `/tmp/screenshot-ios${iosVersion}-${Date.now()}.png`;

				takeScreenshot(simulatorId, screenshotPath);
				verifyScreenshotFileWasCreated(screenshotPath);
				verifyScreenshotFileHasValidContent(screenshotPath);

				// console.log(`Screenshot saved at: ${screenshotPath}`);
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
				this.timeout(300000); // 5 minutes for WDA installation

				// Terminate Safari if it's running (from previous URL test)
				try {
					terminateApp(simulatorId, 'com.apple.mobilesafari');
					await new Promise(resolve => setTimeout(resolve, 2000));
				} catch (e) {
					// Safari might not be running, that's fine
				}

				// This ensures WDA is installed and running before the Safari tests
				// Check foreground app - should be SpringBoard
				const foregroundApp = getForegroundApp(simulatorId);
				verifySpringBoardIsForeground(foregroundApp);

				// Wait a bit more to ensure WDA is fully ready
				await new Promise(resolve => setTimeout(resolve, 3000));
			});

			it('should launch Safari app and verify it is in foreground', async function () {
				launchApp(simulatorId, 'com.apple.mobilesafari');

				// Wait for Safari to fully launch
				await new Promise(resolve => setTimeout(resolve, 10000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySafariIsForeground(foregroundApp);
			});

			it('should terminate Safari app and verify SpringBoard is in foreground', async function () {
				// First launch Safari
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				// Now terminate it
				terminateApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 3000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySpringBoardIsForeground(foregroundApp);
			});

			it('should handle launching app twice (idempotency)', async function () {
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				// Launch again - should not fail
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 3000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySafariIsForeground(foregroundApp);
			});

			it('should handle launch-terminate-launch cycle', async function () {
				// Launch
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				// Terminate
				terminateApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 3000));

				// Launch again
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySafariIsForeground(foregroundApp);
			});

			it('should tap on General button in Settings and navigate to General settings', async function () {
				// Launch Settings app
				launchApp(simulatorId, 'com.apple.Preferences');
				await new Promise(resolve => setTimeout(resolve, 5000));

				// Dump UI to find General button
				const uiDump = dumpUI(simulatorId);
				const generalElement = findElementByName(uiDump, 'General');

				// Calculate center coordinates for tap
				const centerX = generalElement.rect.x + Math.floor(generalElement.rect.width / 2);
				const centerY = generalElement.rect.y + Math.floor(generalElement.rect.height / 2);

				// Tap on General button
				tap(simulatorId, centerX, centerY);
				await new Promise(resolve => setTimeout(resolve, 3000));

				// Verify we're in General settings by checking for About element
				const generalUiDump = dumpUI(simulatorId);
				verifyElementExists(generalUiDump, 'About');
			});

			it('should press HOME button and return to home screen from Safari', async function () {
				// Launch Safari
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				// Verify Safari is in foreground
				const foregroundApp = getForegroundApp(simulatorId);
				verifySafariIsForeground(foregroundApp);

				// Press HOME button
				pressButton(simulatorId, 'HOME');
				await new Promise(resolve => setTimeout(resolve, 3000));

				// Verify SpringBoard (home screen) is now in foreground
				const foregroundAfterHome = getForegroundApp(simulatorId);
				verifySpringBoardIsForeground(foregroundAfterHome);
			});
		});
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
	// console.log(`✓ Screenshot file was created: ${screenshotPath}`);
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

	// Debug: log the response if elements are missing
	if (!response?.data?.elements) {
		console.log('Unexpected dump UI response:', JSON.stringify(response, null, 2));
	}

	return response;
}

function verifySafariIsRunning(uiDump: UIDumpResponse): void {
	// Safari can show either:
	// 1. Home screen with Favorites, Privacy Report, Reading List
	// 2. A web page (if it was previously viewing one)
	const elements = uiDump?.data?.elements;

	if (!elements) {
		throw new Error(`No UI elements found in response. Status: ${uiDump?.status}, Response: ${JSON.stringify(uiDump)}`);
	}

	// Debug: log some labels
	const labels = elements.map(el => el.label || el.name).filter(Boolean).slice(0, 10);
	console.log(`Found ${elements.length} UI elements. First 10 labels:`, labels);

	// Check for Safari home screen elements OR Safari-specific UI elements
	const hasSafariHomeElements = elements.some(el =>
		el.label === 'Favorites' ||
		el.label === 'Privacy Report' ||
		el.label === 'Reading List' ||
		el.name === 'Favorites' ||
		el.name === 'Privacy Report' ||
		el.name === 'Reading List'
	);

	// Check for Safari toolbar elements that appear on any page
	const hasSafariToolbar = elements.some(el =>
		el.label === 'Address' ||
		el.label === 'Back' ||
		el.label === 'Page Menu' ||
		el.name === 'Address' ||
		el.name === 'Back' ||
		el.name === 'Page Menu'
	);

	const isSafariRunning = hasSafariHomeElements || hasSafariToolbar;
	expect(isSafariRunning, `Expected to find Safari UI elements (home screen or toolbar). Sample labels found: ${labels.join(', ')}`).to.be.true;
}

function verifyHomeScreenIsVisible(uiDump: UIDumpResponse): void {
	// Home screen shows app icons - just check if we have any Icon elements
	const elements = uiDump?.data?.elements;

	if (!elements) {
		throw new Error(`No UI elements found in response. Status: ${uiDump?.status}, Response: ${JSON.stringify(uiDump)}`);
	}

	// Debug: log element types
	const elementTypes = elements.map(el => el.type);
	console.log(`Found ${elements.length} UI elements with types:`, [...new Set(elementTypes)]);

	const hasIcons = elements.some(el => el.type === 'Icon');
	expect(hasIcons, `Expected to find Icon elements on home screen, but found types: ${[...new Set(elementTypes)].join(', ')}`).to.be.true;
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

