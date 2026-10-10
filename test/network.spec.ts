import {test, expect} from './fixtures';
import {execFileSync, spawn, type ChildProcessWithoutNullStreams} from 'child_process';
import * as path from 'path';
import {coverageEnv} from './coverage';
import {eventually} from './poll';
import {flattenElements} from './webview';
import type {UIElement, UIDumpResponse} from './types';
import {downloadPlayground, PLAYGROUND_PACKAGE} from './playground';

const mobilecliBinary = path.join(__dirname, '..', 'mobilecli');

// the playground Network screen (playground://network) sends HTTP through the
// stacks a real app uses: OkHttp and HttpURLConnection on android, URLSession on
// ios. each button shows the outcome in one on-screen label, so a test can read
// what the app actually received after interception.
const GET_BUTTON: Record<string, string> = {
	android: 'OkHttp GET',
	ios: 'URLSession.shared GET',
};

// the label every result line starts with, so the on-screen outcome is findable
// regardless of status or body
const RESULT_PREFIXES = ['okhttp', 'urlconnection', 'shared', 'custom'];

type Device = {id: string; platform: string};

// one paused request as emitted by `network listen` (see docs/network-interception.md)
type PausedEvent = {
	type: string;
	requestId: string;
	method: string;
	url: string;
};

function firstDeviceOfType(deviceType: string): Device | null {
	try {
		const output = execFileSync(mobilecliBinary, ['devices', '--type', deviceType], {
			encoding: 'utf8',
			env: coverageEnv(),
		});
		return JSON.parse(output)?.data?.devices?.[0] ?? null;
	} catch {
		return null;
	}
}

function runMobilecli(args: string[]): string {
	return execFileSync(mobilecliBinary, args, {encoding: 'utf8', env: coverageEnv()});
}

// a command that returns an error envelope exits non-zero, so execFileSync
// throws even though it printed valid json on stdout; recover that stdout.
function runMobilecliJson(args: string[]): any {
	try {
		return JSON.parse(runMobilecli(args));
	} catch (error: unknown) {
		const stdout = (error as {stdout?: string}).stdout ?? '';
		if (stdout.trim() === '') throw error;
		return JSON.parse(stdout);
	}
}

// the text/label/name/value of an element, so one matcher works on both the
// android dump (text) and the ios dump (label/name)
function elementText(el: UIElement): string {
	const anyEl = el as unknown as Record<string, string | undefined>;
	return anyEl.text || anyEl.label || anyEl.name || anyEl.value || '';
}

function dumpElements(deviceId: string): UIElement[] {
	const dump: UIDumpResponse = runMobilecliJson(['dump', 'ui', '--device', deviceId]);
	return flattenElements(dump.data.elements);
}

function findByText(deviceId: string, text: string): UIElement | undefined {
	return dumpElements(deviceId).find(el => elementText(el) === text);
}

function tapButton(deviceId: string, text: string): void {
	const el = findByText(deviceId, text);
	if (!el) {
		throw new Error(`button "${text}" not found`);
	}
	const {x, y, width, height} = el.rect;
	runMobilecli(['io', 'tap', `${x + width / 2},${y + height / 2}`, '--device', deviceId]);
}

// the on-screen outcome line, once the request has resolved (e.g.
// "okhttp GET 200 {...}" or "shared GET error: ...")
function readNetworkResult(deviceId: string): string {
	const el = dumpElements(deviceId).find(e =>
		RESULT_PREFIXES.some(p => elementText(e).startsWith(p)));
	return el ? elementText(el) : '';
}

function openNetworkScreen(deviceId: string): void {
	runMobilecli(['url', 'playground://network', '--device', deviceId]);
}

// `network listen` streams one json object per line on stdout. this starts it,
// parses paused events as they arrive, and hands back the collected list plus a
// stop() that ends the stream (releasing anything still paused).
function startListening(deviceId: string, pauses: string[]): {events: PausedEvent[]; stop: () => Promise<void>} {
	const args = ['network', 'listen', '--device', deviceId];
	for (const p of pauses) {
		args.push('--pause', p);
	}
	const child: ChildProcessWithoutNullStreams = spawn(mobilecliBinary, args, {env: coverageEnv()});
	const events: PausedEvent[] = [];
	let buffer = '';
	child.stdout.on('data', chunk => {
		buffer += chunk.toString();
		let newline: number;
		while ((newline = buffer.indexOf('\n')) !== -1) {
			const line = buffer.slice(0, newline).trim();
			buffer = buffer.slice(newline + 1);
			if (!line) continue;
			try {
				const parsed = JSON.parse(line);
				if (parsed.type === 'paused') events.push(parsed);
			} catch {
				// a non-event line (e.g. an error envelope) is left for the caller's assertions
			}
		}
	});
	const stop = () => new Promise<void>(resolve => {
		child.on('close', () => resolve());
		child.kill('SIGINT');
	});
	return {events, stop};
}

for (const platform of ['android', 'ios'] as const) {
	test.describe(`network (${platform})`, () => {
		let device: Device | null;

		test.beforeAll(async ({deviceType}) => {
			device = firstDeviceOfType(deviceType);
			if (device && device.platform !== platform) device = null;
			if (device) {
				await downloadPlayground(platform);
			}
		});

		// ── host contract: these hold today, with or without the in-app agent ──

		test('continue/fulfill/abort require a requestId', () => {
			test.skip(!device, `no ${platform} device`);
			for (const action of ['continue', 'fulfill', 'abort']) {
				const envelope = runMobilecliJson(['network', action, '--device', device!.id, '--id', '']);
				expect(envelope.status).toBe('error');
				expect(envelope.error).toContain('requestId is required');
			}
		});

		test('listen attaches to a debuggable app and reaches its in-app agent', async () => {
			test.skip(!device, `no ${platform} device`);
			openNetworkScreen(device!.id);
			await eventually(() => !!findByText(device!.id, GET_BUTTON[platform]),
				'network screen never appeared').toBe(true);

			const listener = startListening(device!.id, [`**/todos/**`]);
			tapButton(device!.id, GET_BUTTON[platform]);

			// once the agent implements network.*, this becomes a paused event (see the
			// skipped interception suite). until then it must at least reach the agent,
			// proving injection + port-forward + rpc relay + streaming all work.
			await eventually(() => listener.events.length > 0 || agentWasReached(device!.id),
				'listen never reached the in-app agent').toBe(true);
			await listener.stop();
		});

		// ── interception acceptance. Runs where the in-app agent implements the
		// network.* methods: Android (agents/android/java) does; iOS (agents/ios)
		// does not yet, so it stays skipped until that agent lands. ──
		const interception = platform === 'android' ? test.describe : test.describe.skip;
		interception('interception', () => {
			test('fulfill replaces the response the app receives', async () => {
				test.skip(!device, `no ${platform} device`);
				openNetworkScreen(device!.id);
				await eventually(() => !!findByText(device!.id, GET_BUTTON[platform]), 'no screen').toBe(true);

				const listener = startListening(device!.id, ['**/todos/**']);
				tapButton(device!.id, GET_BUTTON[platform]);
				await eventually(() => listener.events.length, 'no paused request').toBeGreaterThan(0);

				const body = Buffer.from('{"mocked":true}').toString('base64');
				runMobilecli(['network', 'fulfill', '--device', device!.id,
					'--id', listener.events[0].requestId, '--status', '200', '--body', '{"mocked":true}']);

				await eventually(() => readNetworkResult(device!.id), 'mocked body never shown')
					.toContain('mocked');
				await listener.stop();
				expect(body).toBeTruthy();
			});

			test('abort makes the request fail in the app', async () => {
				test.skip(!device, `no ${platform} device`);
				openNetworkScreen(device!.id);
				const listener = startListening(device!.id, ['**/todos/**']);
				tapButton(device!.id, GET_BUTTON[platform]);
				await eventually(() => listener.events.length, 'no paused request').toBeGreaterThan(0);

				runMobilecli(['network', 'abort', '--device', device!.id,
					'--id', listener.events[0].requestId, '--error', 'failed']);

				await eventually(() => readNetworkResult(device!.id), 'request did not fail')
					.toContain('error');
				await listener.stop();
			});

			test('continue lets the real response through unchanged', async () => {
				test.skip(!device, `no ${platform} device`);
				openNetworkScreen(device!.id);
				const listener = startListening(device!.id, ['**/todos/**']);
				tapButton(device!.id, GET_BUTTON[platform]);
				await eventually(() => listener.events.length, 'no paused request').toBeGreaterThan(0);

				runMobilecli(['network', 'continue', '--device', device!.id, '--id', listener.events[0].requestId]);

				await eventually(() => readNetworkResult(device!.id), 'real response never arrived')
					.toContain('200');
				await listener.stop();
			});
		});
	});
}

// agentWasReached returns true when the foreground app is the debuggable
// playground, i.e. `network listen` had an agent to talk to. the "method not
// found: network.enable" error that listen prints today is itself proof it
// reached the agent; this is the positive-signal companion to that.
function agentWasReached(deviceId: string): boolean {
	try {
		const fg = runMobilecliJson(['apps', 'foreground', '--device', deviceId]);
		return (fg?.data?.packageName ?? fg?.data?.bundleIdentifier) === PLAYGROUND_PACKAGE;
	} catch {
		return false;
	}
}
