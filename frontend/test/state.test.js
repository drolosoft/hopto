import test from 'node:test';
import assert from 'node:assert/strict';
import {initialState, counts, emptyMessage, totalsText, footerAction} from '../src/state.js';
import {translator} from '../src/i18n.js';
import {decorate} from '../src/filter.js';

const t = translator('en');
const usage = {opens: {}, lastOpened: {}, favorites: []};

function stateWith(overrides) {
    const state = initialState('links');
    state.apps = decorate([{id: 'a', key: 'apps:a', kind: 'app', name: 'Alpha', category: 'x', keywords: []}], usage);
    state.links = decorate([
        {id: 'l', key: 'links:l', kind: 'link', name: 'Link', host: 'l.test', category: 'y', keywords: []},
        {id: 'm', key: 'links:m', kind: 'link', name: 'More', host: 'm.test', category: 'y', keywords: []},
    ], usage);
    return {...state, ...overrides};
}

test('initialState starts clean on the given tab', () => {
    const state = initialState('links');
    assert.equal(state.tab, 'links');
    assert.equal(state.query, '');
    assert.equal(state.selected, 0);
    assert.equal(state.category, '');
    assert.equal(state.editing, false);
});

test('counts says how many of each tab match the query', () => {
    assert.deepEqual(counts(stateWith({}), ''), {apps: 1, links: 2});
    assert.deepEqual(counts(stateWith({}), 'alp'), {apps: 1, links: 0});
});

test('counts drops a hidden app while searching, like the list does', () => {
    const apps = decorate([
        {id: 'a', key: 'apps:a', kind: 'app', name: 'Alpha', category: 'x', keywords: []},
        {id: 'h', key: 'apps:h', kind: 'app', name: 'Alpine', category: 'x', keywords: [], hidden: true},
    ], usage);

    assert.deepEqual(counts(stateWith({apps}), 'al'), {apps: 1, links: 0});
});

test('emptyMessage points at the other tab, or says there is nothing', () => {
    assert.equal(emptyMessage(stateWith({query: 'alp'}), t), 'Nothing here · 1 in Apps (⇥)');
    assert.equal(emptyMessage(stateWith({query: 'zzz'}), t), 'Nothing matches');
    assert.equal(emptyMessage(stateWith({links: []}), t), 'No links yet');
    assert.equal(emptyMessage(stateWith({tab: 'apps', apps: []}), t), 'No apps in /Applications');
});

test('totalsText counts both tabs with plurals', () => {
    assert.equal(totalsText(stateWith({}), t), '1 app · 2 links');
});

test('footerAction names the selected item or the pair', () => {
    assert.equal(footerAction({name: 'Alpha', kind: 'app'}, t), '↩ Open Alpha');
    assert.equal(footerAction({name: 'GitHub', kind: 'app', web: {id: 'x'}}, t), '↩ app · ⌘↩ web');
    assert.equal(footerAction(undefined, t), '');
});
