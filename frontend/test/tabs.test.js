import test from 'node:test';
import assert from 'node:assert/strict';
import {itemsOf, categoriesOf} from '../src/tabs.js';

// A state whose two tabs can be told apart.
const state = {
    tab: 'links',
    apps: [{id: 'mail'}],
    links: [{id: 'mdn'}],
    appCategories: [{id: 'tools'}],
    linkCategories: [{id: 'docs'}],
};

test('itemsOf and categoriesOf follow the state tab unless given one', () => {
    assert.equal(itemsOf(state), state.links);
    assert.equal(categoriesOf(state), state.linkCategories);
    assert.equal(itemsOf(state, 'apps'), state.apps);
    assert.equal(categoriesOf(state, 'apps'), state.appCategories);
});

test('anything that is not links reads as apps, as the old ternaries did', () => {
    assert.equal(itemsOf({...state, tab: 'nonsense'}), state.apps);
    assert.equal(categoriesOf({...state, tab: 'nonsense'}), state.appCategories);
});
