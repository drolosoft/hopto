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
