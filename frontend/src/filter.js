/**
 * Search, ranking and grouping, kept free of the DOM and of Wails so it
 * can be tested with plain Node (`npm test`).
 */

// The id of the virtual "favourites" chip; it filters on the item's
// favourite flag instead of on its category. Go reserves it (and its
// Spanish spelling) so no user category can take it.
export const FAVORITES = 'favorites';

// The points a query word earns depending on where it matches. Name
// matches count whole, the other texts half: a word at the start of a
// name is what the user is most likely typing.
const NAME_PREFIX = 3;
const WORD_PREFIX = 2;
const NAME_SUBSTRING = 1;
const TEXT_WEIGHT = 0.5;

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
 * The words of a query, normalised, empty ones dropped.
 * @param {string} query
 * @returns {string[]}
 */
function words(query) {
    return normalize(query).split(/\s+/).filter(Boolean);
}

/**
 * Points of one query word against one text: prefix of the text, prefix
 * of a word inside it, or substring; 0 when absent.
 * @param {string} text normalised
 * @param {string} word normalised
 * @returns {number}
 */
function scoreText(text, word) {
    if (!text || !text.includes(word)) {
        return 0;
    }

    if (text.startsWith(word)) {
        return NAME_PREFIX;
    }

    if (text.split(/[\s\-_.\/]+/).some((piece) => piece.startsWith(word))) {
        return WORD_PREFIX;
    }

    return NAME_SUBSTRING;
}

/**
 * Points of an item for a query: the best field of each word, summed;
 * 0 as soon as one word matches nowhere, so every word has to appear.
 * @param {{name: string, description?: string, host?: string, keywords?: string[]}} item
 * @param {string} query
 * @returns {number}
 */
export function score(item, query) {
    const pieces = words(query);
    if (pieces.length === 0) {
        return 0;
    }

    const name = normalize(item.name);
    const texts = [item.host, ...(item.keywords ?? []), item.description].map(normalize);

    let total = 0;
    for (const word of pieces) {
        const inName = scoreText(name, word);
        const inTexts = Math.max(0, ...texts.map((text) => scoreText(text, word))) * TEXT_WEIGHT;
        const best = Math.max(inName, inTexts);

        if (best === 0) {
            return 0;
        }

        total += best;
    }

    return total;
}

/**
 * Attaches what the usage file knows to each item: how many times it was
 * opened, when it was last opened (ms since epoch, 0 never) and whether it
 * is a favourite. Items are copied, the catalog is left alone.
 * @param {object[]} items
 * @param {{opens?: Object<string, number>, lastOpened?: Object<string, string>, favorites?: string[]}} usage
 * @returns {object[]}
 */
export function decorate(items, usage) {
    const favorites = new Set(usage?.favorites ?? []);
    const opens = usage?.opens ?? {};
    const lastOpened = usage?.lastOpened ?? {};

    return items.map((item) => ({
        ...item,
        opens: opens[item.key] ?? 0,
        lastOpened: lastOpened[item.key] ? Date.parse(lastOpened[item.key]) || 0 : 0,
        favorite: favorites.has(item.key),
    }));
}

/**
 * Compares two decorated items for the tie-break: favourites first, then
 * most recently opened, then most opened, then name.
 * @param {object} left
 * @param {object} right
 * @returns {number}
 */
function byPreference(left, right) {
    return (Number(right.favorite) - Number(left.favorite))
        || (right.lastOpened - left.lastOpened)
        || (right.opens - left.opens)
        || left.name.localeCompare(right.name);
}

/**
 * The items that match the query, best first.
 * @param {object[]} items decorated
 * @param {string} query
 * @returns {object[]}
 */
export function rankItems(items, query) {
    return items
        .map((item) => ({item, points: score(item, query)}))
        .filter((entry) => entry.points > 0)
        .sort((left, right) => (right.points - left.points) || byPreference(left.item, right.item))
        .map((entry) => entry.item);
}

/**
 * The items of a chip: everything, the favourites, or one category.
 * @param {object[]} items
 * @param {string} category
 * @returns {object[]}
 */
export function filterByCategory(items, category) {
    if (!category) {
        return items;
    }

    if (category === FAVORITES) {
        return items.filter((item) => item.favorite);
    }

    return items.filter((item) => item.category === category);
}

/**
 * The empty-query layout of a tab: favourites, the most recent, and the
 * rest grouped by category in chip order. An item shows once; hidden ones
 * never show; empty sections are dropped.
 * @param {object[]} items decorated
 * @param {{id: string, name: string}[]} categories chip order
 * @param {number} recentLimit
 * @returns {{id: string, items: object[]}[]}
 */
export function sections(items, categories, recentLimit) {
    const visible = items.filter((item) => !item.hidden);
    const placed = new Set();
    const result = [];

    const favorites = visible.filter((item) => item.favorite);
    if (favorites.length > 0) {
        favorites.forEach((item) => placed.add(item.key));
        result.push({id: 'favorites', items: favorites});
    }

    const recent = visible
        .filter((item) => item.lastOpened > 0 && !placed.has(item.key))
        .sort((left, right) => right.lastOpened - left.lastOpened)
        .slice(0, recentLimit);
    if (recent.length > 0) {
        recent.forEach((item) => placed.add(item.key));
        result.push({id: 'recent', items: recent});
    }

    for (const category of categories) {
        const rest = visible.filter((item) => item.category === category.id && !placed.has(item.key));
        if (rest.length > 0) {
            result.push({id: category.id, items: rest});
        }
    }

    return result;
}

/**
 * Folds an Edge app and a link on the same host into one row: the app
 * opens on Enter, the link rides along as `web` for ⌘↩.
 * @param {object[]} items
 * @returns {object[]}
 */
export function pairByHost(items) {
    const linksByHost = new Map();
    items.filter((item) => item.kind === 'link' && item.host).forEach((link) => {
        if (!linksByHost.has(link.host)) {
            linksByHost.set(link.host, link);
        }
    });

    const paired = new Set();
    const rows = [];

    for (const item of items) {
        if (item.kind === 'app' && item.source === 'edge' && item.host && linksByHost.has(item.host)) {
            const web = linksByHost.get(item.host);
            paired.add(web.key);
            rows.push({...item, web});
            continue;
        }

        rows.push(item);
    }

    return rows.filter((row) => !paired.has(row.key));
}

/**
 * One list for both tabs while the user types: apps and links ranked
 * together, hidden apps left out, Edge apps paired with their link.
 * @param {object[]} apps decorated
 * @param {object[]} links decorated
 * @param {string} query
 * @returns {object[]}
 */
export function unifiedSearch(apps, links, query) {
    const ranked = rankItems([...apps.filter((app) => !app.hidden), ...links], query);
    return pairByHost(ranked);
}
