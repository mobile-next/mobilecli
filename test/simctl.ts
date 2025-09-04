import { execSync } from 'child_process';

let createdSimulators: string[] = []; // Track created simulators for cleanup

// Simulator management functions
export function findIOSRuntime(majorVersion: string): string {
  try {
    const stdout = execSync('xcrun simctl list runtimes', { encoding: 'utf8' });
    const lines = stdout.split('\n');
    const matchingLines = lines.filter(line => 
      line.includes(`iOS ${majorVersion}.`) && 
      line.includes('com.apple.CoreSimulator.SimRuntime.')
    );
    
    const runtime = matchingLines[0]?.split(' ').pop();
    
    if (!runtime) {
      throw new Error(`No iOS ${majorVersion} runtime found`);
    }
    return runtime;
  } catch (error) {
    throw new Error(`Failed to find iOS ${majorVersion} runtime: ${error}`);
  }
}

export function createSimulator(name: string, deviceType: string, runtime: string): string {
  try {
    const stdout = execSync(`xcrun simctl create "${name}" "${deviceType}" "${runtime}"`, 
      { encoding: 'utf8' });
    const simulatorId = stdout.trim();
    createdSimulators.push(simulatorId);
    return simulatorId;
  } catch (error) {
    throw new Error(`Failed to create simulator: ${error}`);
  }
}

export function bootSimulator(simulatorId: string): void {
  try {
    execSync(`xcrun simctl boot "${simulatorId}"`, { encoding: 'utf8' });
  } catch (error) {
    // Simulator might already be booted, check if it's actually an error
    const errorMessage = (error as Error).message;
    if (!errorMessage.includes('current state: Booted')) {
      throw new Error(`Failed to boot simulator: ${error}`);
    }
  }
}

export function waitForSimulatorReady(simulatorId: string, timeout: number = 30000): void {
  const startTime = Date.now();
  
  while (Date.now() - startTime < timeout) {
    try {
      const stdout = execSync(`xcrun simctl list devices | grep "${simulatorId}"`, 
        { encoding: 'utf8' });
      if (stdout.includes('(Booted)')) {
        // see if we can interact with status_bar, gets us closer to a functioning simulator
        execSync(`xcrun simctl status_bar "${simulatorId}" list`);
        return;
      }
    } catch (error) {
      // Continue waiting
    }
    
    // Wait 1 second before next attempt
    execSync('sleep 1');
  }
  
  throw new Error(`Simulator did not boot within ${timeout}ms`);
}

export function shutdownSimulator(simulatorId: string): void {
  try {
    execSync(`xcrun simctl shutdown "${simulatorId}"`, { encoding: 'utf8' });
  } catch (error) {
    // Simulator might already be shutdown, which is fine
    console.warn(`Warning: Failed to shutdown simulator ${simulatorId}: ${error}`);
  }
}

export function deleteSimulator(simulatorId: string): void {
  try {
    execSync(`xcrun simctl delete "${simulatorId}"`, { encoding: 'utf8' });
    removeFromTracking(simulatorId);
  } catch (error) {
    console.warn(`Warning: Failed to delete simulator ${simulatorId}: ${error}`);
  }
}

export function createAndLaunchSimulator(iosVersion: string, deviceType: string = 'iPhone 14'): string {
  const runtime = findIOSRuntime(iosVersion);
  const simulatorName = `Test-iOS-${iosVersion}-${Date.now()}`;
  
  // console.log(`Creating iOS ${iosVersion} simulator with runtime ${runtime}...`);
  const simulatorId = createSimulator(simulatorName, deviceType, runtime);
  
  // console.log(`Booting simulator ${simulatorId}...`);
  bootSimulator(simulatorId);
  
  // console.log('Waiting for simulator to be ready...');
  waitForSimulatorReady(simulatorId);
  
  // console.log(`Simulator ${simulatorId} is ready!`);
  return simulatorId;
}

export function cleanupSimulators(): void {
  for (const simulatorId of createdSimulators) {
    shutdownSimulator(simulatorId);
    deleteSimulator(simulatorId);
  }
  createdSimulators = [];
}

export function removeFromTracking(simulatorId: string): void {
  const index = createdSimulators.indexOf(simulatorId);
  if (index > -1) {
    createdSimulators.splice(index, 1);
  }
}
