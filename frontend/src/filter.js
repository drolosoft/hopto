/**
 * Search and category filtering, kept free of the DOM and of Wails so it
 * can be tested with plain Node (`npm test`).
 */

/**
 * Lowercases a text and strips accents, so "cronometro" finds "Cronómetro"
 * and "Musica" finds "Música".
 * @param {string} text
 * @returns {string}
 */
export function normalize(text) {
    return String(text ?? '')
        .normalize('NFD')
        .replace(/[̀-ͯ]/g, '')
        .toLowerCase();
}

/**
 * The text an item is searched on: name, description, host (links only)
 * and id, so "gitea" finds "Gitea" and "repos" finds repos.example.com.
 * @param {{name: string, description?: string, host?: string, id: string}} item
 * @returns {string}
 */
function haystack(item) {
    return normalize([item.name, item.description, item.host, item.id].filter(Boolean).join(' '));
}

/**
 * True when every word of the query appears somewhere in the item. An empty
 * query matches everything.
 * @param {object} item
 * @param {string} query
 * @returns {boolean}
 */
export function matches(item, query) {
    const words = normalize(query).split(/\s+/).filter(Boolean);
    if (words.length === 0) {
        return true;
    }

    const text = haystack(item);
    return words.every((word) => text.includes(word));
}

/**
 * True when the item belongs to the chip: every item for the empty chip,
 * the favourites for the favourites chip, otherwise its category.
 * @param {object} item
 * @param {string} category
 * @returns {boolean}
 */
function inCategory(item, category) {
    if (!category) {
        return true;
    }

    if (category === FAVORITES) {
        return Boolean(item.favorite);
    }

    return item.category === category;
}

/**
 * The items of one chip that match the query, so favourites can be searched
 * like any category.
 * @param {object[]} items
 * @param {string} category
 * @param {string} query
 * @returns {object[]}
 */
export function filterItems(items, category, query) {
    return items.filter((item) => inCategory(item, category) && matches(item, query));
}

// The id of the virtual "favourites" chip; it filters on the item's
// favourite flag instead of on its category.
export const FAVORITES = 'favoritos';

/**
 * Attaches what the usage file knows to each item of a tab: its usage key,
 * how many times it was opened and whether it is a favourite. Items are
 * copied, the catalog is left alone.
 * @param {object[]} items
 * @param {string} tab
 * @param {{opens: Object<string, number>, favorites: string[]}} usage
 * @returns {object[]}
 */
export function decorate(items, tab, usage) {
    const favorites = new Set(usage?.favorites ?? []);
    const opens = usage?.opens ?? {};

    return items.map((item) => {
        const key = `${tab}:${item.id}`;
        return {...item, key, opens: opens[key] ?? 0, favorite: favorites.has(key)};
    });
}

/**
 * Sorts by number of openings, most used first, keeping the catalog order
 * among items opened the same number of times (sort is stable).
 * @param {object[]} items
 * @returns {object[]}
 */
export function sortByUse(items) {
    return [...items].sort((left, right) => (right.opens ?? 0) - (left.opens ?? 0));
}
