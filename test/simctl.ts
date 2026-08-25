import {execSync} from 'child_process';

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
