import test from 'node:test';
import assert from 'node:assert/strict';
import {nextIndex} from '../src/keys.js';

test('the selection wraps around in both directions', () => {
    assert.equal(nextIndex(0, 1, 3), 1);
    assert.equal(nextIndex(2, 1, 3), 0);
    assert.equal(nextIndex(0, -1, 3), 2);
});

test('a jump larger than the list never leaves a negative index', () => {
    // ↑ on the apps grid moves by one row of columns; with two results and
    // five columns that used to compute (0 - 5 + 2) % 2 = -1.
    const index = nextIndex(0, -5, 2);
    assert.ok(index >= 0 && index < 2, `index ${index} is out of range`);
    assert.equal(nextIndex(1, 7, 3), 2);
});

test('an empty list keeps the selection at zero', () => {
    assert.equal(nextIndex(4, 1, 0), 0);
});
