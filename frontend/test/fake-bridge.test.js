import test from 'node:test';
import assert from 'node:assert/strict';
import {readdirSync, readFileSync} from 'node:fs';
import {join} from 'node:path';
import {runInNewContext} from 'node:vm';

// The Go sources, not frontend/wailsjs: the bindings only exist after a
// Wails build, while internal/app is always in the repository, so the
// check runs on a fresh clone too.
const appDir = new URL('../../internal/app/', import.meta.url).pathname;
const bridgeFile = new URL('./harness/fake-bridge.js', import.meta.url).pathname;

// An exported method on the App pointer is what Wails binds for the
// page; unexported ones and the tests are not.
const BOUND_METHOD = /^func \(a \*App\) ([A-Z]\w*)\(/gm;

/**
 * The names of the App methods Wails binds, read from the Go files.
 * @returns {string[]} sorted
 */
function boundMethods() {
    const names = new Set();

    for (const name of readdirSync(appDir)) {
        if (!name.endsWith('.go') || name.endsWith('_test.go')) {
            continue;
        }
        const source = readFileSync(join(appDir, name), 'utf8');
        for (const match of source.matchAll(BOUND_METHOD)) {
            names.add(match[1]);
        }
    }

    return [...names].sort();
}

/**
 * The names of the methods the fake bridge provides, from running the
 * fake in a sandbox with a bare window.
 * @returns {string[]} sorted
 */
function fakedMethods() {
    const window = {addEventListener: () => {}};

    runInNewContext(readFileSync(bridgeFile, 'utf8'), {window});

    return Object.keys(window.go.app.App).sort();
}

test('the fake bridge has every method Go binds', () => {
    const faked = new Set(fakedMethods());
    const missing = boundMethods().filter((name) => !faked.has(name));

    assert.deepEqual(missing, []);
});

test('the fake bridge has nothing Go does not bind', () => {
    const bound = new Set(boundMethods());
    const extra = fakedMethods().filter((name) => !bound.has(name));

    assert.deepEqual(extra, []);
});

test('the Go sources were found', () => {
    // A moved package would make the first test pass on an empty list.
    assert.ok(boundMethods().length > 30);
});
