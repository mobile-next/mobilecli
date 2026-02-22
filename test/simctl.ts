import {execSync} from 'child_process';

// track created simulators for cleanup
const createdSimulators: string[] = [];

export function findIOSRuntime(majorVersion: string): string {
	try {
		const matchingLines = execSync('xcrun simctl list runtimes', {encoding: 'utf8'})
			.toString()
			.split('\n')
			.filter(line => line.includes(`iOS ${majorVersion}.`) && line.includes('com.apple.CoreSimulator.SimRuntime.'));

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
		const stdout = execSync(`xcrun simctl create "${name}" "${deviceType}" "${runtime}"`, {encoding: 'utf8'}).toString();
		const simulatorId = stdout.trim();
		createdSimulators.push(simulatorId);
		return simulatorId;
	} catch (error) {
		throw new Error(`Failed to create simulator: ${error}`);
	}
}

export function bootSimulator(simulatorId: string): void {
	try {
		execSync(`xcrun simctl boot "${simulatorId}"`, {encoding: 'utf8', stdio: ['pipe', 'pipe', 'pipe']});
	} catch (error: any) {
		// Ignore "already booted" errors — check both message and stderr
		const output = String(error?.message ?? '') + String(error?.stderr ?? '');
		if (!output.includes('current state: Booted')) {
			throw new Error(`Failed to boot simulator: ${error}`);
		}
	}
	execSync(`xcrun simctl bootstatus "${simulatorId}"`, {stdio: 'ignore'});
}

export function waitForSimulatorReady(simulatorId: string, timeout: number = 30000): void {
	const startTime = Date.now();

	while (Date.now() - startTime < timeout) {
		const stdout = execSync(`xcrun simctl list devices`, {encoding: 'utf8'})
			.split('\n')
			.filter(line => line.includes(simulatorId))
			.filter(line => line.includes('(Booted)'));
		if (stdout.length > 0) {
			// verify simulator is actually ready by taking a screenshot
			execSync(`xcrun simctl io "${simulatorId}" screenshot /dev/null 2>/dev/null`);
			return;
		}

		execSync('sleep 1');
	}

	throw new Error(`Simulator did not boot within ${timeout}ms`);
}

export function printAllLogsFromSimulator(simulatorId: string): void {
	try {
		execSync(`xcrun simctl spawn "${simulatorId}" log show -last 5m >/tmp/${simulatorId}.txt`, {
			stdio: 'inherit'
		});
	} catch (error) {
		console.warn(`Warning: Failed to print logs from simulator ${simulatorId}: ${error}`);
	}
}

export function shutdownSimulator(simulatorId: string): void {
	try {
		execSync(`xcrun simctl shutdown "${simulatorId}"`, {encoding: 'utf8'});
	} catch (error) {
		// Simulator might already be shutdown, which is fine
		console.warn(`Warning: Failed to shutdown simulator ${simulatorId}: ${error}`);
	}
}

export function deleteSimulator(simulatorId: string): void {
	try {
		execSync(`xcrun simctl delete "${simulatorId}"`, {encoding: 'utf8'});
		removeFromTracking(simulatorId);
	} catch (error) {
		console.warn(`Warning: Failed to delete simulator ${simulatorId}: ${error}`);
	}
}

export function findExistingSimulator(iosVersion: string, preferredName: string = 'iPhone 17 Pro'): string | null {
	const output = execSync('xcrun simctl list devices', {encoding: 'utf8'});
	const lines = output.split('\n');

	let inTargetRuntime = false;

	for (const line of lines) {
		if (line.startsWith('--')) {
			inTargetRuntime = line.includes(`iOS ${iosVersion}.`);
			continue;
		}

		if (inTargetRuntime && line.includes(preferredName)) {
			const match = line.match(/\(([0-9A-F-]{36})\)/i);
			if (match) {
				return match[1];
			}
		}
	}
	return null;
}

export function createAndLaunchSimulator(iosVersion: string, deviceType: string = 'iPhone 14'): string {
	const runtime = findIOSRuntime(iosVersion);
	const simulatorName = `Test-iOS-${iosVersion}`;

	// console.log(`Creating iOS ${iosVersion} simulator with runtime ${runtime}...`);
	const simulatorId = createSimulator(simulatorName, deviceType, runtime);

	// console.log(`Booting simulator ${simulatorId}...`);
	bootSimulator(simulatorId);

	// console.log('Waiting for simulator to be ready...');
	waitForSimulatorReady(simulatorId);

	// console.log(`Simulator ${simulatorId} is ready!`);
	return simulatorId;
}

export function getOrCreateAndLaunchSimulator(iosVersion: string): { id: string; created: boolean } {
	// Prefer reusing an existing iPhone 17 Pro simulator
	const existingId = findExistingSimulator(iosVersion, 'iPhone 17 Pro');
	if (existingId) {
		bootSimulator(existingId);
		waitForSimulatorReady(existingId);
		return {id: existingId, created: false};
	}

	// Fall back to creating a new Test-iOS-{version} simulator
	const id = createAndLaunchSimulator(iosVersion);
	return {id, created: true};
}

export function cleanupSimulators(): void {
	for (const simulatorId of [...createdSimulators]) {
		shutdownSimulator(simulatorId);
		deleteSimulator(simulatorId);
	}

	createdSimulators.length = 0;
}

export function removeFromTracking(simulatorId: string): void {
	const index = createdSimulators.indexOf(simulatorId);
	if (index > -1) {
		createdSimulators.splice(index, 1);
	}
}
