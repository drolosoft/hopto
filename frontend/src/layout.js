/**
 * What the list shows for a given state: the rows, the sections they
 * fall under, and whether it is the unified search. Kept free of the DOM
 * and of Wails, like filter.js, so `npm test` covers it with plain Node.
 */
import {visibleUnder, sections, unifiedSearch} from './filter.js';
import {addOffer} from './draft.js';
import {itemsOf, categoriesOf} from './tabs.js';

// How many "Recent" items the empty-query layout shows.
const RECENT_LIMIT = 5;

/**
 * The "＋ Add" row the search offers when nothing matches, or at the end
 * when the text is an address. It is an entry like any other, so the
 * keyboard, the selection and the footer reach it with no special case.
 * @param {{text: string, url: boolean}} offer
 * @param {Function} t the translator
 * @returns {object}
 */
export function addEntry(offer, t) {
    return {
        key: 'add:link',
        kind: 'add',
        id: '',
        name: t('add.row', {text: offer.text}),
        description: offer.url ? t('add.asURL') : t('add.asName'),
        glyph: '＋',
        text: offer.text,
    };
}

/**
 * The "Search Applications…" row of the apps tab: Enter opens the app
 * editor with the native dialog.
 * @param {Function} t the translator
 * @returns {object}
 */
export function pickEntry(t) {
    return {
        key: 'pick:app',
        kind: 'pick',
        id: '',
        name: t('pick.row'),
        description: t('pick.hint'),
        glyph: '⌕',
    };
}

/**
 * What the list shows for the state: the unified search while typing, a
 * flat list under a chip, or the sections of the tab.
 * @param {object} current the page state
 * @param {Function} t the translator, for the section titles and the add
 *     and pick rows
 * @returns {{entries: object[], groups: {title: string, start: number, count: number}[], unified: boolean}}
 */
export function layoutOf(current, t) {
    if (current.query) {
        const entries = unifiedSearch(current.apps, current.links, current.query);
        const offer = addOffer(current.query, entries.length);
        if (offer) {
            entries.push(addEntry(offer, t));

            // On the apps tab the search also offers the native dialog:
            // the add row itself always adds a link.
            if (current.tab === 'apps') {
                entries.push(pickEntry(t));
            }
        }

        return {entries, groups: [], unified: true};
    }

    const items = itemsOf(current);
    const categories = categoriesOf(current);

    if (current.category) {
        return {entries: visibleUnder(items, current.category), groups: [], unified: false};
    }

    const named = categories.map((category) => ({id: category.id, name: category.virtual ? t(`category.${category.id}`) : category.name}));
    const grouped = sections(items, named, RECENT_LIMIT);

    const entries = [];
    const groups = [];
    for (const group of grouped) {
        const title = group.id === 'favorites' ? t('section.favorites')
            : group.id === 'recent' ? t('section.recent')
            : named.find((category) => category.id === group.id)?.name ?? group.id;
        groups.push({title, start: entries.length, count: group.items.length});
        entries.push(...group.items);
    }

    return {entries, groups, unified: false};
}
