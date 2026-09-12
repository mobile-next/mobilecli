import {expect} from '@playwright/test';
import type {Point, UIDumpResponse, UIElement} from './types';

// the playground webview loads this page, and the page reads its own query string:
// `?source=webview` shows a login form, `?done=<name>` shows a greeting instead.
// that gives every navigation command a visible, assertable effect.
export const WEBVIEW_SAMPLE_URL = 'https://mobilewright.dev/samples/webview/?source=webview';
export const WEBVIEW_DONE_NAME = 'mobilecli';
export const WEBVIEW_DONE_URL = `https://mobilewright.dev/samples/webview/?done=${WEBVIEW_DONE_NAME}`;
export const WEBVIEW_SAMPLE_TITLE = 'Sample Login';
export const WEBVIEW_DONE_GREETING = `Hello there, ${WEBVIEW_DONE_NAME}. You are still in the webview!`;

// the button that opens the webview screen from the playground main menu. android
// labels it through a text node, ios through the button's accessibility label.
export const WEB_VIEW_BUTTON_LABEL = 'Web View';

// both platforms return a nested tree, so flatten it before searching
export function flattenElements(elements: UIElement[]): UIElement[] {
	return elements.flatMap(element => [element, ...flattenElements(element.children ?? [])]);
}

export function centerOf(element: UIElement): Point {
	return {
		x: element.rect.x + Math.floor(element.rect.width / 2),
		y: element.rect.y + Math.floor(element.rect.height / 2),
	};
}

// android puts the caption in a child text node of the row, ios puts it on the
// button itself, so match on any element that carries the label and take the one
// with a tappable area
export function findWebViewButton(uiDump: UIDumpResponse): UIElement {
	const matches = flattenElements(uiDump.data.elements).filter(element =>
		[element.text, element.label, element.name].includes(WEB_VIEW_BUTTON_LABEL));

	const button = matches.find(element => element.rect.width > 0 && element.rect.height > 0);
	if (!button) {
		throw new Error(`no tappable "${WEB_VIEW_BUTTON_LABEL}" button in the playground menu`);
	}

	return button;
}

// `webview list` reports the same fields on both platforms, but ios leaves
// bundleId and processName empty, so those are asserted as present, not filled
export function expectWebViewShape(webView: unknown): asserts webView is WebViewInfo {
	const view = webView as Record<string, unknown>;

	for (const field of ['id', 'url', 'title', 'bundleId', 'processName']) {
		expect(typeof view?.[field], `webview.${field}: ${JSON.stringify(webView)}`).toBe('string');
	}
	expect((view.id as string).length, 'webview.id is empty').toBeGreaterThan(0);
	expect(typeof view.isVisible).toBe('boolean');

	for (const field of ['x', 'y', 'width', 'height']) {
		expect(typeof (view.bounds as Record<string, unknown>)?.[field], `webview.bounds.${field}`).toBe('number');
	}
}

export interface WebViewInfo {
	id: string;
	url: string;
	title: string;
	bundleId: string;
	processName: string;
	bounds: {
		x: number;
		y: number;
		width: number;
		height: number;
	};
	isVisible: boolean;
}

export interface WebViewQueryResult {
	tag: string;
	id: string | null;
	class: string | null;
	href: string | null;
	text: string | null;
	value: string | null;
}

// `goto`, `back` and `forward` return as soon as the navigation is requested, and
// `wait` can observe the load event of the page still on screen. polling the url
// is what actually proves the navigation landed, without a guessed sleep.
export async function expectWebViewUrlToBecome(readUrl: () => string, expected: string): Promise<void> {
	await expect.poll(readUrl, {
		timeout: 15000,
		message: `webview never navigated to ${expected}`,
	}).toBe(expected);
}

// no webview ever has this id, so every command that takes one fails on it
export const WEBVIEW_MISSING_ID = 'no-such-webview';

// each subcommand's arguments after the webview id, so one test can walk them all
export const WEBVIEW_COMMANDS_TAKING_AN_ID: ReadonlyArray<readonly string[]> = [
	['url'],
	['title'],
	['content'],
	['reload'],
	['back'],
	['forward'],
	['wait'],
	['goto', WEBVIEW_SAMPLE_URL],
	['query', 'body'],
	['eval', 'document.title'],
];
