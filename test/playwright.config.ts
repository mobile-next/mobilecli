import {defineConfig} from '@playwright/test';

export default defineConfig({
	testDir: './',
	workers: 1,
	retries: 0,
	fullyParallel: false,
	timeout: 180000,
	reporter: 'list',
	projects: [
		{name: 'server', testMatch: /server\.spec\.ts/},
		{name: 'simulator', testMatch: /simulator\.spec\.ts/},
		{name: 'emulator', testMatch: /emulator\.spec\.ts/},
	],
});
