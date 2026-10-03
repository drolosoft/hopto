import test from 'node:test';
import assert from 'node:assert/strict';
import {primaryKey, glyphsFor} from '../src/modifiers.js';

test('the primary chord key is Command on macOS and Control on Windows', () => {
    const meta = {metaKey: true, ctrlKey: false};
    const ctrl = {metaKey: false, ctrlKey: true};

    assert.equal(primaryKey(meta, 'darwin'), true);
    assert.equal(primaryKey(ctrl, 'darwin'), false);
    assert.equal(primaryKey(meta, 'windows'), false);
    assert.equal(primaryKey(ctrl, 'windows'), true);
    assert.equal(primaryKey(meta, undefined), true, 'no platform reads as macOS');
});

test('glyphs are symbols on macOS and words with a plus on Windows', () => {
    assert.equal(glyphsFor('darwin').cmd, '⌘');
    assert.equal(glyphsFor('windows').cmd, 'Ctrl+');
    assert.equal(glyphsFor('windows').enter, 'Enter');
    assert.equal(glyphsFor('darwin').tabKey, '⇥');
    assert.equal(glyphsFor('windows').tabKey, 'Tab');
    assert.equal(glyphsFor('linux').cmd, '⌘', 'unknown platforms read as macOS');
});

test('AltGr on Windows is not a chord: it arrives as Ctrl+Alt', () => {
    const altGr = {metaKey: false, ctrlKey: true, altKey: true};

    assert.equal(primaryKey(altGr, 'windows'), false, 'Ctrl+Alt+e types €, it is not Ctrl+E');
    assert.equal(primaryKey({...altGr, metaKey: true}, 'darwin'), true, 'macOS still reads Command');
});
