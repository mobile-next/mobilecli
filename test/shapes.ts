import {expect} from '@playwright/test';
import type {InstallResult, InstalledApp, UninstallResult} from './types';

// Shared wire-format assertions.
//
// Each entity is described in exactly one place, so a field that disappears from
// the protocol fails every spec that touches that entity — not just the handful
// of tests that happened to read the missing field.

export function expectDeviceShape(device: any): void {
	expect(typeof device.id, `device.id: ${JSON.stringify(device)}`).toBe('string');
	expect(device.id.length).toBeGreaterThan(0);
	expect(typeof device.name).toBe('string');
	expect(['ios', 'android']).toContain(device.platform);
	expect(typeof device.type).toBe('string');
	expect(device.type.length).toBeGreaterThan(0);
	expect(typeof device.version).toBe('string');
	expect(['online', 'offline']).toContain(device.state);
}

// android reports only a package name; ios adds a display name and version
export function expectAppShape(app: any): void {
	expect(typeof app.packageName, `app.packageName: ${JSON.stringify(app)}`).toBe('string');
	expect(app.packageName.length).toBeGreaterThan(0);
	if (app.appName !== undefined) {
		expect(typeof app.appName).toBe('string');
	}
	if (app.version !== undefined) {
		expect(typeof app.version).toBe('string');
	}
}

export function expectForegroundAppShape(foreground: any): void {
	expect(typeof foreground.packageName).toBe('string');
	expect(foreground.packageName.length).toBeGreaterThan(0);
	expect(typeof foreground.appName).toBe('string');
	// ForegroundAppInfo.Version has no omitempty, so the field is always present
	// even when the platform cannot determine a version
	expect(typeof foreground.version).toBe('string');
}

// both platforms return a tree; only `type` and `rect` are common to all of
// them, since android carries text/identifier and ios carries label/name/value
export function expectUIElementShape(element: any): void {
	expect(typeof element.type, `element.type: ${JSON.stringify(element?.rect)}`).toBe('string');
	expect(element.type.length).toBeGreaterThan(0);

	expect(element.rect, `element missing rect: ${element.type}`).toBeDefined();
	for (const field of ['x', 'y', 'width', 'height']) {
		expect(typeof element.rect[field], `rect.${field} on ${element.type}`).toBe('number');
	}
	expect(element.rect.width).toBeGreaterThanOrEqual(0);
	expect(element.rect.height).toBeGreaterThanOrEqual(0);

	if (element.children !== undefined) {
		expect(Array.isArray(element.children)).toBe(true);
		for (const child of element.children) {
			expectUIElementShape(child);
		}
	}
}

export function expectUIDumpShape(elements: any): void {
	expect(Array.isArray(elements)).toBe(true);
	expect(elements.length).toBeGreaterThan(0);
	for (const element of elements) {
		expectUIElementShape(element);
	}
}

export function expectAgentShape(agent: any): void {
	expect(agent, 'response is missing the agent descriptor').toBeDefined();
	expect(typeof agent.version).toBe('string');
	expect(agent.version.length).toBeGreaterThan(0);
	expect(typeof agent.bundleId).toBe('string');
	expect(agent.bundleId.length).toBeGreaterThan(0);
}

export function expectFsEntryShape(entry: any): void {
	expect(typeof entry.name, `entry.name: ${JSON.stringify(entry)}`).toBe('string');
	expect(entry.name.length).toBeGreaterThan(0);
	expect(typeof entry.path).toBe('string');
	expect(entry.path.length).toBeGreaterThan(0);
	expect(typeof entry.size).toBe('number');
	expect(entry.size).toBeGreaterThanOrEqual(0);
	expect(typeof entry.isDir).toBe('boolean');
	expect(Number.isNaN(Date.parse(entry.modTime)), `entry.modTime not a date: ${entry.modTime}`).toBe(false);
}

export function expectFsListingShape(entries: any): void {
	expect(Array.isArray(entries)).toBe(true);
	for (const entry of entries) {
		expectFsEntryShape(entry);
	}
}

// every cli command that speaks json wraps its payload in this envelope
export function expectOkEnvelope(response: any): any {
	expect(response, 'expected a json envelope').toBeDefined();
	expect(response.status, `expected status ok, got: ${JSON.stringify(response)}`).toBe('ok');
	expect(response.data, 'ok envelope must carry data').toBeDefined();
	return response.data;
}

// `apps list` reports all four fields on both platforms today. expectAppShape stays
// lenient for arbitrary system apps; this one is the strict contract, asserted
// against an app we ship ourselves, so dropping a field is caught as a regression.
export function expectInstalledAppShape(app: unknown): asserts app is InstalledApp {
	const fields: (keyof InstalledApp)[] = ['packageName', 'appName', 'version', 'versionCode'];
	const record = app as Record<string, unknown>;
	for (const field of fields) {
		expect(typeof record?.[field], `app.${field}: ${JSON.stringify(app)}`).toBe('string');
		expect((record[field] as string).length, `app.${field} is empty`).toBeGreaterThan(0);
	}
}

// `apps install` echoes a human-readable message plus the metadata it read out of
// the artifact. it reports no appName, unlike `apps list`.
export function expectInstallResultShape(data: unknown): asserts data is InstallResult {
	const result = data as {message?: unknown; app?: Record<string, unknown>};
	expect(typeof result?.message, `install message: ${JSON.stringify(data)}`).toBe('string');
	expect((result.message as string).length).toBeGreaterThan(0);

	const fields: (keyof InstallResult['app'])[] = ['packageName', 'version', 'versionCode'];
	for (const field of fields) {
		expect(typeof result?.app?.[field], `install app.${field}: ${JSON.stringify(data)}`).toBe('string');
		expect((result.app![field] as string).length, `install app.${field} is empty`).toBeGreaterThan(0);
	}
}

// `apps uninstall` answers with just the bundle id it removed
export function expectUninstallResultShape(data: unknown): asserts data is UninstallResult {
	const result = data as {packageName?: unknown};
	expect(typeof result?.packageName, `uninstall result: ${JSON.stringify(data)}`).toBe('string');
	expect((result.packageName as string).length).toBeGreaterThan(0);
}

// the error half of the envelope: a failing command exits non-zero and prints
// this instead of `data`
export function expectErrorEnvelope(response: unknown): string {
	const envelope = response as {status?: unknown; error?: unknown};
	expect(envelope?.status, `expected status error, got: ${JSON.stringify(response)}`).toBe('error');
	expect(typeof envelope.error, 'error envelope must carry a message').toBe('string');
	return envelope.error as string;
}
