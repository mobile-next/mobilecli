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
		{name: 'launcher', testMatch: /launcher\.spec\.ts/},
		{name: 'simulator', testMatch: /simulator\.spec\.ts/},
		{name: 'emulator', testMatch: /android\.spec\.ts/, use: {deviceType: 'emulator'}},
		{name: 'android', testMatch: /android\.spec\.ts/, use: {deviceType: 'real'}},
		{name: 'network-simulator', testMatch: /network\.spec\.ts/, use: {deviceType: 'simulator'}},
		{name: 'network-emulator', testMatch: /network\.spec\.ts/, use: {deviceType: 'emulator'}},
		{name: 'network-android', testMatch: /network\.spec\.ts/, use: {deviceType: 'real'}},
	],
});
