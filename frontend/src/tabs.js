/**
 * What differs between the two tabs, in one table, so nothing else in the
 * page branches on the tab name. `items` and `categories` name the fields
 * of the page state that hold each tab's entries and chips.
 */
export const TABS = {
    apps: {label: 'tab.apps', layout: 'cards', empty: 'empty.apps', plural: 'apps', items: 'apps', categories: 'appCategories'},
    links: {label: 'tab.links', layout: 'rows', empty: 'empty.links', plural: 'links', items: 'links', categories: 'linkCategories'},
};

/**
 * The tab that is not this one.
 * @param {string} tab
 * @returns {string}
 */
export function otherTab(tab) {
    return tab === 'links' ? 'apps' : 'links';
}

/**
 * The row of TABS for a tab. Anything that is not the links tab reads as
 * apps, as the page has always treated it.
 * @param {string} tab
 * @returns {object}
 */
function tabRow(tab) {
    return tab === 'links' ? TABS.links : TABS.apps;
}

/**
 * The items of a tab, the state's own array.
 * @param {object} state
 * @param {string} [tab] the state's tab when left out
 * @returns {object[]}
 */
export function itemsOf(state, tab = state.tab) {
    return state[tabRow(tab).items];
}

/**
 * The categories of a tab, the state's own array, so a new chip pushed
 * on it is in the state too.
 * @param {object} state
 * @param {string} [tab] the state's tab when left out
 * @returns {object[]}
 */
export function categoriesOf(state, tab = state.tab) {
    return state[tabRow(tab).categories];
}
