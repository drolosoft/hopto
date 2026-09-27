import test from 'node:test';
import assert from 'node:assert/strict';
import {nextIndex, actionFor} from '../src/keys.js';

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

const press = (key, extra = {}) => ({key, metaKey: false, altKey: false, shiftKey: false, ctrlKey: false, isComposing: false, ...extra});
const idle = {query: '', editing: false, helpOpen: false, columns: 4};

test('arrows move, Enter opens, Tab switches', () => {
    assert.deepEqual(actionFor(press('ArrowRight'), idle), {type: 'move', delta: 1});
    assert.deepEqual(actionFor(press('ArrowDown'), idle), {type: 'move', delta: 4});
    assert.deepEqual(actionFor(press('ArrowUp'), idle), {type: 'move', delta: -4});
    assert.deepEqual(actionFor(press('Enter'), idle), {type: 'open'});
    assert.deepEqual(actionFor(press('Tab'), idle), {type: 'tab'});
});

test('Enter with modifiers opens elsewhere, reveals, or opens all', () => {
    assert.deepEqual(actionFor(press('Enter', {metaKey: true}), idle), {type: 'openAlt'});
    assert.deepEqual(actionFor(press('Enter', {altKey: true}), idle), {type: 'reveal'});
    assert.deepEqual(actionFor(press('Enter', {metaKey: true, shiftKey: true}), idle), {type: 'openAll'});
});

test('Review Focus 5: Caps Lock and composition', () => {
    assert.deepEqual(actionFor(press('F', {metaKey: true}), idle), {type: 'favorite'});
    assert.deepEqual(actionFor(press('f', {metaKey: true}), idle), {type: 'favorite'});
    assert.deepEqual(actionFor(press('C', {metaKey: true}), idle), {type: 'copy'});
    assert.equal(actionFor(press('Enter', {isComposing: true}), idle), null);
    assert.equal(actionFor(press('Enter', {key: 'Process'}), idle), null);
});

test('Escape clears the query first and hides on the second press', () => {
    assert.deepEqual(actionFor(press('Escape'), {...idle, query: 'abc'}), {type: 'clear'});
    assert.deepEqual(actionFor(press('Escape'), idle), {type: 'hide'});
    assert.deepEqual(actionFor(press('Escape'), {...idle, helpOpen: true}), {type: 'help'});
});

test('help opens on ? with an empty box or on ⌘/ any time', () => {
    assert.deepEqual(actionFor(press('?'), idle), {type: 'help'});
    assert.equal(actionFor(press('?'), {...idle, query: 'wh'}), null);
    assert.deepEqual(actionFor(press('/', {metaKey: true}), {...idle, query: 'wh'}), {type: 'help'});
});

test('⌘1-9 pick a chip, ⌘N ⌘E ⌘⌫ are the editor keys', () => {
    assert.deepEqual(actionFor(press('3', {metaKey: true}), idle), {type: 'category', index: 2});
    assert.deepEqual(actionFor(press('n', {metaKey: true}), idle), {type: 'new'});
    assert.deepEqual(actionFor(press('e', {metaKey: true}), idle), {type: 'edit'});
    assert.deepEqual(actionFor(press('Backspace', {metaKey: true}), idle), {type: 'delete'});
});

test('⌘1-9 is left to the search box once it holds a query', () => {
    assert.equal(actionFor(press('3', {metaKey: true}), {...idle, query: 'wh'}), null);
});

test('while the editor is open only Escape is taken', () => {
    const editing = {...idle, editing: true};
    assert.equal(actionFor(press('ArrowDown'), editing), null);
    assert.equal(actionFor(press('Enter'), editing), null);
    assert.deepEqual(actionFor(press('Escape'), editing), {type: 'closeEditor'});
});

test('plain typing is left to the search box', () => {
    assert.equal(actionFor(press('a'), idle), null);
    assert.equal(actionFor(press('a', {shiftKey: true}), idle), null);
});
