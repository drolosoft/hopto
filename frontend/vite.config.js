import {defineConfig} from 'vite';

// The Content-Security-Policy only makes sense in the built page: the dev
// server injects its own inline client for hot reload, which the policy
// would block. Nothing in the page needs inline code or remote hosts; the
// icons come from the app's own asset server and the editor previews are
// data URLs.
const CSP = [
    "default-src 'none'",
    "script-src 'self'",
    "style-src 'self'",
    "img-src 'self' data:",
    "font-src 'self'",
    "connect-src 'self'",
    "base-uri 'none'",
    "form-action 'none'",
    "object-src 'none'",
    "frame-src 'none'",
].join('; ');

/**
 * Adds the CSP meta tag to index.html at build time only.
 * @returns {import('vite').Plugin}
 */
function cspOnBuild() {
    return {
        name: 'hopto-csp',
        apply: 'build',
        transformIndexHtml(html) {
            return html.replace('<meta charset="UTF-8"/>', `<meta charset="UTF-8"/>\n    <meta http-equiv="Content-Security-Policy" content="${CSP}"/>`);
        },
    };
}

export default defineConfig({
    plugins: [cspOnBuild()],
    build: {
        // The font must stay a file: a data: URL would need font-src data:.
        assetsInlineLimit: 0,
    },
});
