import {test as base} from '@playwright/test';

// mirrors the values `mobilecli devices --type` accepts
export type DeviceTypeOptions = {
	deviceType: 'emulator' | 'simulator' | 'real';
};

// lets one spec serve both the emulator and a physical handset: the playwright
// project supplies the value, so the tests never hardcode which one they target.
// worker-scoped so it is readable from test.beforeAll, not just from a test body.
export const test = base.extend<{}, DeviceTypeOptions>({
	deviceType: ['emulator', {option: true, scope: 'worker'}],
});

export {expect} from '@playwright/test';
