/**
 * The page state and the texts derived from it, without the DOM.
 */
import {rankItems, filterByCategory, FAVORITES} from './filter.js';
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

        // The editor's draft (draft.js), a native dialog on screen, and the
        // row waiting for "delete? ↩ yes · Esc no"; shown wipes all three.
        draft: null,
        dialogOpen: false,
        confirming: null,
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
    // A hidden app never shows in the list, searching or not, so it must
    // not be counted either: unifiedSearch drops it the same way.
    const count = (items) => {
        const visible = items.filter((item) => !item.hidden);
        return query ? rankItems(visible, query).length : visible.length;
    };

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

    if (entry.kind === 'add') {
        return t('footer.add');
    }

    return entry.web ? t('footer.pair') : t('footer.open', {name: entry.name});
}

/**
 * How many items of the current tab each chip holds: every non-hidden
 * item for "All", the favourites among them for the star chip, and each
 * category's own non-hidden items. Ignores the search box on purpose:
 * chips are hidden while searching, so counting matches would count
 * something nobody is looking at.
 * @param {object} state
 * @returns {Object<string, number>}
 */
export function chipCounts(state) {
    const categories = state.tab === 'links' ? state.linkCategories : state.appCategories;
    const items = (state.tab === 'links' ? state.links : state.apps).filter((item) => !item.hidden);

    const totals = {'': items.length, [FAVORITES]: items.filter((item) => item.favorite).length};
    for (const category of categories) {
        totals[category.id] = filterByCategory(items, category.id).length;
    }

    return totals;
}

/**
 * The items of the active chip, for ⌘⇧↩. Empty with no chip selected: the
 * shortcut opens one category at a time, never "everything on the tab".
 * @param {object} state
 * @returns {object[]}
 */
export function chipItems(state) {
    if (!state.category) {
        return [];
    }

    const items = state.tab === 'links' ? state.links : state.apps;
    return filterByCategory(items.filter((item) => !item.hidden), state.category);
}
