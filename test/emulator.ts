import {expect} from 'chai';
import {execFileSync} from 'child_process';
import * as path from 'path';
import * as fs from 'fs';
import * as os from 'os';

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
		const output = execFileSync(mobilecliBinary, ['devices'], {encoding: 'utf8'});
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
		mobilecli(['screenshot', '--device', device.id, '--format', 'png', '--output', screenshotPath]);

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

		mobilecli(['url', 'https://example.com', '--device', device.id]);
	});

	it('should get device info', function () {
		if (!device) {
			this.skip();
			return;
		}

		this.timeout(30000);

		mobilecli(['device', 'info', '--device', device.id]);
	});

	describe('fs operations on /sdcard/Download', () => {
		const remoteDir = '/sdcard/Download/mobilecli-test';
		const remoteFile = `${remoteDir}/hello.txt`;

		it('should create a nested directory with mkdir -p', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			fsMkdir(device!.id, remoteDir, true);
		});

		it('should push a file into /sdcard/Download', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const localFile = writeTempFile('hello from mobilecli');
			fsPush(device!.id, localFile, remoteFile);
			fs.unlinkSync(localFile);
		});

		it('should list the pushed file in /sdcard/Download', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const entries = fsList(device!.id, remoteDir);
			const names = entries.map((e: any) => e.name);
			expect(names).to.include('hello.txt');
		});

		it('should pull the file back and verify contents match', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const localDest = path.join(os.tmpdir(), `mobilecli-pull-${Date.now()}.txt`);
			fsPull(device!.id, remoteFile, localDest);
			const contents = fs.readFileSync(localDest, 'utf8');
			expect(contents.trim()).to.equal('hello from mobilecli');
			fs.unlinkSync(localDest);
		});

		it('should remove the test directory recursively', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			fsRm(device!.id, remoteDir, true);
			const entries = fsList(device!.id, '/sdcard/Download');
			const names = entries.map((e: any) => e.name);
			expect(names).to.not.include('mobilecli-test');
		});
	});

	describe('fs operations on app container (com.mobilenext.mobilewright_demo)', () => {
		const packageName = 'com.mobilenext.mobilewright_demo';
		let containerPath: string;
		let remoteDir: string;
		let remoteFile: string;

		before(function () {
			if (!device) return;
			containerPath = getAppContainerPath(device!.id, packageName);
			remoteDir = `${containerPath}/files/mobilecli-test`;
			remoteFile = `${remoteDir}/data.txt`;
		});

		it('should return a valid container path for com.mobilenext.mobilewright_demo', function () {
			if (!device) { this.skip(); return; }
			expect(containerPath).to.match(/^\/data\/user\/\d+\/com\.mobilenext\.mobilewright_demo/);
		});

		it('should list the app container root', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const entries = fsList(device!.id, containerPath);
			expect(entries).to.be.an('array');
		});

		it('should create a directory inside the app container', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			fsMkdir(device!.id, remoteDir, true);
		});

		it('should push a file into the app container', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const localFile = writeTempFile('app container test');
			fsPush(device!.id, localFile, remoteFile);
			fs.unlinkSync(localFile);
		});

		it('should list the file inside the app container', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const entries = fsList(device!.id, remoteDir);
			const names = entries.map((e: any) => e.name);
			expect(names).to.include('data.txt');
		});

		it('should pull the file from the app container and verify contents', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			const localDest = path.join(os.tmpdir(), `mobilecli-pull-app-${Date.now()}.txt`);
			fsPull(device!.id, remoteFile, localDest);
			const contents = fs.readFileSync(localDest, 'utf8');
			expect(contents.trim()).to.equal('app container test');
			fs.unlinkSync(localDest);
		});

		it('should remove the test directory from the app container', function () {
			if (!device) { this.skip(); return; }
			this.timeout(30000);
			fsRm(device!.id, remoteDir, true);
			const entries = fsList(device!.id, `${containerPath}/files`);
			const names = entries.map((e: any) => e.name);
			expect(names).to.not.include('mobilecli-test');
		});
	});
});

function mobilecli(args: string[]): void {
	try {
		execFileSync(mobilecliBinary, args, {
			encoding: 'utf8',
			timeout: 180000,
			stdio: ['pipe', 'pipe', 'pipe'],
		});
	} catch (error: any) {
		console.log(`Command failed: ${mobilecliBinary} ${args.join(' ')}`);
		if (error.stderr) console.log(`stderr: ${error.stderr}`);
		if (error.stdout) console.log(`stdout: ${error.stdout}`);
		throw error;
	}
}

function mobilecliJson(args: string[]): any {
	const result = execFileSync(mobilecliBinary, args, {
		encoding: 'utf8',
		timeout: 60000,
		stdio: ['pipe', 'pipe', 'pipe'],
		env: { ANDROID_HOME: process.env.ANDROID_HOME || '' },
	});
	return JSON.parse(result);
}

function getAppContainerPath(deviceId: string, packageName: string): string {
	const response = mobilecliJson(['apps', 'path', packageName, '--device', deviceId]);
	expect(response.status).to.equal('ok');
	return response.data.path;
}

function fsList(deviceId: string, remotePath: string): any[] {
	const response = mobilecliJson(['fs', 'ls', '--device', deviceId, remotePath]);
	expect(response.status).to.equal('ok');
	return response.data;
}

function fsPush(deviceId: string, localPath: string, remotePath: string): void {
	mobilecli(['fs', 'push', '--device', deviceId, localPath, remotePath]);
}

function fsPull(deviceId: string, remotePath: string, localPath: string): void {
	mobilecli(['fs', 'pull', '--device', deviceId, remotePath, localPath]);
}

function fsMkdir(deviceId: string, remotePath: string, parents: boolean): void {
	mobilecli(['fs', 'mkdir', '--device', deviceId, ...(parents ? ['-p'] : []), remotePath]);
}

function fsRm(deviceId: string, remotePath: string, recursive: boolean): void {
	mobilecli(['fs', 'rm', '--device', deviceId, ...(recursive ? ['-r'] : []), remotePath]);
}

function writeTempFile(content: string): string {
	const tmpPath = path.join(os.tmpdir(), `mobilecli-push-${Date.now()}.txt`);
	fs.writeFileSync(tmpPath, content, 'utf8');
	return tmpPath;
}
