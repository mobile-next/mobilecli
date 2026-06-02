import {test, expect} from '@playwright/test';
import {execFileSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import {
	findSimulatorByName,
	printAllLogsFromSimulator,
	shutdownSimulator,
} from './simctl';
import {randomUUID} from "node:crypto";
import {mkdirSync} from "fs";
import type {UIElement, UIDumpResponse, DeviceInfoResponse, ForegroundAppResponse} from './types';

const TEST_SERVER_URL = 'http://localhost:12001';

test.describe('iOS Simulator Tests', () => {
	[/*'16',*/ /*'17', '18',*/ '26'].forEach((iosVersion) => {
		test.describe(`iOS ${iosVersion}`, () => {
			let simulatorId: string;

			test.beforeAll(() => {
				const simulatorName = `Test-iOS-${iosVersion}`;
				try {
					simulatorId = findSimulatorByName(simulatorName);
					installDeviceKitAgent(simulatorId);
				} catch (error) {
					console.log(`Simulator "${simulatorName}" not found, skipping tests: ${error}`);
				}
			});

			test.afterAll(() => {
				if (simulatorId) {
					printAllLogsFromSimulator(simulatorId);
				}
			});

			test('should take screenshot', async () => {
				test.skip(!simulatorId, 'simulator not found');

				const screenshotPath = `/tmp/screenshot-ios${iosVersion}-${Date.now()}.png`;

				takeScreenshot(simulatorId, screenshotPath);
				verifyScreenshotFileWasCreated(screenshotPath);
				verifyScreenshotFileHasValidContent(screenshotPath);

				// console.log(`Screenshot saved at: ${screenshotPath}`);
			});

			test('should open URL https://example.com', async () => {
				test.skip(!simulatorId, 'simulator not found');

				openUrl(simulatorId, 'https://example.com');
			});

			test('should list all devices', async () => {
				test.skip(!simulatorId, 'simulator not found');

				const devices = listDevices(false);
				verifyDeviceListContainsSimulator(devices, simulatorId);
			});

			test('should get device info', async () => {
				test.skip(!simulatorId, 'simulator not found');

				const info = getDeviceInfo(simulatorId);
				verifyDeviceInfo(info, simulatorId);
			});

			test('should list installed apps', async () => {
				test.skip(!simulatorId, 'simulator not found');

				const apps = listApps(simulatorId);
				verifyAppsListContainsSafari(apps);
			});

			test('should launch Safari app and verify it is in foreground', async () => {
				test.skip(!simulatorId, 'simulator not found');

				launchApp(simulatorId, 'com.apple.mobilesafari');

				// Wait for Safari to fully launch
				await new Promise(resolve => setTimeout(resolve, 10000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySafariIsForeground(foregroundApp);
			});

			test('should terminate Safari app and verify SpringBoard is in foreground', async () => {
				test.skip(!simulatorId, 'simulator not found');

				// First launch Safari
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				// Now terminate it
				terminateApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 3000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySpringBoardIsForeground(foregroundApp);
			});

			test('should handle launching app twice (idempotency)', async () => {
				test.skip(!simulatorId, 'simulator not found');

				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 10000));

				// Launch again - should not fail
				launchApp(simulatorId, 'com.apple.mobilesafari');
				await new Promise(resolve => setTimeout(resolve, 3000));

				const foregroundApp = getForegroundApp(simulatorId);
				verifySafariIsForeground(foregroundApp);
			});

			test('should handle launch-terminate-launch cycle', async () => {
				test.skip(!simulatorId, 'simulator not found');

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

			test('should tap on General button in Settings and navigate to General settings', async () => {
				test.skip(!simulatorId, 'simulator not found');

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

			test('should press HOME button and return to home screen from Safari', async () => {
				test.skip(!simulatorId, 'simulator not found');

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

			test.skip('should test device lifecycle: boot, reboot, shutdown', async () => {
				// shutdown simulator using simctl to get it offline
				shutdownSimulator(simulatorId);
				await new Promise(resolve => setTimeout(resolve, 3000));

				// list offline devices - verify simulator is there and offline
				const offlineDevices = listDevices(true);
				verifyDeviceIsOffline(offlineDevices, simulatorId);

				// boot the simulator using mobilecli
				bootDevice(simulatorId);
				await new Promise(resolve => setTimeout(resolve, 5000));

				// verify simulator is now online
				const devicesAfterBoot = listDevices(false);
				verifyDeviceIsOnline(devicesAfterBoot, simulatorId);

				// reboot the simulator
				rebootDevice(simulatorId);

				// immediately check - should be offline (or at least not in the online list during reboot)
				await new Promise(resolve => setTimeout(resolve, 2000));
				const devicesDuringReboot = listDevices(true);
				// during reboot, state might be "Booting" or "Shutdown"
				// we just verify it exists in the full list
				verifyDeviceExists(devicesDuringReboot, simulatorId);

				// wait a bit more for reboot to complete
				await new Promise(resolve => setTimeout(resolve, 15000));

				// verify simulator came back online
				const devicesAfterReboot = listDevices(false);
				verifyDeviceIsOnline(devicesAfterReboot, simulatorId);

				// shutdown the simulator
				shutdownDevice(simulatorId);
				await new Promise(resolve => setTimeout(resolve, 3000));

				// verify simulator is offline
				const devicesAfterShutdown = listDevices(true);
				verifyDeviceIsOffline(devicesAfterShutdown, simulatorId);

				// boot it again for cleanup and other tests
				bootDevice(simulatorId);
				await new Promise(resolve => setTimeout(resolve, 5000));
			});

			test('should dump UI source in raw format', async () => {
				test.skip(!simulatorId, 'simulator not found');

				const rawDump = dumpUIRaw(simulatorId);
				verifyRawViewtreeDump(rawDump);
			});

			test.describe('fs operations on app container (com.mobilenext.playground)', () => {
				const packageName = 'com.mobilenext.playground';
				let containerPath: string;
				let remoteDir: string;
				let remoteFile: string;

				test.beforeAll(() => {
					if (!simulatorId) return;
					containerPath = getAppContainerPath(simulatorId, packageName);
					remoteDir = `${containerPath}/Documents/mobilecli-test-` + (+new Date());
					remoteFile = `${remoteDir}/data.txt`;
				});

				test('should return a valid container path for com.mobilenext.playground', async () => {
					test.skip(!simulatorId, 'simulator not found');
					expect(typeof containerPath).toBe('string');
					expect(containerPath).toMatch(/^\/Users\//);
				});

				test('should list the app container root', async () => {
					test.skip(!simulatorId, 'simulator not found');
					const entries = fsList(simulatorId, containerPath);
					expect(Array.isArray(entries)).toBe(true);
					const known = entries.filter(e => e.name === "Documents" || e.name === "Library");
					expect(known.length).toBe(2);
				});

				test('should create a directory inside the app container', async () => {
					test.skip(!simulatorId, 'simulator not found');
					fsMkdir(simulatorId, remoteDir, true);
				});

				test('should push a file into the app container', async () => {
					test.skip(!simulatorId, 'simulator not found');
					const localFile = writeTempFile('app container test');
					fsPush(simulatorId, localFile, remoteFile);
					fs.unlinkSync(localFile);
				});

				test('should list the file inside the app container', async () => {
					test.skip(!simulatorId, 'simulator not found');
					const entries = fsList(simulatorId, remoteDir);
					const names = entries.map((e: any) => e.name);
					expect(names).toContain('data.txt');
				});

				test('should pull the file from the app container and verify contents match', async () => {
					test.skip(!simulatorId, 'simulator not found');
					const localDest = path.join(os.tmpdir(), `mobilecli-pull-app-${Date.now()}.txt`);
					fsPull(simulatorId, remoteFile, localDest);
					const contents = fs.readFileSync(localDest, 'utf8');
					expect(contents.trim()).toBe('app container test');
					fs.unlinkSync(localDest);
				});

				test('should remove the test directory from the app container', async () => {
					test.skip(!simulatorId, 'simulator not found');
					fsRm(simulatorId, remoteDir, true);
					const entries = fsList(simulatorId, `${containerPath}/Documents`);
					const names = entries.map((e: any) => e.name);
					expect(names).not.toContain('mobilecli-test');
				});

				test('should prevent escaping the app container sandbox', async () => {
					test.skip(!simulatorId, 'simulator not found');
					const localDest = path.join(os.tmpdir(), `mobilecli-pull-app-${Date.now()}.txt`);
					for (let depth=1; depth<32; depth++) {
						try {
							const remoteFile = remoteDir + "/..".repeat(depth) + "/etc/hosts";
							fsPull(simulatorId, remoteFile, localDest);
						} catch {
							// ignored, expected fsPull tof ail
						}

						expect(fs.existsSync(localDest)).toBe(false);
					}
				});
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
}

function installDeviceKitAgent(simulatorId: string): void {
	mobilecli(['agent', 'install', '--device', simulatorId]);
}

function takeScreenshot(simulatorId: string, screenshotPath: string): void {
	mobilecli(['screenshot', '--device', simulatorId, '--format', 'png', '--output', screenshotPath]);
}

function verifyScreenshotFileWasCreated(screenshotPath: string): void {
	const fileExists = fs.existsSync(screenshotPath);
	expect(fileExists).toBe(true);
	// console.log(`✓ Screenshot file was created: ${screenshotPath}`);
}

function verifyScreenshotFileHasValidContent(screenshotPath: string): void {
	const stats = fs.statSync(screenshotPath);
	const fileSizeInBytes = stats.size;

	expect(fileSizeInBytes).toBeGreaterThan(100 * 1024);
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
	expect(jsonString).toContain(simulatorId);
}

function getDeviceInfo(simulatorId: string): DeviceInfoResponse {
	return mobilecli(['device', 'info', '--device', simulatorId]);
}

function verifyDeviceInfo(info: DeviceInfoResponse, simulatorId: string): void {
	expect(info.data.device.id).toBe(simulatorId);
	expect(info.data.device.platform).toBe('ios');
	expect(info.data.device.type).toBe('simulator');
	expect(info.data.device.state).toBe('online');
}

function listApps(simulatorId: string): any {
	return mobilecli(['apps', 'list', '--device', simulatorId]);
}

function verifyAppsListContainsSafari(response: any): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).toContain('com.apple.mobilesafari');
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
	expect(foregroundApp.data.packageName).toBe('com.apple.mobilesafari');
	expect(foregroundApp.data.appName).toBe('Safari');
}

function verifySpringBoardIsForeground(foregroundApp: ForegroundAppResponse): void {
	expect(foregroundApp.data.packageName).toBe('com.apple.springboard');
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
	expect(isSafariRunning, `Expected to find Safari UI elements (home screen or toolbar). Sample labels found: ${labels.join(', ')}`).toBe(true);
}

/*
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
*/

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
	expect(jsonString).toContain(simulatorId);

	// verify device has state "online"
	const devices = response.data?.devices || [];
	const device = devices.find((d: any) => d.id === simulatorId);
	expect(device).toBeDefined();
	expect(device.state).toBe('online');
}

function verifyDeviceIsOffline(response: any, simulatorId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).toContain(simulatorId);

	// verify device has state "offline"
	const devices = response.data?.devices || [];
	const device = devices.find((d: any) => d.id === simulatorId);
	expect(device).toBeDefined();
	expect(device.state).toBe('offline');
}

function verifyDeviceExists(response: any, simulatorId: string): void {
	const jsonString = JSON.stringify(response);
	expect(jsonString).toContain(simulatorId);

	// just verify device exists in the list
	const devices = response.data?.devices || [];
	const device = devices.find((d: any) => d.id === simulatorId);
	expect(device).toBeDefined();
}

function dumpUIRaw(simulatorId: string): any {
	return mobilecli(['dump', 'ui', '--device', simulatorId, '--format', 'raw']);
}

function verifyRawViewtreeDump(response: any): void {
	// verify it's a valid response
	expect(response).toBeDefined();
	expect(response.status).toBe('ok');

	// raw format returns rawData field
	const data = response.data;
	expect(data).toBeDefined();
	expect(data.rawData).toBeDefined();

	// rawData should contain the tree structure directly from WDA
	const rawData = data.rawData;
	expect(Array.isArray(rawData.children)).toBe(true);
}

function getAppContainerPath(simulatorId: string, packageName: string): string {
	const response = mobilecli(['apps', 'path', packageName, '--device', simulatorId]);
	expect(response.status).toBe('ok');
	return response.data.path;
}

function fsList(simulatorId: string, remotePath: string): any[] {
	const response = mobilecli(['fs', 'ls', '--device', simulatorId, remotePath]);
	expect(response.status).toBe('ok');
	return response.data;
}

function fsPush(simulatorId: string, localPath: string, remotePath: string): void {
	mobilecli(['fs', 'push', '--device', simulatorId, localPath, remotePath]);
}

function fsPull(simulatorId: string, remotePath: string, localPath: string): void {
	mobilecli(['fs', 'pull', '--device', simulatorId, remotePath, localPath]);
}

function fsMkdir(simulatorId: string, remotePath: string, parents: boolean): void {
	mobilecli(['fs', 'mkdir', '--device', simulatorId, ...(parents ? ['-p'] : []), remotePath]);
}

function fsRm(simulatorId: string, remotePath: string, recursive: boolean): void {
	mobilecli(['fs', 'rm', '--device', simulatorId, ...(recursive ? ['-r'] : []), remotePath]);
}

function writeTempFile(content: string): string {
	const tmpPath = path.join(os.tmpdir(), `mobilecli-push-${Date.now()}.txt`);
	fs.writeFileSync(tmpPath, content, 'utf8');
	return tmpPath;
}
