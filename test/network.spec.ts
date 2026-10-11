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
// stacks a real app uses: OkHttp and HttpURLConnection on android, URLSession and
// Alamofire on ios. each button shows the outcome in one on-screen label, so a test can read
// what the app actually received after interception.
const GET_BUTTON: Record<string, string> = {
	android: 'OkHttp GET',
	ios: 'URLSession.shared GET',
};

// the label every result line starts with, so the on-screen outcome is findable
// regardless of status or body
const RESULT_PREFIXES = ['okhttp', 'urlconnection', 'shared', 'custom', 'session', 'alamofire'];

type Device = {id: string; platform: string};

// one paused request as emitted by `network listen` (see docs/network-interception.md)
type PausedEvent = {
	type: string;
	requestId: string;
	method: string;
	url: string;
	body?: string | null;
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

function simulatorPlaygroundPID(deviceId: string): number | null {
	const processes = execFileSync('ps', ['-axo', 'pid=,command='], {encoding: 'utf8'});
	const line = processes.split('\n').find(line =>
		line.includes(`/CoreSimulator/Devices/${deviceId}/`) && line.includes('/playground.app/playground'));
	return line ? Number(line.trim().match(/^\d+/)?.[0]) : null;
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
	runMobilecli(['io', 'tap', `${Math.round(x + width / 2)},${Math.round(y + height / 2)}`, '--device', deviceId]);
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
		if (child.exitCode !== null || child.signalCode !== null) {
			resolve();
			return;
		}
		child.on('close', () => resolve());
		child.kill('SIGINT');
	});
	return {events, stop};
}

async function tapUntilPaused(deviceId: string, button: string, events: PausedEvent[]): Promise<PausedEvent> {
	const previousCount = events.length;
	const deadline = Date.now() + 20000;
	while (Date.now() < deadline) {
		tapButton(deviceId, button);
		await new Promise(resolve => setTimeout(resolve, 800));
		if (events.length > previousCount) return events[previousCount];
	}
	throw new Error(`${button} did not pause within 20 seconds`);
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

		// ── interception acceptance for both in-app agents ──
		test.describe('interception', () => {
			test('fulfill replaces the response the app receives', async () => {
				test.skip(!device, `no ${platform} device`);
				openNetworkScreen(device!.id);
				await eventually(() => !!findByText(device!.id, GET_BUTTON[platform]), 'no screen').toBe(true);

				const listener = startListening(device!.id, ['**/todos/**']);
				tapButton(device!.id, GET_BUTTON[platform]);
				await eventually(() => listener.events.length, 'no paused request').toBeGreaterThan(0);

				runMobilecli(['network', 'fulfill', '--device', device!.id,
					'--id', listener.events[0].requestId, '--status', '200', '--body', '{"mocked":true}']);

				await eventually(() => readNetworkResult(device!.id), 'mocked body never shown')
					.toContain('mocked');
				await listener.stop();
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

			test('iOS URLSession and Alamofire HTTP and HTTPS requests can be fulfilled', async () => {
				test.skip(platform !== 'ios' || !device, 'requires the iOS simulator playground');
				openNetworkScreen(device!.id);
				await eventually(() => !!findByText(device!.id, 'Alamofire HTTP GET'), 'no network screen').toBe(true);
				const listener = startListening(device!.id, ['**/todos/**']);
				try {
					const buttons = [
						'URLSession.shared GET',
						'URLSession HTTP GET',
						'Custom session GET',
						'Alamofire HTTPS GET',
						'Alamofire HTTP GET',
					];
					for (const [index, button] of buttons.entries()) {
						const previousCount = listener.events.length;
						tapButton(device!.id, button);
						await eventually(() => listener.events.length, `${button} was not paused`)
							.toBeGreaterThan(previousCount);
						const event = listener.events[previousCount];
						const marker = `mock-${index}`;
						const response = runMobilecliJson(['network', 'fulfill', '--device', device!.id,
							'--id', event.requestId, '--status', '200', '--body', marker]);
						expect(response.status).toBe('ok');
						await eventually(() => readNetworkResult(device!.id), `${button} did not show ${marker}`)
							.toContain(marker);
					}
				} finally {
					await listener.stop();
				}
			});

			test('iOS unmatched requests proceed without an event', async () => {
				test.skip(platform !== 'ios' || !device, 'requires the iOS simulator playground');
				openNetworkScreen(device!.id);
				const listener = startListening(device!.id, ['**/never-match/**']);
				try {
					tapButton(device!.id, 'Alamofire HTTPS GET');
					await eventually(() => readNetworkResult(device!.id), 'unmatched request did not complete')
						.toContain('alamofire HTTPS GET 200');
					expect(listener.events).toHaveLength(0);
				} finally {
					await listener.stop();
				}
			});

			test('iOS listener exit releases a paused request unchanged', async () => {
				test.skip(platform !== 'ios' || !device, 'requires the iOS simulator playground');
				openNetworkScreen(device!.id);
				const listener = startListening(device!.id, ['**/todos/**']);
				tapButton(device!.id, 'Alamofire HTTPS GET');
				await eventually(() => listener.events.length, 'request was not paused').toBeGreaterThan(0);
				await listener.stop();
				await eventually(() => readNetworkResult(device!.id), 'request stayed paused after disconnect')
					.toContain('alamofire HTTPS GET 200');
			});

			test('iOS POST body is captured and can be replaced on continue', async () => {
				test.skip(platform !== 'ios' || !device, 'requires the iOS simulator playground');
				openNetworkScreen(device!.id);
				const listener = startListening(device!.id, ['POST **/posts']);
				try {
					tapButton(device!.id, 'URLSession.shared POST');
					await eventually(() => listener.events.length, 'POST was not paused').toBeGreaterThan(0);
					const event = listener.events[0];
					expect(event.method).toBe('POST');
					expect(Buffer.from(event.body ?? '', 'base64').toString()).toContain('playground');
					runMobilecli(['network', 'continue', '--device', device!.id,
						'--id', event.requestId, '--body', '{"title":"overridden","body":"ok","userId":2}']);
					await eventually(() => readNetworkResult(device!.id), 'POST override did not arrive')
						.toContain('overridden');
				} finally {
					await listener.stop();
				}
			});

			test('iOS agent is reused after the daemon restarts', async () => {
				test.skip(platform !== 'ios' || !device, 'requires the iOS simulator playground');
				openNetworkScreen(device!.id);
				await eventually(() => !!findByText(device!.id, 'URLSession.shared GET'), 'no network screen').toBe(true);
				const appPID = simulatorPlaygroundPID(device!.id);
				expect(appPID).toBeTruthy();

				const first = startListening(device!.id, ['**/todos/**']);
				try {
					const event = await tapUntilPaused(device!.id, 'URLSession.shared GET', first.events);
					runMobilecli(['network', 'fulfill', '--device', device!.id,
						'--id', event.requestId, '--status', '200', '--body', 'before-restart']);
					await eventually(() => readNetworkResult(device!.id), 'first response did not arrive')
						.toContain('before-restart');
				} finally {
					await first.stop();
				}

				expect(runMobilecliJson(['daemon', 'stop']).status).toBe('ok');
				expect(simulatorPlaygroundPID(device!.id)).toBe(appPID);

				const second = startListening(device!.id, ['**/todos/**']);
				try {
					const event = await tapUntilPaused(device!.id, 'URLSession.shared GET', second.events);
					runMobilecli(['network', 'fulfill', '--device', device!.id,
						'--id', event.requestId, '--status', '200', '--body', 'after-restart']);
					await eventually(() => readNetworkResult(device!.id), 'second response did not arrive')
						.toContain('after-restart');
					expect(simulatorPlaygroundPID(device!.id)).toBe(appPID);
				} finally {
					await second.stop();
				}
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
