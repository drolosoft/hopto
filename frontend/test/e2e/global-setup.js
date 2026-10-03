import {existsSync, readdirSync, statSync} from 'node:fs';
import {join} from 'node:path';
import {fileURLToPath} from 'node:url';
import {startHarness} from '../harness/serve.js';

const frontend = fileURLToPath(new URL('../../', import.meta.url));
const builtPage = join(frontend, 'dist', 'index.html');

/**
 * Every file under a folder, its subfolders included.
 * @param {string} dir
 * @returns {string[]} absolute paths
 */
function filesUnder(dir) {
    return readdirSync(dir, {withFileTypes: true, recursive: true})
        .filter((entry) => entry.isFile())
        .map((entry) => join(entry.parentPath, entry.name));
}

/**
 * The sources the build reads that changed after the build was made.
 * Vite writes dist/index.html on every build, so its time is the build's.
 * @returns {string[]} paths relative to frontend/
 */
function sourcesNewerThanBuild() {
    const built = statSync(builtPage).mtimeMs;
    const sources = [...filesUnder(join(frontend, 'src')), join(frontend, 'index.html')];

    return sources
        .filter((file) => statSync(file).mtimeMs > built)
        .map((file) => file.slice(frontend.length));
}

/**
 * Starts the harness once for the whole run and hands its URL to the
 * tests through the environment, which Playwright passes to the workers.
 * It refuses an old build: the tests would pass on the code as it was,
 * not as it is.
 * @returns {Promise<() => Promise<void>>} the teardown
 */
export default async function globalSetup() {
    if (!existsSync(builtPage)) {
        throw new Error('frontend/dist is empty: run `make build` (or `npm run build`) first');
    }

    const stale = sourcesNewerThanBuild();
    if (stale.length > 0) {
        throw new Error(`frontend/dist is older than ${stale.join(', ')}: run \`make build\` (or \`npm run build\`) again`);
    }

    const harness = await startHarness(0);
    process.env.HARNESS_URL = harness.url;

    return () => harness.close();
}
