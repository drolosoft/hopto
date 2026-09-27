import {existsSync} from 'node:fs';
import {startHarness} from '../harness/serve.js';

/**
 * Starts the harness once for the whole run and hands its URL to the
 * tests through the environment, which Playwright passes to the workers.
 * @returns {Promise<() => Promise<void>>} the teardown
 */
export default async function globalSetup() {
    if (!existsSync(new URL('../../dist/index.html', import.meta.url))) {
        throw new Error('frontend/dist is empty: run `npm run build` (or `wails build`) first');
    }

    const harness = await startHarness(0);
    process.env.HARNESS_URL = harness.url;

    return () => harness.close();
}
