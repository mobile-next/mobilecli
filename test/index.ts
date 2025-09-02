import { expect } from 'chai';
import { spawn, ChildProcess, execSync } from 'child_process';
import axios, { AxiosResponse } from 'axios';
import * as path from 'path';
import * as fs from 'fs';
import { 
  createAndLaunchSimulator, 
  shutdownSimulator, 
  deleteSimulator, 
  cleanupSimulators,
  removeFromTracking 
} from './simctl';

const TEST_SERVER_URL = 'http://localhost:12001';
const TEST_SERVER_PORT = '12001';
const SERVER_TIMEOUT = 8000; // 8 seconds

let serverProcess: ChildProcess | null = null;

// JSON-RPC types
interface JSONRPCRequest {
  jsonrpc: string;
  method: string;
  params?: any;
  id: number | string;
}

interface JSONRPCResponse {
  jsonrpc: string;
  result?: any;
  error?: {
    code: number;
    message: string;
    data?: any;
  };
  id: number | string | null;
}

// Error codes from the Go server
const ErrCodeParseError = -32700;
const ErrCodeInvalidRequest = -32600;
const ErrCodeMethodNotFound = -32601;
const ErrCodeServerError = -32000;

describe('server jsonrpc', () => {
  // Start server before all tests
  before(async function() {
    this.timeout(SERVER_TIMEOUT + 2000);
    
    await startTestServer();
    await waitForServer(TEST_SERVER_URL, SERVER_TIMEOUT);
  });

  // Stop server after all tests
  after(() => {
    stopTestServer();
    cleanupSimulators();
  });

  it('should return status "ok" for root endpoint', async () => {
    const response = await axios.get(TEST_SERVER_URL);
    
    expect(response.status).to.equal(200);
    expect(response.data).to.have.property('status', 'ok');
  });

  it('GET should return 405 Method Not Allowed for /rpc endpoint', async () => {
    try {
      await axios.get(`${TEST_SERVER_URL}/rpc`);
      throw new Error('Expected request to fail');
    } catch (error: any) {
      expect(error.response.status).to.equal(405);
    }
  });

  it('Empty POST body should return parse error', async () => {
    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      '',
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
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
      jsonrpc: '1.0',
      method: 'devices',
      id: 1
    };

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      JSON.stringify(payload),
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
    expect(jsonResp.jsonrpc).to.equal('2.0');
    expect(jsonResp.error).to.not.be.null;
    expect(jsonResp.error).to.not.be.undefined;

    if (jsonResp.error) {
      expect(jsonResp.error.code).to.equal(ErrCodeInvalidRequest);
      expect(jsonResp.error.data).to.equal("'jsonrpc' must be '2.0'");
    }
  });

  it('Missing id field should return error', async () => {
    const payload = {
      jsonrpc: '2.0',
      method: 'devices',
      params: {}
    };

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      JSON.stringify(payload),
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
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
      method: 'device_info',
      id: 1
    };

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      JSON.stringify(payload),
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
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

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      JSON.stringify(payload),
      {
        headers: { 'Content-Type': 'application/json' }
        }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
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

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      JSON.stringify(payload),
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    const jsonResp: JSONRPCResponse = response.data;
    expect(jsonResp.error).to.not.be.null;

    if (jsonResp.error) {
      expect(jsonResp.error.code).to.equal(ErrCodeServerError);
      expect(jsonResp.error.data).to.equal("'method' is required");
    }
  });

  it('should handle invalid JSON gracefully', async () => {
    const invalidJson = '{invalid json}';

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      invalidJson,
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
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

    const response = await axios.post(
      `${TEST_SERVER_URL}/rpc`,
      JSON.stringify(payload),
      {
        headers: { 'Content-Type': 'application/json' }
      }
    );

    expect(response.status).to.equal(200);

    const jsonResp: JSONRPCResponse = response.data;
    expect(jsonResp.error).to.not.be.null;
  });

  it('should detect iOS simulator in device list', async function() {
    this.timeout(60000); // Simulator creation can take a while
    
    // Create and launch iOS 18 simulator
    const simulatorId = createAndLaunchSimulator('18');
    
    try {
      // Call mobilecli devices via JSON-RPC
      const payload: JSONRPCRequest = {
        jsonrpc: '2.0',
        method: 'devices',
        id: 1
      };

      const response = await axios.post(
        `${TEST_SERVER_URL}/rpc`,
        JSON.stringify(payload),
        {
          headers: { 'Content-Type': 'application/json' }
        }
      );

      expect(response.status).to.equal(200);

      const jsonResp: JSONRPCResponse = response.data;
      expect(jsonResp.jsonrpc).to.equal('2.0');
      expect(jsonResp.id).to.equal(1);
      expect(jsonResp.error).to.be.undefined;
      expect(jsonResp.result).to.not.be.null;

      // Check that the result contains devices
      const result = jsonResp.result;
      expect(result).to.have.property('devices');
      expect(Array.isArray(result.devices)).to.be.true;

      // Look for our simulator in the device list
      const devices = result.devices;
      const simulator = devices.find((device: any) => 
        device.id === simulatorId || 
        device.name?.includes('Test-iOS-18') ||
        device.platform === 'ios' && device.type === 'simulator'
      );

      expect(simulator).to.not.be.undefined;
      if (simulator) {
        expect(simulator).to.have.property('platform', 'ios');
        expect(simulator).to.have.property('type', 'simulator');
      }
    } finally {
      // Clean up this specific simulator
      shutdownSimulator(simulatorId);
      deleteSimulator(simulatorId);
      // Remove from tracking array
      removeFromTracking(simulatorId);
    }
  });

  it('should take screenshot of iOS simulator', async function() {
    this.timeout(60000); // Simulator creation can take a while
    
    // Create and launch iOS 18 simulator
    const simulatorId = createAndLaunchSimulator('18');
    const screenshotPath = '/tmp/screenshot.png';
    
    cleanupExistingScreenshotFile(screenshotPath);
    
    try {
      takeScreenshotUsingMobileCLI(simulatorId, screenshotPath);
      verifyScreenshotFileWasCreated(screenshotPath);
      verifyScreenshotFileHasValidContent(screenshotPath);
    } finally {
      cleanupTestArtifacts(simulatorId, screenshotPath);
    }
  });
});

// Helper functions
async function startTestServer(): Promise<void> {
  return new Promise((resolve, reject) => {
    // Look for mobilecli binary in parent directory
    const binaryPath = path.join(__dirname, '..', 'mobilecli');
    
    serverProcess = spawn(binaryPath, ['server', 'start', '--listen', `localhost:${TEST_SERVER_PORT}`], {
      stdio: 'pipe' // Capture stdout/stderr but don't display
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
        // Consume stdout data silently
      });
    }
    
    if (serverProcess.stderr) {
      serverProcess.stderr.on('data', () => {
        // Consume stderr data silently
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
      const response = await axios.get(url, { timeout: 1000 });
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

// Screenshot test helper functions with descriptive English names
function cleanupExistingScreenshotFile(screenshotPath: string): void {
  if (fs.existsSync(screenshotPath)) {
    fs.unlinkSync(screenshotPath);
    console.log(`Cleaned up existing screenshot file: ${screenshotPath}`);
  }
}

function takeScreenshotUsingMobileCLI(simulatorId: string, screenshotPath: string): void {
  const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
  const command = `${mobilecliBinary} screenshot --device ${simulatorId} --format png --output ${screenshotPath}`;
  
  console.log(`Taking screenshot with command: ${command}`);
  
  // Use 60 second timeout to match Mocha timeout
  const stdout = execSync(command, { 
    encoding: 'utf8', 
    timeout: 60000 // 60 seconds - matches Mocha timeout
  });
  
  console.log(`Screenshot command completed successfully`);
  if (stdout.trim()) {
    console.log(`Command output: ${stdout}`);
  }
}

function verifyScreenshotFileWasCreated(screenshotPath: string): void {
  const fileExists = fs.existsSync(screenshotPath);
  expect(fileExists).to.be.true;
  console.log(`✓ Screenshot file was created: ${screenshotPath}`);
}

function verifyScreenshotFileHasValidContent(screenshotPath: string): void {
  const stats = fs.statSync(screenshotPath);
  const fileSizeInBytes = stats.size;
  
  // PNG files should be at least a few hundred bytes for a valid image
  expect(fileSizeInBytes).to.be.greaterThan(100);
  console.log(`✓ Screenshot file has valid content: ${fileSizeInBytes} bytes`);
}

function cleanupTestArtifacts(simulatorId: string, screenshotPath: string): void {
  // Keep screenshot file for inspection - don't delete it
  console.log(`Screenshot saved at: ${screenshotPath}`);
  
  // Clean up simulator
  shutdownSimulator(simulatorId);
  deleteSimulator(simulatorId);
  removeFromTracking(simulatorId);
  console.log(`Cleaned up simulator: ${simulatorId}`);
}

