import * as path from 'path';
import {mkdirSync} from 'fs';

// a mobilecli built with `go build -cover` writes counter files to GOCOVERDIR on
// exit, and emits nothing at all when it is unset. every spec must therefore pass
// this env to each child process it spawns, or that binary's work goes unmeasured.
// `make test-e2e` reads the same directory back with `go tool covdata`.
export function coverageEnv(): NodeJS.ProcessEnv {
	const dir = path.join(__dirname, 'coverage');
	mkdirSync(dir, {recursive: true});
	return {...process.env, GOCOVERDIR: dir};
}
