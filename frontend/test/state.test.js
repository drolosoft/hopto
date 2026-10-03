import test from 'node:test';
import assert from 'node:assert/strict';
import {initialState, counts, chipCounts, emptyMessage, totalsText, footerAction, removalOf, confirmQuestion} from '../src/state.js';
import {translator} from '../src/i18n.js';
import {decorate, FAVORITES} from '../src/filter.js';

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
    assert.equal(state.draft, null);
    assert.equal(state.dialogOpen, false);
    assert.equal(state.confirming, null);
    assert.equal(state.renaming, null);
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
    assert.equal(emptyMessage(stateWith({links: []}), t), 'No links yet · ⌘N adds one');
    assert.equal(emptyMessage(stateWith({tab: 'apps', apps: []}), t), 'No apps in /Applications');
});

test('totalsText counts both tabs with plurals', () => {
    assert.equal(totalsText(stateWith({}), t), '1 app · 2 links');
});

test('chipCounts adds up all, favourites and each category, hidden items left out', () => {
    const state = stateWith({
        linkCategories: [{id: 'y', name: 'Y'}, {id: 'z', name: 'Z'}],
        links: decorate([
            {id: 'l', key: 'links:l', kind: 'link', name: 'Link', category: 'y', keywords: []},
            {id: 'm', key: 'links:m', kind: 'link', name: 'More', category: 'y', keywords: []},
            {id: 'n', key: 'links:n', kind: 'link', name: 'None', category: 'z', keywords: [], hidden: true},
        ], {opens: {}, lastOpened: {}, favorites: ['links:l']}),
    });

    // "n" is hidden, so it counts nowhere: not in "All" and not in "Z".
    assert.deepEqual(chipCounts(state), {'': 2, [FAVORITES]: 1, y: 2, z: 0});
});

test('footerAction names the selected item or the pair', () => {
    assert.equal(footerAction({name: 'Alpha', kind: 'app'}, t), '↩ Open Alpha');
    assert.equal(footerAction({name: 'GitHub', kind: 'app', web: {id: 'x'}}, t), '↩ app · ⌘↩ web');
    assert.equal(footerAction({name: 'Add “x”', kind: 'add'}, t), '↩ Add');
    assert.equal(footerAction({name: 'Search Applications…', kind: 'pick'}, t), '↩ Choose');
    assert.equal(footerAction(undefined, t), '');
});

test('footerAction says what Enter does in the open editor, whatever row is under it', () => {
    assert.equal(footerAction({name: 'Alpha', kind: 'app'}, t, true), '↩ Save');
    assert.equal(footerAction(undefined, t, true), '↩ Save');
    assert.equal(footerAction(undefined, translator('es'), true), '↩ Guardar');
});

test('removalOf: links and hand-added apps are deleted, found apps hidden, hidden ones unhidden', () => {
    assert.equal(removalOf({kind: 'link', source: 'library'}), 'delete');
    assert.equal(removalOf({kind: 'app', source: 'library'}), 'delete');
    assert.equal(removalOf({kind: 'app', source: 'applications', hidden: false}), 'hide');
    assert.equal(removalOf({kind: 'app', source: 'edge', hidden: true}), 'unhide');
    assert.equal(removalOf({kind: 'add'}), '');
    assert.equal(removalOf(undefined), '');
});

test('confirmQuestion names what goes and how to answer', () => {
    assert.equal(confirmQuestion({action: 'delete', name: 'MDN'}, t), 'Delete MDN? ↩ yes · Esc no');
    assert.equal(confirmQuestion({action: 'hide', name: 'Mail'}, t), 'Hide Mail? ↩ yes · Esc no');
    assert.equal(confirmQuestion({action: 'deleteCategory', name: 'Docs'}, t), 'Delete the category Docs? ↩ yes · Esc no');
    assert.equal(confirmQuestion(null, t), '');
});

test('a search-only app counts while typing and not in the totals', () => {
    const apps = decorate([
        {id: 'a', key: 'apps:a', kind: 'app', name: 'Alpha', category: 'x', keywords: []},
        {id: 's', key: 'apps:s', kind: 'app', name: 'Alarm', category: 'applications', keywords: [], searchOnly: true},
    ], usage);
    const state = stateWith({apps});

    assert.deepEqual(counts(state, ''), {apps: 1, links: 2});
    assert.deepEqual(counts(state, 'al'), {apps: 2, links: 0});
    assert.equal(totalsText(state, t), '1 app · 2 links');
});
