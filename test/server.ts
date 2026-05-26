import {expect} from 'chai';
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

describe('server jsonrpc', () => {
	// Start server before all tests
	before(async function () {
		this.timeout(SERVER_TIMEOUT + 2000);

		await startTestServer();
		await waitForServer(TEST_SERVER_URL, SERVER_TIMEOUT);
	});

	// Stop server after all tests
	after(() => {
		stopTestServer();
	});

	it('should return status "ok" for root endpoint', async () => {
		const response = await fetch(TEST_SERVER_URL);

		expect(response.status).to.equal(200);
		expect(await response.json()).to.have.property('status', 'ok');
	});

	it('GET should return 405 Method Not Allowed for /rpc endpoint', async () => {
		const response = await fetch(`${TEST_SERVER_URL}/rpc`);
		expect(response.status).to.equal(405);
	});

	it('Empty POST body should return parse error', async () => {
		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: ''
		});

		expect(response.status).to.equal(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.jsonrpc).to.equal('2.0');
		expect(jsonResp.error).to.not.be.null;
		expect(jsonResp.error).to.not.be.undefined;

		if (jsonResp.error) {
			expect(jsonResp.error.code).to.equal(ErrCodeParseError);
			expect(jsonResp.error.data).to.equal('expecting jsonrpc payload');
		}
	});

	it('Invalid jsonrpc version should return error', async () => {
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

		expect(response.status).to.equal(200);
		expect(data.jsonrpc).to.equal('2.0');
		expect(data.error).to.not.be.null;
		expect(data.error).to.not.be.undefined;
		expect(data.error!.code).to.equal(ErrCodeInvalidRequest);
		expect(data.error!.data).to.equal("'jsonrpc' must be '2.0'");
	});

	it('Missing id field should return error', async () => {
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

		expect(response.status).to.equal(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.jsonrpc).to.equal('2.0');
		expect(jsonResp.error).to.not.be.null;
		expect(jsonResp.error).to.not.be.undefined;

		if (jsonResp.error) {
			expect(jsonResp.error.code).to.equal(ErrCodeInvalidRequest);
			expect(jsonResp.error.data).to.equal("'id' field is required");
		}
	});

	it('should require params for device_info method', async () => {
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

		expect(response.status).to.equal(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.jsonrpc).to.equal('2.0');
		expect(jsonResp.id).to.equal(1);
		expect(jsonResp.error).to.not.be.null;

		if (jsonResp.error) {
			expect(jsonResp.error.code).to.equal(ErrCodeServerError);
			expect(jsonResp.error.data).to.equal("'params' is required with fields: deviceId");
		}
	});

	it('should return method not found error for unknown methods', async () => {
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

		expect(response.status).to.equal(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).to.not.be.null;

		if (jsonResp.error) {
			expect(jsonResp.error.code).to.equal(ErrCodeMethodNotFound);
		}
	});

	it('should return error when method field is missing', async () => {
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
		expect(jsonResp.error).to.not.be.null;

		if (jsonResp.error) {
			expect(jsonResp.error.code).to.equal(ErrCodeServerError);
			expect(jsonResp.error.data).to.equal("'method' is required");
		}
	});

	it('should handle invalid JSON gracefully', async () => {
		const invalidJson = '{invalid json}';

		const response = await fetch(`${TEST_SERVER_URL}/rpc`, {
			method: 'POST',
			headers: {'Content-Type': 'application/json'},
			body: invalidJson
		});

		expect(response.status).to.equal(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).to.not.be.null;

		if (jsonResp.error) {
			expect(jsonResp.error.code).to.equal(ErrCodeParseError);
		}
	});

	it('should return error for empty method string', async () => {
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

		expect(response.status).to.equal(200);

		const jsonResp: JSONRPCResponse = await response.json();
		expect(jsonResp.error).to.not.be.null;
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
