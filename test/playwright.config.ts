import {defineConfig} from '@playwright/test';
import type {DeviceTypeOptions} from './fixtures';

export default defineConfig<{}, DeviceTypeOptions>({
	testDir: './',
	workers: 1,
	retries: 0,
	fullyParallel: false,
	timeout: 180000,
	reporter: 'list',
	projects: [
		{name: 'server', testMatch: /server\.spec\.ts/},
		{name: 'daemon', testMatch: /daemon\.spec\.ts/},
		{name: 'simulator', testMatch: /simulator\.spec\.ts/},
		{name: 'emulator', testMatch: /android\.spec\.ts/, use: {deviceType: 'emulator'}},
		{name: 'android', testMatch: /android\.spec\.ts/, use: {deviceType: 'real'}},
	],
});
