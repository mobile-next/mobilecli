import {test, expect} from '@playwright/test';
import {spawn} from 'child_process';
import type {ChildProcess} from 'child_process';
import * as path from 'path';
import type {JSONRPCRequest, JSONRPCResponse} from './jsonrpc';
import {
	ErrCodeParseError,
	ErrCodeInvalidRequest,
	ErrCodeMethodNotFound,
	ErrCodeServerError
} from './jsonrpc';
import {coverageEnv} from './coverage';
import {
	expectAppShape,
	expectDeviceShape,
	expectForegroundAppShape,
	expectUIDumpShape,
} from './shapes';

const TEST_SERVER_URL = 'http://localhost:12001';
const TEST_SERVER_PORT = '12001';
const SERVER_TIMEOUT = 8000; // 8 seconds

// every device-scoped handler resolves the device before doing any work, so an id
// that cannot exist makes them fail identically whether or not hardware is attached.
// this keeps the validation suite deterministic on a bare CI runner.
const UNKNOWN_DEVICE_ID = '__no_such_device__';

type RpcDevice = {
	id: string;
	platform: string;
	type: string;
	state: string;
};

let serverProcess: ChildProcess | null = null;

test.describe('server jsonrpc', () => {
	// Start server before all tests
	test.beforeAll(async () => {
		await startTestServer();
		await waitForServer(TEST_SERVER_URL, SERVER_TIMEOUT);
	});

	// Stop server after all tests
	test.afterAll(() => {
		stopTestServer();
	});

	test('should return status "ok" for root endpoint', async () => {
		const response = await fetch(TEST_SERVER_URL);

		expect(response.status).toBe(200);
		expect(await response.json()).toHaveProperty('status', 'ok');
	});

	test('GET should return 405 Method Not Allowed for /rpc endpoint', async () => {
		const response = await fetch(`${TEST_SERVER_URL}/rpc`);
		expect(response.status).toBe(405);
	});

	test('Empty POST body should return parse error', async () => {
		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: ''
		});

		expect(response.status).toBe(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.jsonrpc).toBe('2.0');
		expect(jsonResp.error).toBeDefined();
		expect(jsonResp.error).not.toBeNull();

		if (jsonResp.error) {
			expect(jsonResp.error.code).toBe(ErrCodeParseError);
			expect(jsonResp.error.data).toBe('expecting jsonrpc payload');
		}
	});

	test('Invalid jsonrpc version should return error', async () => {
		const payload = {
			jsonrpc: '0.1',
			method: 'devices',
			id: 1,
		};

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(payload)
		});
		const data: JSONRPCResponse = await response.json();

		expect(response.status).toBe(200);
		expect(data.jsonrpc).toBe('2.0');
		expect(data.error).toBeDefined();
		expect(data.error).not.toBeNull();
		expect(data.error!.code).toBe(ErrCodeInvalidRequest);
		expect(data.error!.data).toBe("'jsonrpc' must be '2.0'");
	});

	test('Missing id field should return error', async () => {
		const payload = {
			jsonrpc: '2.0',
			method: 'devices',
			params: {}
		};

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(payload)
		});

		expect(response.status).toBe(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.jsonrpc).toBe('2.0');
		expect(jsonResp.error).toBeDefined();
		expect(jsonResp.error).not.toBeNull();

		if (jsonResp.error) {
			expect(jsonResp.error.code).toBe(ErrCodeInvalidRequest);
			expect(jsonResp.error.data).toBe("'id' field is required");
		}
	});

	test('should require params for device_info method', async () => {
		const payload: JSONRPCRequest = {
			jsonrpc: '2.0',
			method: 'device.info',
			id: 1
		};

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(payload)
		});

		expect(response.status).toBe(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.jsonrpc).toBe('2.0');
		expect(jsonResp.id).toBe(1);
		expect(jsonResp.error).not.toBeNull();

		if (jsonResp.error) {
			expect(jsonResp.error.code).toBe(ErrCodeServerError);
			expect(jsonResp.error.data).toBe("'params' is required with fields: deviceId");
		}
	});

	test('should return method not found error for unknown methods', async () => {
		const payload: JSONRPCRequest = {
			jsonrpc: '2.0',
			method: 'unknown_method',
			id: 1
		};

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(payload)
		});

		expect(response.status).toBe(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).not.toBeNull();

		if (jsonResp.error) {
			expect(jsonResp.error.code).toBe(ErrCodeMethodNotFound);
		}
	});

	test('should return error when method field is missing', async () => {
		const payload = {
			jsonrpc: '2.0',
			id: 1
		};

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(payload)
		});

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).not.toBeNull();

		if (jsonResp.error) {
			expect(jsonResp.error.code).toBe(ErrCodeServerError);
			expect(jsonResp.error.data).toBe("'method' is required");
		}
	});

	test('should handle invalid JSON gracefully', async () => {
		const invalidJson = '{invalid json}';

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: invalidJson
		});

		expect(response.status).toBe(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).not.toBeNull();

		if (jsonResp.error) {
			expect(jsonResp.error.code).toBe(ErrCodeParseError);
		}
	});

	test('should return error for empty method string', async () => {
		const payload: JSONRPCRequest = {
			jsonrpc: '2.0',
			method: '',
			id: 1
		};

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: JSON.stringify(payload)
		});

		expect(response.status).toBe(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).not.toBeNull();
	});
});

// these run anywhere, including a CI runner with no devices attached. they reach
// each handler's unmarshal + validation path, which is the half of the dispatch
// table the CLI e2e specs can never cover.
test.describe('rpc method validation', () => {
	test.beforeAll(async () => {
		await startTestServer();
		await waitForServer(TEST_SERVER_URL, SERVER_TIMEOUT);
	});

	test.afterAll(() => {
		stopTestServer();
	});

	test('server.info should report name and version without params', async () => {
		const result = await rpcExpectResult('server.info');
		expect(result.name).toBe('mobilecli');
		expect(typeof result.version).toBe('string');
	});

	test('devices.list should succeed with includeOffline', async () => {
		const result = await rpcExpectResult('devices.list', {includeOffline: true});
		expect(Array.isArray(result.devices)).toBe(true);
		result.devices.forEach(expectDeviceShape);
	});

	const methodsRequiringAKnownDevice: {method: string; params: any}[] = [
		{method: 'device.info', params: {}},
		{method: 'device.screenshot', params: {format: 'png'}},
		{method: 'device.dump.ui', params: {}},
		{method: 'device.url', params: {url: 'https://example.com'}},
		{method: 'device.apps.list', params: {}},
		{method: 'device.apps.foreground', params: {}},
		{method: 'device.apps.launch', params: {bundleId: 'com.example.app'}},
		{method: 'device.apps.terminate', params: {bundleId: 'com.example.app'}},
		{method: 'device.apps.path', params: {bundleId: 'com.example.app'}},
		{method: 'device.apps.uninstall', params: {packageName: 'com.example.app'}},
		{method: 'device.io.tap', params: {x: 10, y: 10}},
		{method: 'device.io.longpress', params: {x: 10, y: 10}},
		{method: 'device.io.swipe', params: {x1: 1, y1: 2, x2: 3, y2: 4}},
		{method: 'device.io.button', params: {button: 'HOME'}},
		{method: 'device.io.text', params: {text: 'hello'}},
		{method: 'device.io.keys', params: {keys: ['a']}},
		{method: 'device.io.orientation.get', params: {}},
		{method: 'device.io.orientation.set', params: {orientation: 'portrait'}},
		{method: 'device.boot', params: {}},
		{method: 'device.shutdown', params: {}},
		{method: 'device.reboot', params: {}},
		{method: 'device.settings.apply', params: {animations: false}},
		{method: 'device.fs.ls', params: {remotePath: '/tmp'}},
		{method: 'device.fs.mkdir', params: {remotePath: '/tmp/x'}},
		{method: 'device.fs.rm', params: {remotePath: '/tmp/x'}},
		{method: 'device.webview.list', params: {}},
	];

	for (const {method, params} of methodsRequiringAKnownDevice) {
		test(`${method} should return an error for an unknown device`, async () => {
			await rpcExpectError(method, {...params, deviceId: UNKNOWN_DEVICE_ID});
		});
	}

	const methodsMissingARequiredField: {name: string; method: string; params: any}[] = [
		{name: 'device.apps.path without bundleId', method: 'device.apps.path', params: {}},
		{name: 'device.crashes.get without id', method: 'device.crashes.get', params: {}},
		{name: 'device.fs.pull without remotePath', method: 'device.fs.pull', params: {}},
		{name: 'device.fs.push without remotePath', method: 'device.fs.push', params: {content: 'aGk='}},
		{name: 'device.fs.push without content', method: 'device.fs.push', params: {remotePath: '/tmp/x'}},
		{name: 'device.fs.mkdir without remotePath', method: 'device.fs.mkdir', params: {}},
		{name: 'device.fs.rm without remotePath', method: 'device.fs.rm', params: {}},
		{name: 'device.io.swipe without x2', method: 'device.io.swipe', params: {deviceId: UNKNOWN_DEVICE_ID, x1: 1, y1: 2, y2: 4}},
		{name: 'device.webview.goto without url', method: 'device.webview.goto', params: {deviceId: UNKNOWN_DEVICE_ID, id: 'x'}},
	];

	for (const {name, method, params} of methodsMissingARequiredField) {
		test(`${name} should return an error`, async () => {
			await rpcExpectError(method, params);
		});
	}

	// handleFsPush decodes and size-checks the payload before it ever looks for a
	// device, so these two branches are reachable without hardware
	test('device.fs.push should reject content that is not valid base64', async () => {
		const error = await rpcExpectError('device.fs.push', {
			deviceId: UNKNOWN_DEVICE_ID,
			remotePath: '/tmp/x',
			content: 'not!valid!base64!',
		});
		expect(error.data).toContain('base64');
	});

	test('device.fs.push should reject a payload above the 1MB transfer limit', async () => {
		const oversized = Buffer.alloc(1024 * 1024 + 1, 0x41).toString('base64');
		const error = await rpcExpectError('device.fs.push', {
			deviceId: UNKNOWN_DEVICE_ID,
			remotePath: '/tmp/x',
			content: oversized,
		});
		expect(error.data).toContain('too large');
	});

});

test.describe('rpc methods against a live device', () => {
	let device: RpcDevice | null = null;

	test.beforeAll(async () => {
		await startTestServer();
		await waitForServer(TEST_SERVER_URL, SERVER_TIMEOUT);
		device = await findFirstVirtualDevice();
		if (!device) {
			console.log('No emulator or simulator found. See test/README.md for setup instructions.');
		}
	});

	test.afterAll(() => {
		stopTestServer();
	});

	test('device.info should describe the selected device', async () => {
		test.skip(!device, 'no device found');

		const result = await rpcExpectResult('device.info', {deviceId: device!.id});
		expectDeviceShape(result.device);
		expect(result.device.id).toBe(device!.id);
		expect(result.device.platform).toBe(device!.platform);
	});

	test('device.screenshot should return image bytes', async () => {
		test.skip(!device, 'no device found');

		const result = await rpcExpectResult('device.screenshot', {deviceId: device!.id, format: 'png'});
		expect(Buffer.from(result.data, 'base64').length).toBeGreaterThan(64 * 1024);
	});

	test('device.apps.list should return installed packages', async () => {
		test.skip(!device, 'no device found');

		const result = await rpcExpectResult('device.apps.list', {deviceId: device!.id});
		expect(result.length).toBeGreaterThan(0);
		result.forEach(expectAppShape);
	});

	test('device.apps.launch and device.apps.foreground should agree', async () => {
		test.skip(!device, 'no device found');

		const bundleId = settingsPackageFor(device!.platform);
		await rpcExpectResult('device.apps.launch', {deviceId: device!.id, bundleId});
		await sleep(5000);

		const foreground = await rpcExpectResult('device.apps.foreground', {deviceId: device!.id});
		expectForegroundAppShape(foreground);
		expect(foreground.packageName).toBe(bundleId);
	});

	test('device.apps.terminate should stop the running app', async () => {
		test.skip(!device, 'no device found');

		const bundleId = settingsPackageFor(device!.platform);
		await rpcExpectResult('device.apps.launch', {deviceId: device!.id, bundleId});
		await sleep(5000);
		await rpcExpectResult('device.apps.terminate', {deviceId: device!.id, bundleId});
		await sleep(3000);

		const foreground = await rpcExpectResult('device.apps.foreground', {deviceId: device!.id});
		expect(foreground.packageName).not.toBe(bundleId);
	});

	test('device.dump.ui should return a non-empty element tree', async () => {
		test.skip(!device, 'no device found');

		const result = await rpcExpectResult('device.dump.ui', {deviceId: device!.id});
		expectUIDumpShape(result.elements);
	});

	test('device.io input methods should all be accepted', async () => {
		test.skip(!device, 'no device found');

		// coordinates near the top-left are inert on both home screens, so these
		// exercise the transport without navigating anywhere
		await rpcExpectResult('device.io.tap', {deviceId: device!.id, x: 5, y: 5});
		await rpcExpectResult('device.io.longpress', {deviceId: device!.id, x: 5, y: 5});
		await rpcExpectResult('device.io.swipe', {deviceId: device!.id, x1: 5, y1: 5, x2: 5, y2: 5});
		await rpcExpectResult('device.io.button', {deviceId: device!.id, button: 'HOME'});
		await rpcExpectResult('device.io.text', {deviceId: device!.id, text: 'mobilecli'});
	});

	test('device.io.orientation.get should report the current orientation', async () => {
		test.skip(!device, 'no device found');

		const result = await rpcExpectResult('device.io.orientation.get', {deviceId: device!.id});
		expect(typeof result.orientation).toBe('string');
	});

	test('device.url should open a url without error', async () => {
		test.skip(!device, 'no device found');

		await rpcExpectResult('device.url', {deviceId: device!.id, url: 'https://example.com'});
	});

	test('device.crashes.list should return a list', async () => {
		test.skip(!device, 'no device found');

		const result = await rpcExpectResult('device.crashes.list', {deviceId: device!.id});
		expect(Array.isArray(result.crashes ?? result)).toBe(true);
	});
});

// Helper functions
function sleep(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

function settingsPackageFor(platform: string): string {
	return platform === 'android' ? 'com.android.settings' : 'com.apple.Preferences';
}

async function rpc(method: string, params?: any): Promise<JSONRPCResponse> {
	const payload: JSONRPCRequest = {jsonrpc: '2.0', method, id: 1};
	if (params !== undefined) {
		payload.params = params;
	}

	const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
		method: 'POST',
		headers: {'Content-Type': 'application/json'},
		body: JSON.stringify(payload),
	});

	expect(response.status).toBe(200);
	const body = await response.json() as JSONRPCResponse;

	// enforced on every call so a protocol regression fails the whole suite rather
	// than slipping past tests that only look at their own payload
	expect(body.jsonrpc, `${method} response is not jsonrpc 2.0`).toBe('2.0');
	expect(body.id, `${method} did not echo the request id`).toBe(payload.id);
	const hasResult = body.result !== undefined;
	const hasError = body.error !== undefined;
	expect(hasResult !== hasError, `${method} must carry exactly one of result/error: ${JSON.stringify(body)}`).toBe(true);

	return body;
}

async function rpcExpectResult(method: string, params?: any): Promise<any> {
	const response = await rpc(method, params);
	expect(response.error, `${method} unexpectedly failed: ${JSON.stringify(response.error)}`).toBeUndefined();
	return response.result;
}

async function rpcExpectError(method: string, params?: any): Promise<NonNullable<JSONRPCResponse['error']>> {
	const response = await rpc(method, params);
	expect(response.error, `${method} unexpectedly succeeded`).toBeDefined();
	// without this, a method dropped from the dispatch table (or a typo in this
	// file) would still "return an error" and the test would pass for the wrong reason
	expect(response.error!.code, `${method} is not registered in the dispatch table`).not.toBe(ErrCodeMethodNotFound);
	// a registered method that fails reports a server error; anything else means
	// the request was rejected before dispatch, which these tests never intend
	expect([ErrCodeServerError, ErrCodeInvalidRequest], `${method} returned an unexpected error code`).toContain(response.error!.code);
	expect(typeof response.error!.message).toBe('string');
	expect(response.error!.message.length, `${method} returned an empty error message`).toBeGreaterThan(0);
	return response.error!;
}

// these tests launch apps, tap, type and press HOME, so they must never drive a
// physical handset that happens to be plugged in — only an emulator or simulator
async function findFirstVirtualDevice(): Promise<RpcDevice | null> {
	const result = await rpcExpectResult('devices.list');
	return (result.devices as RpcDevice[]).find(d => d.state === 'online' && d.type !== 'real') ?? null;
}


async function startTestServer(): Promise<void> {
	return new Promise((resolve, reject) => {
		const binaryPath = path.join(__dirname, '..', 'mobilecli');
		serverProcess = spawn(binaryPath, ['server', 'start', '--listen', `localhost:${TEST_SERVER_PORT}`], {
			stdio: 'pipe', // Capture stdout/stderr but don't display
			env: coverageEnv(),
		});

		if (!serverProcess) {
			return reject(new Error('Failed to start server process'));
		}

		// Handle process events
		serverProcess.on('error', (error) => {
			reject(new Error(`Failed to start server: ${error.message}`));
		});

		serverProcess.on('spawn', () => {
			resolve();
		});

		// Suppress output by consuming the streams
		if (serverProcess.stdout) {
			serverProcess.stdout.on('data', () => {
			});
		}

		if (serverProcess.stderr) {
			serverProcess.stderr.on('data', () => {
			});
		}
	});
}

function stopTestServer(): void {
	if (serverProcess && !serverProcess.killed) {
		serverProcess.kill();
		serverProcess = null;
	}
}

async function waitForServer(url: string, timeout: number): Promise<void> {
	const startTime = Date.now();

	while (Date.now() - startTime < timeout) {
		try {
			const controller = new AbortController();
			const timer = setTimeout(() => controller.abort(), 1000);
			const response = await fetch(url, {signal: controller.signal});
			clearTimeout(timer);
			if (response.status === 200) {
				return;
			}
		} catch (error) {
			// Server not ready yet, continue waiting
		}

		// Wait 100ms before next attempt
		await new Promise(resolve => setTimeout(resolve, 100));
	}

	throw new Error(`Server did not start within ${timeout}ms`);
}
