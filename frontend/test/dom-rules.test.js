import test from 'node:test';
import assert from 'node:assert/strict';
import {readdirSync, readFileSync} from 'node:fs';
import {join} from 'node:path';

// The page builds every node with createElement: no HTML strings ever
// reach the DOM, so nothing a user typed can become markup. The only
// tolerated form is clearing (`= ''`), and even that is unused today.
test('no innerHTML, outerHTML or insertAdjacentHTML in frontend/src', () => {
    const dir = new URL('../src/', import.meta.url).pathname;
    const offenders = [];

    for (const name of readdirSync(dir).filter((file) => file.endsWith('.js'))) {
        const source = readFileSync(join(dir, name), 'utf8');
        for (const [index, line] of source.split('\n').entries()) {
            if (/insertAdjacentHTML/.test(line) || (/\.(innerHTML|outerHTML)\b/.test(line) && !/=\s*''/.test(line))) {
                offenders.push(`${name}:${index + 1}: ${line.trim()}`);
            }
        }
    }

    assert.deepEqual(offenders, []);
});

const sourceDir = new URL('../src/', import.meta.url).pathname;
const indexHtml = readFileSync(new URL('../index.html', import.meta.url), 'utf8');

/**
 * Every line of the page's modules that matches a pattern, as
 * "file:line: text", so a failure says where.
 * @param {RegExp} pattern
 * @returns {string[]}
 */
function sourceLinesMatching(pattern) {
    const found = [];

    for (const name of readdirSync(sourceDir).filter((file) => file.endsWith('.js'))) {
        const source = readFileSync(join(sourceDir, name), 'utf8');
        for (const [index, line] of source.split('\n').entries()) {
            if (pattern.test(line)) {
                found.push(`${name}:${index + 1}: ${line.trim()}`);
            }
        }
    }

    return found;
}

// The style card's inline-code rules. The CSP of the build blocks inline
// scripts and styles, so a break here would be a page that works under
// Vite's dev server and fails in the app.
test('index.html has no inline script, style element, style attribute or event handler', () => {
    const inlineScripts = [...indexHtml.matchAll(/<script\b(?![^>]*\bsrc=)[^>]*>/g)].map((match) => match[0]);

    assert.deepEqual(inlineScripts, []);
    assert.doesNotMatch(indexHtml, /<style\b/i);
    assert.doesNotMatch(indexHtml, /\sstyle\s*=/);
    assert.doesNotMatch(indexHtml, /\son[a-z]+\s*=/i);
});

test('the modules set no inline style and no handler attribute', () => {
    assert.deepEqual(sourceLinesMatching(/\.style\.|\.style\s*=|setAttribute\(\s*['"](style|on[a-z]+)['"]/), []);
});

test('colours are tokens: no hex colour outside the :root blocks', () => {
    const css = readFileSync(join(sourceDir, 'style.css'), 'utf8')
        .replace(/\/\*[\s\S]*?\*\//g, '')
        .replace(/:root\s*\{[^}]*\}/g, '');
    // A hex colour can only sit in a declaration's value, after a colon and
    // before the semicolon that ends it; a value may run over several
    // lines, so the match is made on the whole text. An id such as #fade
    // in a selector is not a colour.
    const strays = [...css.matchAll(/:[^;{}]*#[0-9a-fA-F]{3,8}\b/g)].map((match) => match[0].trim());

    assert.deepEqual(strays, []);
    // In the modules a colour can also sit inside a longer string, such as
    // '1px solid #fff'.
    assert.deepEqual(sourceLinesMatching(/(['"`])[^'"`\n]*#[0-9a-fA-F]{3,8}\b[^'"`\n]*\1/), []);
});
