import {execSync} from 'child_process';
import {readdirSync} from 'fs';

const ANDROID_HOME = process.env.ANDROID_HOME;
const EMULATOR_PATH = `${ANDROID_HOME}/emulator/emulator`;
const ADB_PATH = `${ANDROID_HOME}/platform-tools/adb`;

let createdEmulators: string[] = []; // Track created emulators for cleanup

// Emulator management functions
export function findAndroidSystemImage(apiLevel: string): string {
	try {
		const deviceTypes = ['google_apis_playstore', 'google_apis'];

		for (const type of deviceTypes) {
			try {
				const systemImagePath = `${ANDROID_HOME}/system-images/android-${apiLevel}/${type}`;
				const files = readdirSync(systemImagePath);

				const hasX86_64 = files.includes('x86_64');
				const hasX86 = files.includes('x86');
				const hasArm64 = files.includes('arm64-v8a');

				if (!hasX86_64 && !hasX86 && !hasArm64) {
					continue;
				}

				let arch = 'x86_64';
				if (!hasX86_64) {
					if (hasX86) {
						arch = 'x86';
					} else if (hasArm64) {
						arch = 'arm64-v8a';
					}
				}

				return `system-images;android-${apiLevel};${type};${arch}`;
			} catch (e) {
				continue;
			}
		}

		throw new Error(`No compatible system image found for API ${apiLevel}`);
	} catch (error) {
		throw new Error(`Failed to find Android API ${apiLevel} system image: ${error}`);
	}
}

function listAvds(): Array<String> {
	return execSync(`${EMULATOR_PATH} -list-avds`, {encoding: 'utf8'})
		.toString()
		.split("\n");
}

export function createEmulator(name: string, systemImage: string, deviceProfile: string = 'pixel'): string {
	try {
		if (listAvds().includes(name)) {
			throw new Error(`AVD ${name} already exists`);
		}

		execSync(`echo "no" | ${ANDROID_HOME}/cmdline-tools/latest/bin/avdmanager create avd -n "${name}" -k "${systemImage}" -d "${deviceProfile}"`, {encoding: 'utf8'});

		createdEmulators.push(name);
		return name;
	} catch (error) {
		throw new Error(`Failed to create emulator: ${error}`);
	}
}

export function launchEmulator(emulatorName: string): void {
	try {
		execSync(`${EMULATOR_PATH} -avd "${emulatorName}" -no-snapshot-save -wipe-data 2>&1 >/dev/null &`,
			{encoding: 'utf8'});

		execSync('sleep 60');
	} catch (error) {
		throw new Error(`Failed to launch emulator: ${error}`);
	}
}

export function waitForEmulatorReady(emulatorName: string, timeout: number = 180000): string {
	const startTime = Date.now();
	let deviceId = '';

	console.log(`Waiting for emulator ${emulatorName} to be ready...`);

	while (Date.now() - startTime < timeout) {
		try {
			// Check if emulator is listed in adb devices
			const devices = execSync(`${ADB_PATH} devices`, {encoding: 'utf8'});
			const deviceLines = devices
				.split('\n')
				.filter(line => line.includes('device') && !line.includes('List'));

			for (const line of deviceLines) {
				const parts = line.split('\t');
				if (parts.length >= 2 && parts[1].trim() === 'device') {
					deviceId = parts[0].trim();

					try {
						const bootCompleted = execSync(`${ADB_PATH} -s ${deviceId} shell getprop sys.boot_completed`, {
							encoding: 'utf8',
							timeout: 5000
						}).trim();
						const bootAnim = execSync(`${ADB_PATH} -s ${deviceId} shell getprop init.svc.bootanim`, {
							encoding: 'utf8',
							timeout: 5000
						}).trim();

						if (bootCompleted === '1' && bootAnim === 'stopped') {
							console.log(`Emulator ${emulatorName} is ready with device ID: ${deviceId}`);
							return deviceId;
						}
					} catch (propError) {
					}
				}
			}
		} catch (error) {
			// Continue waiting
		}

		// Wait 2 seconds before next attempt
		execSync('sleep 2');
	}

	throw new Error(`Emulator did not become ready within ${timeout}ms`);
}

export function shutdownEmulator(deviceId: string): void {
	try {
		execSync(`${ADB_PATH} -s ${deviceId} emu kill`, {encoding: 'utf8'});
	} catch (error) {
		try {
			// Force kill if graceful shutdown fails
			const processes = execSync(`ps aux | grep "${deviceId}" | grep -v grep`, {encoding: 'utf8'});
			if (processes.trim()) {
				execSync(`pkill -f "${deviceId}"`);
			}
		} catch (killError) {
			console.warn(`Warning: Failed to shutdown emulator ${deviceId}: ${error}`);
		}
	}
}

export function deleteEmulator(emulatorName: string): void {
	try {
		const homeDir = process.env.HOME;
		const avdDir = `${homeDir}/.android/avd`;

		// Remove AVD directory and ini file
		execSync(`rm -rf "${avdDir}/${emulatorName}.avd"`);
		execSync(`rm -f "${avdDir}/${emulatorName}.ini"`);

		removeFromTracking(emulatorName);
	} catch (error) {
		console.warn(`Warning: Failed to delete emulator ${emulatorName}: ${error}`);
	}
}

export function createAndLaunchEmulator(apiLevel: string = '36', deviceProfile: string = 'pixel'): { name: string, deviceId: string } {
	const systemImage = findAndroidSystemImage(apiLevel);
	const emulatorName = `Test-Android-${apiLevel}-${Date.now()}`;

	console.log(`Creating Android API ${apiLevel} emulator ${emulatorName}...`);
	createEmulator(emulatorName, systemImage, deviceProfile);

	launchEmulator(emulatorName);
	const deviceId = waitForEmulatorReady(emulatorName);

	console.log(`Emulator ${emulatorName} is ready with device ID ${deviceId}!`);
	return {name: emulatorName, deviceId};
}

export function cleanupEmulators(): void {
	// First try to shutdown all running emulators
	try {
		const devices = execSync(`${ADB_PATH} devices`, {encoding: 'utf8'});
		const deviceLines = devices.split('\n').filter(line => line.includes('device') && !line.includes('List'));

		for (const line of deviceLines) {
			const parts = line.split('\t');
			if (parts.length >= 2 && parts[1].trim() === 'device') {
				shutdownEmulator(parts[0].trim());
			}
		}
	} catch (error) {
		console.warn('Warning: Failed to shutdown running emulators during cleanup');
	}

	// Delete tracked emulators
	for (const emulatorName of createdEmulators) {
		deleteEmulator(emulatorName);
	}
	createdEmulators = [];
}

export function removeFromTracking(emulatorName: string): void {
	const index = createdEmulators.indexOf(emulatorName);
	if (index > -1) {
		createdEmulators.splice(index, 1);
	}
}

export function getAvailableEmulators(): string[] {
	try {
		const stdout = execSync(`${EMULATOR_PATH} -list-avds`, {encoding: 'utf8'});
		return stdout.trim().split('\n').filter(line => line.trim().length > 0);
	} catch (error) {
		throw new Error(`Failed to list available emulators: ${error}`);
	}
}
