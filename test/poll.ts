import {expect} from '@playwright/test';

// A device does not settle on demand: an app takes time to come to the
// foreground, a webview takes time to commit a navigation, and a screen takes
// time to draw. A fixed sleep either guesses too low and fails on a slow
// emulator, or guesses too high and pads every run.
//
// Ten attempts, one second apart, is long enough for the slowest of these and
// returns immediately when the device is ready.
const ATTEMPTS = 10;
const INTERVAL_MS = 1000;

const POLL_OPTIONS = {
	timeout: ATTEMPTS * INTERVAL_MS,
	intervals: new Array(ATTEMPTS).fill(INTERVAL_MS),
};

// Reads the device repeatedly until the assertion chained onto it passes:
//
//   await eventually(() => foregroundPackage(id), 'settings never came up').toBe(SETTINGS_PACKAGE);
//
// The returned value is a normal expect matcher, so any assertion works.
export function eventually<T>(read: () => T, message: string) {
	return expect.poll(read, {...POLL_OPTIONS, message});
}
