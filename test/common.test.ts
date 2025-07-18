import { spawn, ChildProcess } from 'child_process';
import { describe, it, before, after } from 'mocha';
import { strict as assert } from 'assert';
import path from 'path';

describe('common', () => {
	let serverProcess: ChildProcess;
	const serverUrl = 'http://localhost:12000';
	const timeout = 10000;

	before(async function () {
		this.timeout(timeout);

		// Start the mobilecli server
		serverProcess = spawn('../mobilecli', ['server', 'start'], {
			cwd: path.dirname(__filename),
			stdio: ['ignore', 'pipe', 'pipe']
		});

		// Wait for server to start by polling the endpoint
		await waitForServer(serverUrl, 8000); // Wait up to 8 seconds for server to start
	});

	after(() => {
		// Clean up the server process
		if (serverProcess) {
			serverProcess.kill('SIGTERM');
		}
	});

	it('should return status "ok" from root endpoint', async () => {
		const response = await fetch(serverUrl);
		assert.equal(response.status, 200);

		const data = await response.json();
		assert.deepEqual(data, { status: 'ok' });
	});

	it('should return method_not_allowed for GET /rpc endpoint', async () => {
		const response = await fetch(`${serverUrl}/rpc`);
		assert.equal(response.status, 405);
	});

	it('should return method_not_allowed for POST /rpc endpoint', async () => {
		const response = await fetch(`${serverUrl}/rpc`, { method: 'POST' });
		assert.equal(response.status, 200);
		const json = await response.json();
		assert.equal(json.jsonrpc, '2.0');
		assert.equal(json.error.code, -32700);
		assert.equal(json.error.data, 'expecting jsonrpc payload');
	});

	it('should return error if jsonrpc is not 2.0', async () => {
		const response = await fetch(`${serverUrl}/rpc`, { method: 'POST', body: JSON.stringify({ jsonrpc: '1.0' }) });
		assert.equal(response.status, 200);
		const json = await response.json();
		assert.equal(json.jsonrpc, '2.0');
		assert.equal(json.error.code, -32600);
		assert.equal(json.error.data, "'jsonrpc' must be '2.0'");
	});

	it('should return error if id is missing', async () => {
		const response = await fetch(`${serverUrl}/rpc`, { method: 'POST', body: JSON.stringify({ jsonrpc: '2.0', method: 'devices', params: {} }) });
		assert.equal(response.status, 200);
		const json = await response.json();
		assert.equal(json.jsonrpc, '2.0');
		assert.equal(json.error.code, -32600);
		assert.equal(json.error.data, "'id' field is required");
	});

	it('should accept "devices" call without params', async () => {
		const response = await fetch(`${serverUrl}/rpc`, {
			method: 'POST',
			body: JSON.stringify({
				jsonrpc: '2.0',
				method: 'devices',
				id: 1
			})
		});
		assert.equal(response.status, 200);
		const json = await response.json();
		assert.equal(json.jsonrpc, '2.0');
		assert.equal(json.id, 1);
		assert.ok(Array.isArray(json.result.devices));
	});

	it('should return error for "device_info" call without params', async () => {
		const response = await fetch(`${serverUrl}/rpc`, {
			method: 'POST', 
			body: JSON.stringify({
				jsonrpc: '2.0',
				method: 'device_info',
				id: 1
			})
		});
		assert.equal(response.status, 200);
		const json = await response.json();
		assert.equal(json.jsonrpc, '2.0');
		assert.equal(json.id, 1);
		assert.equal(json.error.code, -32000);
		assert.equal(json.error.data, "'params' is required with fields: deviceId");
	});
});

async function waitForServer(url: string, timeoutMs: number): Promise<void> {
	const startTime = Date.now();

	while (Date.now() - startTime < timeoutMs) {
		try {
			const response = await fetch(url);
			if (response.status === 200) {
				return; // Server is ready
			}
		} catch (error) {
			// Server not ready yet, continue waiting
		}

		// Wait 100ms before next attempt
		await new Promise(resolve => setTimeout(resolve, 100));
	}

	throw new Error(`Server did not start within ${timeoutMs}ms`);
}
