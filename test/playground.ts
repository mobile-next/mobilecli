import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';

// the playground app is the one app both specs can install and uninstall freely:
// it is ours, it is not part of any system image, and its bundle id is identical
// on both platforms. bump this one constant to move every spec to a new build.
export const PLAYGROUND_VERSION = 'v1.0.6';
export const PLAYGROUND_PACKAGE = 'com.mobilenext.playground';

// values the release is built with, asserted so a regression that drops a field
// from `apps list` or `apps install` fails here instead of shipping
export const PLAYGROUND_APP_NAME = 'Playground';
export const PLAYGROUND_APP_VERSION = '1.0';
export const PLAYGROUND_APP_VERSION_CODE = '1';

// android installs an apk; the ios simulator takes a zipped .app bundle
type PlaygroundPlatform = 'android' | 'ios';

const ARTIFACT_EXTENSION: Record<PlaygroundPlatform, string> = {
	android: 'apk',
	ios: 'zip',
};

function artifactUrl(platform: PlaygroundPlatform): string {
	const version = PLAYGROUND_VERSION.replace(/^v/, '');
	const file = `Playground-${version}.${ARTIFACT_EXTENSION[platform]}`;
	return `https://github.com/mobile-next/playground/releases/download/${PLAYGROUND_VERSION}/${file}`;
}

// cached under the version, so a rerun on the same build skips the download and a
// version bump can never install a stale artifact left behind by an earlier run
function artifactPath(platform: PlaygroundPlatform): string {
	const version = PLAYGROUND_VERSION.replace(/^v/, '');
	return path.join(os.tmpdir(), `mobilecli-playground-${version}.${ARTIFACT_EXTENSION[platform]}`);
}

export async function downloadPlayground(platform: PlaygroundPlatform): Promise<string> {
	const localPath = artifactPath(platform);
	if (fs.existsSync(localPath) && fs.statSync(localPath).size > 0) {
		return localPath;
	}

	const url = artifactUrl(platform);
	const response = await fetch(url);
	if (!response.ok) {
		throw new Error(`failed to download ${url}: ${response.status} ${response.statusText}`);
	}

	// written to a temp name first, so an interrupted run cannot leave a truncated
	// file that the next run would happily treat as cached
	const partialPath = `${localPath}.partial`;
	fs.writeFileSync(partialPath, Buffer.from(await response.arrayBuffer()));
	fs.renameSync(partialPath, localPath);
	return localPath;
}
