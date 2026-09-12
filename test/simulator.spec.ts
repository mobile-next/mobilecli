import {test, expect} from '@playwright/test';
import {execFileSync, spawn} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';
import {
	printAllLogsFromSimulator,
	shutdownSimulator,
} from './simctl';
import {randomUUID} from "node:crypto";
import {coverageEnv} from './coverage';
import {
	expectAppShape,
	expectDeviceShape,
	expectFsListingShape,
	expectForegroundAppShape,
	expectErrorEnvelope,
	expectInstallResultShape,
	expectInstalledAppShape,
	expectOkEnvelope,
	expectUIDumpShape,
} from './shapes';
import {
	centerOf,
	expectWebViewShape,
	expectWebViewUrlToBecome,
	findWebViewButton,
	WEBVIEW_COMMANDS_TAKING_AN_ID,
	WEBVIEW_DONE_GREETING,
	WEBVIEW_DONE_URL,
	WEBVIEW_MISSING_ID,
	WEBVIEW_SAMPLE_TITLE,
	WEBVIEW_SAMPLE_URL,
} from './webview';
import type {WebViewInfo, WebViewQueryResult} from './webview';
import {
	downloadPlayground,
	PLAYGROUND_APP_NAME,
	PLAYGROUND_APP_VERSION,
	PLAYGROUND_APP_VERSION_CODE,
	PLAYGROUND_PACKAGE,
} from './playground';
import type {AppsListResponse, InstalledApp, UIElement, UIDumpResponse, DeviceInfoResponse, ForegroundAppResponse} from './types';

type Dimensions = {
	width: number;
	height: number;
};

const TEST_SERVER_URL = 'http://localhost:12001';

// ships on every simulator image and is never a debug build
const IOS_SETTINGS_BUNDLE_ID = 'com.apple.Preferences';

test.describe('iOS Simulator Tests', () => {
	[/*'16',*/ /*'17', '18',*/ '26'].forEach((iosVersion) => {
		test.describe(`iOS ${iosVersion}`, () => {
			let simulatorId: string;

			test.beforeAll(() => {
				try {
					simulatorId = findFirstSimulatorId() ?? '';
					if (!simulatorId) {
						console.log('No booted iOS simulator found. See test/README.md for setup instructions.');
						return;
					}
					installDeviceKitAgent(simulatorId);
				} catch (error) {
					console.log(`Could not look up an iOS simulator, skipping tests: ${error}`);
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

			test.describe('screenrecord', () => {
				test('should record with --time-limit 5 and produce a playable mp4', () => {
					test.skip(!simulatorId, 'simulator not found');

					const videoPath = path.join(os.tmpdir(), `mobilecli-rec-timelimit-${Date.now()}.mp4`);
					recordScreenWithTimeLimit(simulatorId, videoPath, 5);

					verifyVideoIsPlayable(videoPath);
					verifyVideoMatchesScreenshotDimensions(videoPath, simulatorId);
					fs.unlinkSync(videoPath);
				});

				test('should record without time limit and finalize a playable mp4 on Ctrl-C', async () => {
					test.skip(!simulatorId, 'simulator not found');

					const videoPath = path.join(os.tmpdir(), `mobilecli-rec-ctrlc-${Date.now()}.mp4`);
					await recordScreenThenInterruptWithCtrlC(simulatorId, videoPath, 5);

					verifyVideoIsPlayable(videoPath);
					verifyVideoMatchesScreenshotDimensions(videoPath, simulatorId);
					fs.unlinkSync(videoPath);
				});
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

			test('should set and read back clipboard text', async () => {
				test.skip(!simulatorId, 'simulator not found');

				const text = `mobilecli-${randomUUID()}`;
				setClipboard(simulatorId, text);
				expect(getClipboard(simulatorId)).toBe(text);
			});

			test('should clear the clipboard when set to an empty string', async () => {
				test.skip(!simulatorId, 'simulator not found');

				setClipboard(simulatorId, 'not empty');
				setClipboard(simulatorId, '');
				expect(getClipboard(simulatorId)).toBe('');
			});

			test('should reject clipboard set without a text argument', async () => {
				test.skip(!simulatorId, 'simulator not found');

				expect(() => mobilecli(['io', 'clipboard', 'set', '--device', simulatorId])).toThrow();
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

			test.describe('install and uninstall playground', () => {
				// installs an app we own rather than anything on the simulator image, so
				// the uninstall half is safe. also leaves playground installed for the
				// app-container fs group below.
				let zipPath: string;

				test.beforeAll(async () => {
					zipPath = await downloadPlayground('ios');
				});

				test('should uninstall playground and no longer list it', async () => {
					test.skip(!simulatorId, 'simulator not found');

					uninstallPlaygroundIfPresent(simulatorId);
					expect(installedPackageNames(simulatorId)).not.toContain(PLAYGROUND_PACKAGE);
				});

				test('should install playground from a local zip', async () => {
					test.skip(!simulatorId, 'simulator not found');

					const result: unknown = mobilecli(['apps', 'install', zipPath, '--device', simulatorId]).data;
					expectInstallResultShape(result);
					expect(result.app.packageName).toBe(PLAYGROUND_PACKAGE);
					expect(result.app.version).toBe(PLAYGROUND_APP_VERSION);
					expect(result.app.versionCode).toBe(PLAYGROUND_APP_VERSION_CODE);
				});

				test('should list playground with every field it reports today', async () => {
					test.skip(!simulatorId, 'simulator not found');

					const app = findInstalledApp(simulatorId, PLAYGROUND_PACKAGE);
					expect(app, `${PLAYGROUND_PACKAGE} missing after install`).toBeDefined();
					expectInstalledAppShape(app);
					expect(app!.appName).toBe(PLAYGROUND_APP_NAME);
					expect(app!.version).toBe(PLAYGROUND_APP_VERSION);
					expect(app!.versionCode).toBe(PLAYGROUND_APP_VERSION_CODE);
				});
			});

			test.describe('webview', () => {
				// the playground webview screen is the one embedded webview we control on
				// both platforms. a real handset would be left on an arbitrary screen.
				let webViewId: string;

				test.beforeAll(async () => {
					if (!simulatorId) return;
					await openPlaygroundWebViewScreen(simulatorId);
					await sleep(3000);
					webViewId = firstWebView(simulatorId).id;

					// the app restores whatever url the webview last showed, so start every
					// run from the sample page instead of inheriting the previous run's state
					webViewGoto(simulatorId, webViewId, WEBVIEW_SAMPLE_URL);
					webViewWait(simulatorId, webViewId, 'load');
				});

				test('should list the playground webview', () => {
					test.skip(!simulatorId, 'simulator not found');

					const webView = firstWebView(simulatorId);
					expect(webView.url).toBe(WEBVIEW_SAMPLE_URL);
					expect(webView.title).toBe(WEBVIEW_SAMPLE_TITLE);
					// ios reports no owning bundle for an inspected webview
					expect(webView.bundleId).toBe('');
					expect(webView.isVisible).toBe(true);
				});

				test('should report the url and title of the webview', () => {
					test.skip(!simulatorId, 'simulator not found');

					expect(webViewUrl(simulatorId, webViewId)).toBe(WEBVIEW_SAMPLE_URL);
					expect(webViewTitle(simulatorId, webViewId)).toBe(WEBVIEW_SAMPLE_TITLE);
				});

				test('should evaluate javascript inside the webview', () => {
					test.skip(!simulatorId, 'simulator not found');

					expect(webViewEval(simulatorId, webViewId, 'document.title')).toBe(WEBVIEW_SAMPLE_TITLE);
				});

				test('should dump the html content of the webview', () => {
					test.skip(!simulatorId, 'simulator not found');

					const html = webViewContent(simulatorId, webViewId);
					expect(html).toContain('<form id="loginForm"');
					expect(html).toContain(WEBVIEW_SAMPLE_TITLE);
				});

				test('should query dom elements by css selector', () => {
					test.skip(!simulatorId, 'simulator not found');

					const inputs = webViewQuery(simulatorId, webViewId, 'input#name');
					expect(inputs.length).toBe(1);
					expect(inputs[0].tag).toBe('input');
					expect(inputs[0].id).toBe('name');
				});

				test('should wait for the webview to finish loading', () => {
					test.skip(!simulatorId, 'simulator not found');

					webViewWait(simulatorId, webViewId, 'domcontentloaded');
					webViewWait(simulatorId, webViewId, 'load');
				});

				test('should navigate the webview to another url', async () => {
					test.skip(!simulatorId, 'simulator not found');

					webViewGoto(simulatorId, webViewId, WEBVIEW_DONE_URL);
					webViewWait(simulatorId, webViewId, 'load');

					await expectWebViewUrlToBecome(() => webViewUrl(simulatorId, webViewId), WEBVIEW_DONE_URL);
					expect(webViewQuery(simulatorId, webViewId, 'h1')[0].text).toBe(WEBVIEW_DONE_GREETING);
				});

				test('should go back to the page it navigated away from', async () => {
					test.skip(!simulatorId, 'simulator not found');

					webViewGoBack(simulatorId, webViewId);
					await sleep(2000);

					await expectWebViewUrlToBecome(() => webViewUrl(simulatorId, webViewId), WEBVIEW_SAMPLE_URL);
				});

				test('should go forward again', async () => {
					test.skip(!simulatorId, 'simulator not found');

					webViewGoForward(simulatorId, webViewId);
					await sleep(2000);

					await expectWebViewUrlToBecome(() => webViewUrl(simulatorId, webViewId), WEBVIEW_DONE_URL);
				});

				test('should report an error for every command given an unknown webview id', () => {
					test.skip(!simulatorId, 'simulator not found');

					for (const [subcommand, ...args] of WEBVIEW_COMMANDS_TAKING_AN_ID) {
						const message = webViewCommandError(simulatorId, [subcommand, WEBVIEW_MISSING_ID, ...args]);
						expect(message, `${subcommand} accepted an unknown webview id`).toContain(WEBVIEW_MISSING_ID);
					}
				});

				test('should report an error when the device does not exist', () => {
					test.skip(!simulatorId, 'simulator not found');

					const message = webViewCommandError('no-such-device', ['list']);
					expect(message).toContain('error finding device');
				});

				test('should reload the webview and stay on the same url', async () => {
					test.skip(!simulatorId, 'simulator not found');

					webViewReload(simulatorId, webViewId);
					webViewWait(simulatorId, webViewId, 'load');

					await expectWebViewUrlToBecome(() => webViewUrl(simulatorId, webViewId), WEBVIEW_DONE_URL);
				});

			});

			// its own describe, not a test inside the playground group above: `webview list`
			// reads the foreground app, so this launches a different app and would break the
			// shared state the playground tests set up once in their beforeAll
			test.describe('webview on an app that cannot be inspected', () => {
				test.beforeAll(async () => {
					if (!simulatorId) return;
					launchApp(simulatorId, IOS_SETTINGS_BUNDLE_ID);
					await sleep(3000);
				});

				test('should fail to list webviews in an app that is not debuggable', () => {
					test.skip(!simulatorId, 'simulator not found');

					const message = webViewCommandError(simulatorId, ['list']);
					expect(message).toContain('webview list failed');
					expect(message).toContain(IOS_SETTINGS_BUNDLE_ID);
				});
			});

			test.describe('fs operations on app container (com.mobilenext.playground)', () => {
				const packageName = PLAYGROUND_PACKAGE;
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

function mobilecli(args: string[]): any {
	const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');

	const result = execFileSync(mobilecliBinary, [...args, '--verbose'], {
		encoding: 'utf8',
		timeout: 180000,
		stdio: ['pipe', 'pipe', 'pipe'],
		env: coverageEnv(),
	});

	// every command routed through here answers with a json envelope; asserting it
	// centrally means a change to the output format fails these tests instead of
	// slipping past whichever test only reads its own payload field
	const parsed = JSON.parse(result);
	expectOkEnvelope(parsed);
	return parsed;
}

// asks mobilecli rather than simctl so the tests exercise the same discovery path
// users do. `devices` omits offline entries unless --include-offline is passed, so
// this is the first *booted* simulator; nothing booted means the tests skip rather
// than fail partway through. --type simulator keeps a plugged-in iphone out of it.
function findFirstSimulatorId(): string | null {
	const response = mobilecli(['devices', '--platform', 'ios', '--type', 'simulator']);
	return response.data.devices[0]?.id ?? null;
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
	response.data.devices.forEach(expectDeviceShape);
	expect(response.data.devices.map((d: any) => d.id)).toContain(simulatorId);
}

function getDeviceInfo(simulatorId: string): DeviceInfoResponse {
	return mobilecli(['device', 'info', '--device', simulatorId]);
}

function verifyDeviceInfo(info: DeviceInfoResponse, simulatorId: string): void {
	expectDeviceShape(info.data.device);
	expect(info.data.device.id).toBe(simulatorId);
	expect(info.data.device.platform).toBe('ios');
	expect(info.data.device.type).toBe('simulator');
	expect(info.data.device.state).toBe('online');
}

function listApps(simulatorId: string): AppsListResponse {
	return mobilecli(['apps', 'list', '--device', simulatorId]) as AppsListResponse;
}

function verifyAppsListContainsSafari(response: AppsListResponse): void {
	response.data.forEach(expectAppShape);
	expect(response.data.map(app => app.packageName)).toContain('com.apple.mobilesafari');
}

function installedApps(simulatorId: string): InstalledApp[] {
	return listApps(simulatorId).data as InstalledApp[];
}

function installedPackageNames(simulatorId: string): string[] {
	return installedApps(simulatorId).map(app => app.packageName);
}

function findInstalledApp(simulatorId: string, packageName: string): InstalledApp | undefined {
	return installedApps(simulatorId).find(app => app.packageName === packageName);
}

// uninstalling an app that is not installed is not an error worth failing on: the
// point of this call is only to reach a known-clean starting state
function uninstallPlaygroundIfPresent(simulatorId: string): void {
	try {
		mobilecli(['apps', 'uninstall', PLAYGROUND_PACKAGE, '--device', simulatorId]);
	} catch {
		// already absent
	}
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
	expectForegroundAppShape(foregroundApp.data);
	expect(foregroundApp.data.packageName).toBe('com.apple.mobilesafari');
	expect(foregroundApp.data.appName).toBe('Safari');
}

function verifySpringBoardIsForeground(foregroundApp: ForegroundAppResponse): void {
	expectForegroundAppShape(foregroundApp.data);
	expect(foregroundApp.data.packageName).toBe('com.apple.springboard');
}

function dumpUI(simulatorId: string): UIDumpResponse {
	const response = mobilecli(['dump', 'ui', '--device', simulatorId]);
	expectUIDumpShape(response.data.elements);
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

function setClipboard(simulatorId: string, text: string): void {
	mobilecli(['io', 'clipboard', 'set', text, '--device', simulatorId]);
}

function getClipboard(simulatorId: string): string {
	const response = mobilecli(['io', 'clipboard', 'get', '--device', simulatorId]);
	return response.data.text;
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

async function openPlaygroundWebViewScreen(simulatorId: string): Promise<void> {
	launchApp(simulatorId, PLAYGROUND_PACKAGE);
	await sleep(3000);
	const button = findWebViewButton(dumpUI(simulatorId));
	tap(simulatorId, centerOf(button).x, centerOf(button).y);
}

// runs a webview command that is expected to fail and returns the error message
function webViewCommandError(simulatorId: string, args: string[]): string {
	try {
		mobilecli(['webview', ...args, '--device', simulatorId]);
	} catch (error: unknown) {
		const stdout = (error as {stdout?: string}).stdout ?? '';
		return expectErrorEnvelope(JSON.parse(stdout));
	}

	throw new Error(`webview ${args.join(' ')} unexpectedly succeeded`);
}

function firstWebView(simulatorId: string): WebViewInfo {
	const webViews = mobilecli(['webview', 'list', '--device', simulatorId]).data as unknown[];
	expect(webViews.length, 'no webview reported by the playground app').toBeGreaterThan(0);
	expectWebViewShape(webViews[0]);
	return webViews[0];
}

function webViewUrl(simulatorId: string, webViewId: string): string {
	return mobilecli(['webview', 'url', webViewId, '--device', simulatorId]).data as string;
}

function webViewTitle(simulatorId: string, webViewId: string): string {
	return mobilecli(['webview', 'title', webViewId, '--device', simulatorId]).data as string;
}

function webViewContent(simulatorId: string, webViewId: string): string {
	return mobilecli(['webview', 'content', webViewId, '--device', simulatorId]).data as string;
}

function webViewEval(simulatorId: string, webViewId: string, expression: string): unknown {
	return mobilecli(['webview', 'eval', webViewId, expression, '--device', simulatorId]).data;
}

function webViewQuery(simulatorId: string, webViewId: string, selector: string): WebViewQueryResult[] {
	return mobilecli(['webview', 'query', webViewId, selector, '--device', simulatorId]).data as WebViewQueryResult[];
}

function webViewGoto(simulatorId: string, webViewId: string, url: string): void {
	mobilecli(['webview', 'goto', webViewId, url, '--device', simulatorId]);
}

function webViewReload(simulatorId: string, webViewId: string): void {
	mobilecli(['webview', 'reload', webViewId, '--device', simulatorId]);
}

function webViewGoBack(simulatorId: string, webViewId: string): void {
	mobilecli(['webview', 'back', webViewId, '--device', simulatorId]);
}

function webViewGoForward(simulatorId: string, webViewId: string): void {
	mobilecli(['webview', 'forward', webViewId, '--device', simulatorId]);
}

function webViewWait(simulatorId: string, webViewId: string, state: string): void {
	mobilecli(['webview', 'wait', webViewId, '--state', state, '--timeout', '15000', '--device', simulatorId]);
}

// playwright has no sleep of its own and these waits are for device settling
function sleep(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

function getAppContainerPath(simulatorId: string, packageName: string): string {
	return mobilecli(['apps', 'path', packageName, '--device', simulatorId]).data.path;
}

function fsList(simulatorId: string, remotePath: string): any[] {
	const response = mobilecli(['fs', 'ls', '--device', simulatorId, remotePath]);
	expectFsListingShape(response.data);
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

// screenrecord emits progress on stderr (not JSON on stdout), so it can't use
// the JSON-parsing mobilecli() helper. these runners drive the binary directly
// while keeping the GOCOVERDIR env so coverage data is still collected.
function recordScreenWithTimeLimit(simulatorId: string, videoPath: string, timeLimitSeconds: number): void {
	const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
	execFileSync(mobilecliBinary, ['screenrecord', '--device', simulatorId, '--time-limit', String(timeLimitSeconds), '--output', videoPath], {
		encoding: 'utf8',
		timeout: 180000,
		stdio: ['pipe', 'pipe', 'pipe'],
		env: coverageEnv(),
	});
}

// records with no time limit, lets it run for recordSeconds, then sends SIGINT
// (Ctrl-C). mobilecli is expected to catch the signal, finalize the mp4, and
// exit cleanly. resolves once the process has fully exited.
function recordScreenThenInterruptWithCtrlC(simulatorId: string, videoPath: string, recordSeconds: number): Promise<void> {
	const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
	return new Promise((resolve, reject) => {
		const child = spawn(mobilecliBinary, ['screenrecord', '--device', simulatorId, '--output', videoPath], {
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
function verifyVideoIsPlayable(videoPath: string): void {
	expect(fs.existsSync(videoPath)).toBe(true);
	expect(fs.statSync(videoPath).size).toBeGreaterThan(0);

	const {width, height} = probeVideoDimensions(videoPath);
	expect(width).toBeGreaterThan(0);
	expect(height).toBeGreaterThan(0);
}

// the simulator records at native resolution, so the recording dimensions
// should match what a screenshot reports.
function verifyVideoMatchesScreenshotDimensions(videoPath: string, simulatorId: string): void {
	const video = probeVideoDimensions(videoPath);
	const screenshot = getScreenshotDimensions(simulatorId);
	expect(video).toEqual(screenshot);
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

function getScreenshotDimensions(simulatorId: string): Dimensions {
	const screenshotPath = path.join(os.tmpdir(), `mobilecli-screenshot-${Date.now()}.png`);
	takeScreenshot(simulatorId, screenshotPath);
	const dimensions = readPngDimensions(screenshotPath);
	fs.unlinkSync(screenshotPath);
	return dimensions;
}

// reads width/height straight from the PNG IHDR chunk (big-endian uint32 at
// byte offsets 16 and 20) to avoid pulling in an image-decoding dependency.
function readPngDimensions(pngPath: string): Dimensions {
	const buffer = fs.readFileSync(pngPath);
	return {
		width: buffer.readUInt32BE(16),
		height: buffer.readUInt32BE(20),
	};
}
