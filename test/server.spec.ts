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
import {randomUUID} from "node:crypto";
import {mkdirSync} from "fs";

const TEST_SERVER_URL = 'http://localhost:12001';
const TEST_SERVER_PORT = '12001';
const SERVER_TIMEOUT = 8000; // 8 seconds

let serverProcess: ChildProcess | null = null;

const createCoverageDirectory = (): string => {
	const dir = path.join(__dirname, "cover-" + randomUUID());
	mkdirSync(dir);
	return dir;
}

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

// Helper functions
async function startTestServer(): Promise<void> {
	return new Promise((resolve, reject) => {
		const binaryPath = path.join(__dirname, '..', 'mobilecli');
		const coverdata = createCoverageDirectory();

		serverProcess = spawn(binaryPath, ['server', 'start', '--listen', `localhost:${TEST_SERVER_PORT}`], {
			stdio: 'pipe', // Capture stdout/stderr but don't display
			env: {
				...process.env,
				"GOCOVERDIR": coverdata,
			},
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
