import { execSync } from 'child_process';
import { readdirSync } from 'fs';

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

export function createEmulator(name: string, systemImage: string, deviceProfile: string = 'pixel_9'): string {
	try {
		// First check if AVD already exists
		const existingAvds = execSync(`${EMULATOR_PATH} -list-avds`, { encoding: 'utf8' });
		if (existingAvds.includes(name)) {
			throw new Error(`AVD ${name} already exists`);
		}

		// Create the AVD using avdmanager if available, otherwise create manually
		try {
			execSync(`echo "no" | ${ANDROID_HOME}/cmdline-tools/latest/bin/avdmanager create avd -n "${name}" -k "${systemImage}" -d "${deviceProfile}"`,
				{ encoding: 'utf8' });
		} catch (avdError) {
			// If avdmanager is not available, we'll use the existing AVD as a template
			console.warn('avdmanager not found, will clone existing AVD configuration');
			cloneExistingAvd(name, deviceProfile);
		}

		createdEmulators.push(name);
		return name;
	} catch (error) {
		throw new Error(`Failed to create emulator: ${error}`);
	}
}

function cloneExistingAvd(newName: string, deviceProfile: string): void {
	try {
		// Use existing Pixel AVD as template if available
		const existingAvds = execSync(`${EMULATOR_PATH} -list-avds`, { encoding: 'utf8' });
		const templateAvd = existingAvds.split('\n').find(avd => avd.includes('Pixel')) || 'Pixel_6';

		// Copy AVD configuration files
		const homeDir = process.env.HOME;
		const avdDir = `${homeDir}/.android/avd`;

		execSync(`cp -r "${avdDir}/${templateAvd}.avd" "${avdDir}/${newName}.avd"`);
		execSync(`cp "${avdDir}/${templateAvd}.ini" "${avdDir}/${newName}.ini"`);

		// Update the .ini file to point to the new AVD
		const iniContent = execSync(`cat "${avdDir}/${newName}.ini"`, { encoding: 'utf8' });
		const updatedIni = iniContent.replace(new RegExp(templateAvd, 'g'), newName);
		execSync(`echo '${updatedIni}' > "${avdDir}/${newName}.ini"`);

		// Update config.ini in the AVD directory
		const configPath = `${avdDir}/${newName}.avd/config.ini`;
		const configContent = execSync(`cat "${configPath}"`, { encoding: 'utf8' });
		const updatedConfig = configContent
			.replace(/AvdId=.*/g, `AvdId=${newName}`)
			.replace(/avd.ini.displayname=.*/g, `avd.ini.displayname=${newName}`);
		execSync(`echo '${updatedConfig}' > "${configPath}"`);

	} catch (error) {
		throw new Error(`Failed to clone existing AVD: ${error}`);
	}
}

export function launchEmulator(emulatorName: string): void {
	try {
		// Launch emulator in background
		execSync(`${EMULATOR_PATH} -avd "${emulatorName}" -no-snapshot-save -wipe-data > /dev/null 2>&1 &`,
			{ encoding: 'utf8' });

		// Give it a moment to start
		execSync('sleep 3');
	} catch (error) {
		throw new Error(`Failed to launch emulator: ${error}`);
	}
}

export function waitForEmulatorReady(emulatorName: string, timeout: number = 120000): string {
	const startTime = Date.now();
	let deviceId = '';

	console.log(`Waiting for emulator ${emulatorName} to be ready...`);

	while (Date.now() - startTime < timeout) {
		try {
			// Check if emulator is listed in adb devices
			const devices = execSync(`${ADB_PATH} devices`, { encoding: 'utf8' });
			const deviceLines = devices.split('\n').filter(line => line.includes('device') && !line.includes('List'));

			for (const line of deviceLines) {
				const parts = line.split('\t');
				if (parts.length >= 2 && parts[1].trim() === 'device') {
					deviceId = parts[0].trim();

					try {
						const bootCompleted = execSync(`${ADB_PATH} -s ${deviceId} shell getprop sys.boot_completed`, { encoding: 'utf8', timeout: 5000 }).trim();
						const bootAnim = execSync(`${ADB_PATH} -s ${deviceId} shell getprop init.svc.bootanim`, { encoding: 'utf8', timeout: 5000 }).trim();

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
		// Try graceful shutdown first
		execSync(`${ADB_PATH} -s ${deviceId} emu kill`, { encoding: 'utf8' });
	} catch (error) {
		try {
			// Force kill if graceful shutdown fails
			const processes = execSync(`ps aux | grep "${deviceId}" | grep -v grep`, { encoding: 'utf8' });
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

export function createAndLaunchEmulator(apiLevel: string = '36', deviceProfile: string = 'pixel_9'): { name: string, deviceId: string } {
	const systemImage = findAndroidSystemImage(apiLevel);
	const emulatorName = `Test-Android-${apiLevel}-${Date.now()}`;

	console.log(`Creating Android API ${apiLevel} emulator ${emulatorName}...`);
	createEmulator(emulatorName, systemImage, deviceProfile);

	console.log(`Launching emulator ${emulatorName}...`);
	launchEmulator(emulatorName);

	console.log('Waiting for emulator to be ready...');
	const deviceId = waitForEmulatorReady(emulatorName);

	console.log(`Emulator ${emulatorName} is ready with device ID ${deviceId}!`);
	return { name: emulatorName, deviceId };
}

export function cleanupEmulators(): void {
	// First try to shutdown all running emulators
	try {
		const devices = execSync(`${ADB_PATH} devices`, { encoding: 'utf8' });
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
		const stdout = execSync(`${EMULATOR_PATH} -list-avds`, { encoding: 'utf8' });
		return stdout.trim().split('\n').filter(line => line.trim().length > 0);
	} catch (error) {
		throw new Error(`Failed to list available emulators: ${error}`);
	}
}
