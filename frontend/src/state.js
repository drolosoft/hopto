/**
 * The page state and the texts derived from it, without the DOM.
 */
import {rankItems, filterByCategory} from './filter.js';
import {TABS, otherTab} from './tabs.js';

/**
 * Everything the user can change while the launcher is on screen. `shown`
 * builds a fresh one, so the launcher never reappears with last time's
 * search or filter.
 * @param {string} tab
 * @returns {object}
 */
export function initialState(tab) {
    return {
        tab: tab === 'links' ? 'links' : 'apps',
        category: '',
        query: '',
        selected: 0,
        editing: false,
        helpOpen: false,
        apps: [],
        links: [],
        appCategories: [],
        linkCategories: [],
        usage: {opens: {}, lastOpened: {}, favorites: []},
        settings: {language: 'en', secondaryBrowser: ''},
        status: {path: '', error: '', line: 0, readOnly: false},
    };
}

/**
 * How many items of each tab match the query (all of them for an empty
 * query), for the tab labels.
 * @param {object} state
 * @param {string} query
 * @returns {{apps: number, links: number}}
 */
export function counts(state, query) {
    const count = (items) => (query ? rankItems(items, query).length : items.filter((item) => !item.hidden).length);
    return {apps: count(state.apps), links: count(state.links)};
}

/**
 * What to say when nothing is listed: the other tab has matches, nothing
 * matches, or the tab is empty altogether.
 * @param {object} state
 * @param {Function} t
 * @returns {string}
 */
export function emptyMessage(state, t) {
    const items = state.tab === 'links' ? state.links : state.apps;
    if (items.length === 0) {
        return t(TABS[state.tab].empty);
    }

    const other = otherTab(state.tab);
    const elsewhere = counts(state, state.query)[other];
    if (state.query && elsewhere > 0) {
        return t('empty.elsewhere', {count: elsewhere, tab: t(TABS[other].label)});
    }

    return t('empty.none');
}

/**
 * The footer's left side: the size of both catalogs, whatever the filter.
 * @param {object} state
 * @param {Function} t
 * @returns {string}
 */
export function totalsText(state, t) {
    return `${t.plural('apps', state.apps.filter((app) => !app.hidden).length)} · ${t.plural('links', state.links.length)}`;
}

/**
 * The footer's middle: what Enter does with the selected item.
 * @param {object|undefined} entry
 * @param {Function} t
 * @returns {string}
 */
export function footerAction(entry, t) {
    if (!entry) {
        return '';
    }

    return entry.web ? t('footer.pair') : t('footer.open', {name: entry.name});
}

/**
 * The items the chip filter leaves, for callers that need a flat list
 * (⌘⇧↩ opens all of them).
 * @param {object} state
 * @returns {object[]}
 */
export function chipItems(state) {
    const items = state.tab === 'links' ? state.links : state.apps;
    return filterByCategory(items.filter((item) => !item.hidden), state.category);
}
