import { expect } from 'chai';
import { execSync } from 'child_process';
import axios from 'axios';
import * as path from 'path';
import * as fs from 'fs';
import { 
  createAndLaunchSimulator, 
  shutdownSimulator, 
  deleteSimulator, 
  cleanupSimulators,
  findIOSRuntime
} from './simctl';

const TEST_SERVER_URL = 'http://localhost:12001';

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

describe('iOS Simulator Tests', () => {
  after(() => {
    cleanupSimulators();
  });

  ['18.6', '26.0'].forEach((iosVersion) => {
    describe(`iOS ${iosVersion}`, () => {
      let simulatorId: string;
      let runtimeAvailable: boolean = false;

      before(function() {
        this.timeout(180000);
        
        // Check if runtime is available
        try {
          findIOSRuntime(iosVersion);
          runtimeAvailable = true;
          simulatorId = createAndLaunchSimulator(iosVersion);
        } catch (error) {
          console.log(`iOS ${iosVersion} runtime not available, skipping tests: ${error}`);
          runtimeAvailable = false;
        }
      });

      after(() => {
        if (simulatorId) {
          shutdownSimulator(simulatorId);
          deleteSimulator(simulatorId);
        }
      });

      it('should take screenshot', async function() {
        if (!runtimeAvailable) {
          this.skip();
          return;
        }
        
        this.timeout(180000);
        
        const screenshotPath = `/tmp/screenshot-ios${iosVersion}-${Date.now()}.png`;
        
        takeScreenshot(simulatorId, screenshotPath);
        verifyScreenshotFileWasCreated(screenshotPath);
        verifyScreenshotFileHasValidContent(screenshotPath);
        
        // console.log(`Screenshot saved at: ${screenshotPath}`);
      });

      it('should open URL https://example.com', async function() {
        if (!runtimeAvailable) {
          this.skip();
          return;
        }
        
        this.timeout(180000);
        
        openUrl(simulatorId, 'https://example.com');
      });
    });
  });
});

function mobilecli(args: string): void {
  const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
  const command = `${mobilecliBinary} ${args}`;
  
  try {
    const result = execSync(command, { 
      encoding: 'utf8', 
      timeout: 180000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch (error: any) {
    console.log(`Command failed: ${command}`);
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
  mobilecli(`screenshot --device ${simulatorId} --format png --output ${screenshotPath}`);
}

function verifyScreenshotFileWasCreated(screenshotPath: string): void {
  const fileExists = fs.existsSync(screenshotPath);
  expect(fileExists).to.be.true;
  // console.log(`✓ Screenshot file was created: ${screenshotPath}`);
}

function verifyScreenshotFileHasValidContent(screenshotPath: string): void {
  const stats = fs.statSync(screenshotPath);
  const fileSizeInBytes = stats.size;
  
  expect(fileSizeInBytes).to.be.greaterThan(100*1024);
}

function openUrl(simulatorId: string, url: string): void {
  mobilecli(`url "${url}" --device ${simulatorId}`);
}

