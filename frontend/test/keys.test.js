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

test('while the editor is open: Esc closes, Enter saves, ⌘1-9 picks a category', () => {
    const editing = {...idle, editing: true, field: 'name'};
    assert.equal(actionFor(press('ArrowDown'), editing), null, 'arrows move the caret in a text field');
    assert.equal(actionFor(press('a'), editing), null);
    assert.equal(actionFor(press('Tab'), editing), null, 'Tab walks the fields');
    assert.deepEqual(actionFor(press('Enter'), editing), {type: 'save'});
    assert.deepEqual(actionFor(press('Escape'), editing), {type: 'closeEditor'});
    assert.deepEqual(actionFor(press('2', {metaKey: true}), editing), {type: 'pickCategory', index: 1});
    assert.equal(actionFor(press('Enter', {isComposing: true}), editing), null, 'Enter while composing is the IME\'s');
});

test('on the chip row the arrows move the category', () => {
    const onChips = {...idle, editing: true, field: 'category'};
    assert.deepEqual(actionFor(press('ArrowRight'), onChips), {type: 'moveCategory', delta: 1});
    assert.deepEqual(actionFor(press('ArrowDown'), onChips), {type: 'moveCategory', delta: 1});
    assert.deepEqual(actionFor(press('ArrowLeft'), onChips), {type: 'moveCategory', delta: -1});
    assert.deepEqual(actionFor(press('ArrowUp'), onChips), {type: 'moveCategory', delta: -1});
});

test('plain typing is left to the search box', () => {
    assert.equal(actionFor(press('a'), idle), null);
    assert.equal(actionFor(press('a', {shiftKey: true}), idle), null);
});

test('a pending confirmation takes Enter and Esc; anything else goes on', () => {
    const confirming = {...idle, confirming: true};
    assert.deepEqual(actionFor(press('Enter'), confirming), {type: 'confirm'});
    assert.deepEqual(actionFor(press('Escape'), confirming), {type: 'cancelConfirm'});
    assert.deepEqual(actionFor(press('Escape'), {...confirming, query: 'md'}), {type: 'cancelConfirm'}, 'Esc cancels before it clears');
    assert.deepEqual(actionFor(press('ArrowDown'), confirming), {type: 'move', delta: 4}, 'dispatch cancels, then moves');
    assert.deepEqual(actionFor(press('Enter', {metaKey: true}), confirming), {type: 'openAlt'});
});

test('⌘⇧E and ⌘⇧⌫ act on the chip; while renaming only Enter and Esc are taken', () => {
    assert.deepEqual(actionFor(press('E', {metaKey: true, shiftKey: true}), idle), {type: 'renameCategory'});
    assert.deepEqual(actionFor(press('Backspace', {metaKey: true, shiftKey: true}), idle), {type: 'deleteCategory'});

    const renaming = {...idle, renaming: true};
    assert.deepEqual(actionFor(press('Enter'), renaming), {type: 'saveRename'});
    assert.deepEqual(actionFor(press('Escape'), renaming), {type: 'cancelRename'});
    assert.equal(actionFor(press('a', {metaKey: true}), renaming), null, '⌘A selects the name');
    assert.equal(actionFor(press('ArrowLeft'), renaming), null);
});

test('in the editor ⌘E asks to edit the duplicate', () => {
    assert.deepEqual(actionFor(press('e', {metaKey: true}), {...idle, editing: true, field: 'url'}), {type: 'editDuplicate'});
});

test('in the editor ⌘O asks for the native dialog', () => {
    assert.deepEqual(actionFor(press('o', {metaKey: true}), {...idle, editing: true, field: 'name'}), {type: 'pickApp'});
});
