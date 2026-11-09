import {execSync} from 'child_process';
import * as os from 'os';
import * as path from 'path';
import {DatabaseSync} from 'node:sqlite';

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

export function grantPrivacyPermission(simulatorId: string, service: string, bundleId: string): void {
	execSync(`xcrun simctl privacy "${simulatorId}" grant ${service} "${bundleId}"`, {encoding: 'utf8'});
}

// the simulator keeps privacy decisions in its own TCC database, one row per (service, app)
export function privacyServicesDecidedFor(simulatorId: string, bundleId: string): string[] {
	const tccDatabase = path.join(os.homedir(), 'Library/Developer/CoreSimulator/Devices', simulatorId, 'data/Library/TCC/TCC.db');
	const db = new DatabaseSync(tccDatabase, {readOnly: true});
	try {
		const rows = db.prepare('SELECT service FROM access WHERE client = ?').all(bundleId) as Array<{service: string}>;
		return rows.map(row => row.service);
	} finally {
		db.close();
	}
}
