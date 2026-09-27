/**
 * What differs between the two tabs, in one table, so nothing else in the
 * page branches on the tab name.
 */
export const TABS = {
    apps: {label: 'tab.apps', layout: 'cards', empty: 'empty.apps', plural: 'apps'},
    links: {label: 'tab.links', layout: 'rows', empty: 'empty.links', plural: 'links'},
};

/**
 * The tab that is not this one.
 * @param {string} tab
 * @returns {string}
 */
export function otherTab(tab) {
    return tab === 'links' ? 'apps' : 'links';
}
