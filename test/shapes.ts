import {expect} from '@playwright/test';

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
