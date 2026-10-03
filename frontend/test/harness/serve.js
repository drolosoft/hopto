/**
 * Serves the built page (frontend/dist) with the fake bridge injected, so
 * the DOM logic can be driven with Playwright, or tried in any browser,
 * without Wails. Build first (`npm run build` or `wails build`).
 *
 * As a module it exports startHarness(); run directly it listens on 8765:
 *   node test/harness/serve.js
 */
import {createServer} from 'node:http';
import {readFile} from 'node:fs/promises';
import {extname, join, normalize} from 'node:path';
import {fileURLToPath} from 'node:url';

const dist = fileURLToPath(new URL('../../dist/', import.meta.url));
const bridge = fileURLToPath(new URL('./fake-bridge.js', import.meta.url));

// What the browser needs to know about each file it may ask for.
const TYPES = {
    '.html': 'text/html; charset=utf-8',
    '.js': 'text/javascript',
    '.css': 'text/css',
    '.woff2': 'font/woff2',
    '.png': 'image/png',
    '.svg': 'image/svg+xml',
};

/**
 * Answers one request: index.html with the bridge injected before the
 * page's module, anything else straight from dist.
 * @param {import('node:http').IncomingMessage} request
 * @param {import('node:http').ServerResponse} response
 */
async function handle(request, response) {
    const path = new URL(request.url, 'http://harness').pathname;

    try {
        if (path === '/' || path === '/index.html') {
            const html = await readFile(join(dist, 'index.html'), 'utf8');
            // The bridge has to exist before the page's module runs. The CSP
            // meta of the build allows 'self', and the bridge is served here.
            const injected = html.replace('<script type="module"', '<script src="/fake-bridge.js"></script><script type="module"');
            response.writeHead(200, {'Content-Type': TYPES['.html']});
            response.end(injected);
            return;
        }

        if (path === '/fake-bridge.js') {
            response.writeHead(200, {'Content-Type': TYPES['.js']});
            response.end(await readFile(bridge));
            return;
        }

        // Icons come from Go in the real app; the harness has none.
        if (path.startsWith('/user-icons/')) {
            response.writeHead(404);
            response.end();
            return;
        }

        const file = normalize(join(dist, path));
        if (!file.startsWith(dist)) {
            response.writeHead(403);
            response.end();
            return;
        }

        // Read before the head is written: a missing file (/favicon.ico,
        // say) has to reach the 404 below with no head sent, or writing
        // that 404 throws and takes the harness down.
        const body = await readFile(file);
        response.writeHead(200, {'Content-Type': TYPES[extname(file)] ?? 'application/octet-stream'});
        response.end(body);
    } catch {
        response.writeHead(404);
        response.end();
    }
}

/**
 * Starts the harness on a port (0 picks a free one) and returns its URL
 * and a way to stop it.
 * @param {number} port
 * @returns {Promise<{url: string, close: () => Promise<void>}>}
 */
export function startHarness(port = 0) {
    return new Promise((resolve) => {
        const server = createServer(handle);
        server.listen(port, '127.0.0.1', () => {
            const url = `http://127.0.0.1:${server.address().port}`;
            resolve({url, close: () => new Promise((done) => server.close(done))});
        });
    });
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
    startHarness(8765).then(({url}) => console.log(`Harness at ${url}`));
}
