import { expect } from 'chai';
import { execSync } from 'child_process';
import axios from 'axios';
import * as path from 'path';
import * as fs from 'fs';
import { 
  createAndLaunchSimulator, 
  shutdownSimulator, 
  deleteSimulator, 
  cleanupSimulators
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

  ['16', '17', '18'].forEach((iosVersion) => {
    describe(`iOS ${iosVersion}`, () => {
      let simulatorId: string;

      before(function() {
        this.timeout(60000);
        simulatorId = createAndLaunchSimulator(iosVersion);
      });

      after(() => {
        if (simulatorId) {
          shutdownSimulator(simulatorId);
          deleteSimulator(simulatorId);
        }
      });

      it('should take screenshot', async function() {
        this.timeout(60000);
        
        const screenshotPath = `/tmp/screenshot-ios${iosVersion}.png`;
        
        cleanupExistingScreenshotFile(screenshotPath);
        
        takeScreenshotUsingMobileCLI(simulatorId, screenshotPath);
        verifyScreenshotFileWasCreated(screenshotPath);
        verifyScreenshotFileHasValidContent(screenshotPath);
        
        // console.log(`Screenshot saved at: ${screenshotPath}`);
      });

      it('should open URL https://example.com', async function() {
        this.timeout(60000);
        
        openURLUsingMobileCLI(simulatorId, 'https://example.com');
      });
    });
  });
});

// Screenshot test helper functions with descriptive English names
function cleanupExistingScreenshotFile(screenshotPath: string): void {
  if (fs.existsSync(screenshotPath)) {
    fs.unlinkSync(screenshotPath);
  }
}

function executeMobileCLI(args: string, description: string): void {
  const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');
  const command = `${mobilecliBinary} ${args}`;
  
  try {
    const result = execSync(command, { 
      encoding: 'utf8', 
      timeout: 60000,
      stdio: ['pipe', 'pipe', 'pipe']
    });
  } catch (error: any) {
    console.log(`${description} command failed: ${command}`);
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

function takeScreenshotUsingMobileCLI(simulatorId: string, screenshotPath: string): void {
  executeMobileCLI(
    `screenshot --device ${simulatorId} --format png --output ${screenshotPath}`,
    'Screenshot'
  );
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

function openURLUsingMobileCLI(simulatorId: string, url: string): void {
  executeMobileCLI(
    `url "${url}" --device ${simulatorId}`,
    'URL'
  );
}

