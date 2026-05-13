import {expect} from 'chai';
import {execSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';

const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');

type Device = {
	id: string;
	name: string;
	platform: string;
	type: string;
	version: string;
	state: string;
};

function getFirstAndroidDevice(): Device | null {
	try {
		const output = execSync(`${mobilecliBinary} devices`, {encoding: 'utf8'});
		const result = JSON.parse(output);
		return result?.data?.devices?.find((d: Device) => d.platform === 'android') ?? null;
	} catch (error) {
		return null;
	}
}

describe('Android Emulator Tests', () => {
	let device: Device | null;

	before(function () {
		device = getFirstAndroidDevice();
		if (!device) {
			console.log('No Android device found. See test/README.md for setup instructions.');
		}
	});

	it('should take screenshot', function () {
		if (!device) {
			this.skip();
			return;
		}

		this.timeout(30000);

		const screenshotPath = `/tmp/screenshot-android-${Date.now()}.png`;
		mobilecli(`screenshot --device ${device.id} --format png --output ${screenshotPath}`);

		const fileExists = fs.existsSync(screenshotPath);
		expect(fileExists).to.be.true;

		const stats = fs.statSync(screenshotPath);
		expect(stats.size).to.be.greaterThan(64 * 1024);
	});

	it('should open URL https://example.com', function () {
		if (!device) {
			this.skip();
			return;
		}

		this.timeout(30000);

		mobilecli(`url "https://example.com" --device ${device.id}`);
	});

	it('should get device info', function () {
		if (!device) {
			this.skip();
			return;
		}

		this.timeout(30000);

		mobilecli(`device info --device ${device.id}`);
	});
});

function mobilecli(args: string): void {
	const command = `${mobilecliBinary} ${args}`;

	try {
		execSync(command, {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
		});
	} catch (error: any) {
		console.log(`Command failed: ${command}`);
		if (error.stderr) console.log(`stderr: ${error.stderr}`);
		if (error.stdout) console.log(`stdout: ${error.stdout}`);
		throw error;
	}
}
