import test from 'node:test';
import assert from 'node:assert/strict';
import {prettyHotkey} from '../src/hotkeys.js';
import {translator} from '../src/i18n.js';

test('prettyHotkey writes a spec the way macOS menus do', () => {
    const t = translator('en');

    assert.equal(prettyHotkey('cmd+shift+space', t), '⇧⌘Space', 'modifiers in the order ⌃⌥⇧⌘');
    assert.equal(prettyHotkey('cmd+option+space', t), '⌥⌘Space', 'modifiers in the order ⌃⌥⇧⌘');
    assert.equal(prettyHotkey('Ctrl + Alt + K', t), '⌃⌥K');
    assert.equal(prettyHotkey('command+f5', t), '⌘F5');
    assert.equal(prettyHotkey('cmd+shift+space', translator('es')), '⇧⌘Espacio');
});

test('prettyHotkey leaves what it does not understand as it came', () => {
    const t = translator('en');

    assert.equal(prettyHotkey('space', t), 'space');
    assert.equal(prettyHotkey('hyper+k', t), 'hyper+k');
    assert.equal(prettyHotkey(undefined, t), '');
});

test('prettyHotkey writes Windows specs with words and plus signs', () => {
    const t = translator('en', 'windows');

    assert.equal(prettyHotkey('ctrl+shift+space', t, 'windows'), 'Ctrl+Shift+Space');
    assert.equal(prettyHotkey('ctrl+alt+space', t, 'windows'), 'Ctrl+Alt+Space');
    assert.equal(prettyHotkey('cmd+shift+space', t, 'windows'), 'Win+Shift+Space');
    assert.equal(prettyHotkey('ctrl+shift+space', translator('es', 'windows'), 'windows'), 'Ctrl+Shift+Espacio');
});
