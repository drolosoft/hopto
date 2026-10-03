import test from 'node:test';
import assert from 'node:assert/strict';
import {addEntry, pickEntry, layoutOf} from '../src/layout.js';
import {initialState} from '../src/state.js';
import {translator} from '../src/i18n.js';
import {decorate, FAVORITES, HIDDEN} from '../src/filter.js';

const t = translator('en');

// Alpha is a favourite and Beta was opened once, so the empty-query
// layout of the apps tab has a section of each kind.
const usage = {opens: {'apps:b': 1}, lastOpened: {'apps:b': '2026-10-01T10:00:00Z'}, favorites: ['apps:a']};

/**
 * A state with two app categories, one of them virtual, a hidden app and
 * two links, with the given fields on top.
 * @param {object} overrides
 * @returns {object}
 */
function stateWith(overrides) {
    const state = initialState('apps');
    state.appCategories = [{id: 'work', name: 'Work'}, {id: 'applications', name: '', virtual: true}];
    state.linkCategories = [{id: 'docs', name: 'Docs'}];
    state.apps = decorate([
        {id: 'a', key: 'apps:a', kind: 'app', source: 'library', name: 'Alpha', category: 'work', keywords: []},
        {id: 'b', key: 'apps:b', kind: 'app', source: 'applications', name: 'Beta', category: 'applications', keywords: []},
        {id: 'c', key: 'apps:c', kind: 'app', source: 'applications', name: 'Gamma', category: 'applications', keywords: []},
        {id: 'h', key: 'apps:h', kind: 'app', source: 'applications', name: 'Hidden One', category: 'applications', keywords: [], hidden: true},
    ], usage);
    state.links = decorate([
        {id: 'mdn', key: 'links:mdn', kind: 'link', name: 'MDN', host: 'developer.mozilla.org', category: 'docs', keywords: []},
        {id: 'go', key: 'links:go', kind: 'link', name: 'Go', host: 'go.dev', category: 'docs', keywords: []},
    ], usage);

    return {...state, ...overrides};
}

test('addEntry is a row the keyboard can reach, worded for a name or an address', () => {
    const byName = addEntry({text: 'notes', url: false}, t);
    assert.equal(byName.key, 'add:link');
    assert.equal(byName.kind, 'add');
    assert.equal(byName.id, '');
    assert.equal(byName.name, 'Add “notes”');
    assert.equal(byName.description, 'New link with this name');
    assert.equal(byName.text, 'notes');

    const byURL = addEntry({text: 'example.com', url: true}, t);
    assert.equal(byURL.description, 'New link to this address');
});

test('pickEntry is the apps tab row that opens the native dialog', () => {
    const entry = pickEntry(t);
    assert.equal(entry.key, 'pick:app');
    assert.equal(entry.kind, 'pick');
    assert.equal(entry.id, '');
    assert.equal(entry.name, 'Search Applications…');
    assert.equal(entry.description, 'Pick a .app to add it by hand');
});

test('layoutOf searches both tabs while typing, without sections or hidden apps', () => {
    const layout = layoutOf(stateWith({query: 'go'}), t);
    assert.equal(layout.unified, true);
    assert.deepEqual(layout.groups, []);
    assert.deepEqual(layout.entries.map((entry) => entry.key), ['links:go']);

    const hidden = layoutOf(stateWith({query: 'hidden'}), t);
    assert.ok(hidden.entries.every((entry) => entry.key !== 'apps:h'));
});

test('layoutOf offers the add row and, on the apps tab only, the pick row when nothing matches', () => {
    const apps = layoutOf(stateWith({query: 'zzz'}), t);
    assert.deepEqual(apps.entries.map((entry) => entry.kind), ['add', 'pick']);

    const links = layoutOf(stateWith({tab: 'links', query: 'zzz'}), t);
    assert.deepEqual(links.entries.map((entry) => entry.kind), ['add']);
});

test('layoutOf puts the add row after the matches when the text is an address', () => {
    const layout = layoutOf(stateWith({tab: 'links', query: 'go.dev'}), t);
    assert.deepEqual(layout.entries.map((entry) => entry.key), ['links:go', 'add:link']);
});

test('layoutOf lists a chip flat, and the hidden chip shows only the hidden apps', () => {
    const work = layoutOf(stateWith({category: 'work'}), t);
    assert.equal(work.unified, false);
    assert.deepEqual(work.groups, []);
    assert.deepEqual(work.entries.map((entry) => entry.key), ['apps:a']);

    const hidden = layoutOf(stateWith({category: HIDDEN}), t);
    assert.deepEqual(hidden.entries.map((entry) => entry.key), ['apps:h']);

    const favorites = layoutOf(stateWith({category: FAVORITES}), t);
    assert.deepEqual(favorites.entries.map((entry) => entry.key), ['apps:a']);
});

test('layoutOf groups the empty query in sections, each with its start and count', () => {
    const layout = layoutOf(stateWith({}), t);
    assert.equal(layout.unified, false);
    assert.deepEqual(layout.entries.map((entry) => entry.key), ['apps:a', 'apps:b', 'apps:c']);

    // A virtual category takes its translated name; a user one keeps its
    // own. Work is empty once Alpha sits under the favourites, so it goes.
    assert.deepEqual(layout.groups, [
        {title: 'Favourites', start: 0, count: 1},
        {title: 'Recent', start: 1, count: 1},
        {title: 'Applications', start: 2, count: 1},
    ]);
});

test('layoutOf reads the links tab with its own categories', () => {
    const layout = layoutOf(stateWith({tab: 'links'}), t);
    assert.deepEqual(layout.entries.map((entry) => entry.key), ['links:mdn', 'links:go']);
    assert.deepEqual(layout.groups, [{title: 'Docs', start: 0, count: 2}]);
});

test('layoutOf titles the sections in the language of the translator', () => {
    const layout = layoutOf(stateWith({}), translator('es'));
    assert.deepEqual(layout.groups.map((group) => group.title), ['Favoritos', 'Recientes', 'Aplicaciones']);
});
