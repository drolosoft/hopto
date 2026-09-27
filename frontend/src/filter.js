/**
 * Search, ranking and grouping, kept free of the DOM and of Wails so it
 * can be tested with plain Node (`npm test`).
 */

// The id of the virtual "favourites" chip; it filters on the item's
// favourite flag instead of on its category. Go reserves it (and its
// Spanish spelling) so no user category can take it.
export const FAVORITES = 'favorites';

// The id of the virtual chip of the hidden apps. It is the only place a
// hidden app is listed, and Go reserves the id like the favourites one.
export const HIDDEN = 'hidden';

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
 * @param {{id?: string, name: string, description?: string, host?: string, keywords?: string[]}} item
 * @param {string} query
 * @returns {number}
 */
export function score(item, query) {
    const pieces = words(query);
    if (pieces.length === 0) {
        return 0;
    }

    const name = normalize(item.name);
    // The id is searched too, at the same half weight as host, keywords
    // and description: the old launcher found an item by "go-doc" even
    // when neither its name nor its description mentioned "doc".
    const texts = [item.id, item.host, ...(item.keywords ?? []), item.description].map(normalize);

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
 * The items of a chip: everything, the favourites, or one category. A
 * search-only app (from /System/Applications) is never under a chip:
 * the chips show what the user keeps, those only come up by typing.
 * @param {object[]} items
 * @param {string} category
 * @returns {object[]}
 */
export function filterByCategory(items, category) {
    const listed = items.filter((item) => !item.searchOnly);

    if (!category) {
        return listed;
    }

    if (category === FAVORITES) {
        return listed.filter((item) => item.favorite);
    }

    return listed.filter((item) => item.category === category);
}

/**
 * The items a chip lists: under the hidden chip only the hidden apps,
 * under any other one the items of that chip that are not hidden.
 * @param {object[]} items
 * @param {string} category
 * @returns {object[]}
 */
export function visibleUnder(items, category) {
    if (category === HIDDEN) {
        return items.filter((item) => item.hidden);
    }

    return filterByCategory(items.filter((item) => !item.hidden), category);
}

/**
 * Sorts a category group with the most opened items first. `sort` is
 * stable, so a comparator on `opens` alone keeps the library order for
 * items opened the same number of times (0 for two never-opened items,
 * for instance). The array is copied: the caller's filter result is not
 * touched.
 * @param {object[]} items one category, library order
 * @returns {object[]}
 */
function byOpens(items) {
    return [...items].sort((left, right) => right.opens - left.opens);
}

/**
 * The empty-query layout of a tab: favourites, the most recent, and the
 * rest grouped by category, most opened first inside each group. An item
 * shows once; hidden and search-only ones never show; empty sections are
 * dropped.
 * @param {object[]} items decorated
 * @param {{id: string, name: string}[]} categories chip order
 * @param {number} recentLimit
 * @returns {{id: string, items: object[]}[]}
 */
export function sections(items, categories, recentLimit) {
    const visible = items.filter((item) => !item.hidden && !item.searchOnly);
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
            result.push({id: category.id, items: byOpens(rest)});
        }
    }

    return result;
}

/**
 * Reduces a URL to what identifies the page, the way Go's
 * library.NormalizeURL does: lowercase scheme and host (the URL parser
 * already lowercases them and drops a default port), no "www.", no
 * fragment, no trailing slash, the query kept. "" when it does not parse.
 * @param {string} raw
 * @returns {string}
 */
export function pageKey(raw) {
    let parsed;
    try {
        parsed = new URL(String(raw ?? '').trim());
    } catch {
        return '';
    }

    const scheme = parsed.protocol.replace(/:$/, '');
    const host = parsed.hostname.replace(/^www\./, '');
    const port = parsed.port ? `:${parsed.port}` : '';
    const path = parsed.pathname.replace(/\/+$/, '');

    return `${scheme}://${host}${port}${path}${parsed.search}`;
}

/**
 * Folds an Edge app and the link to the very same page into one row: the
 * app opens on Enter, the link rides along as `web` for ⌘↩. Sharing a
 * host is not enough (a web app on one path of a server is not the
 * server's front page), and a link that matches the query better than
 * the app keeps its own row: folding it would bury the best result
 * under a weaker one.
 * @param {object[]} items ranked
 * @param {string} query
 * @returns {object[]}
 */
export function pairSamePage(items, query) {
    const linksByPage = new Map();
    for (const item of items) {
        const key = item.kind === 'link' ? pageKey(item.url) : '';
        if (key && !linksByPage.has(key)) {
            linksByPage.set(key, item);
        }
    }

    const paired = new Set();
    const rows = [];

    for (const item of items) {
        const isWebApp = item.kind === 'app' && item.source === 'edge';
        const web = isWebApp ? linksByPage.get(pageKey(item.url)) : undefined;

        if (web && !paired.has(web.key) && score(web, query) <= score(item, query)) {
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
 * together, hidden apps left out, Edge apps paired with the link to the
 * same page.
 * @param {object[]} apps decorated
 * @param {object[]} links decorated
 * @param {string} query
 * @returns {object[]}
 */
export function unifiedSearch(apps, links, query) {
    const ranked = rankItems([...apps.filter((app) => !app.hidden), ...links], query);
    return pairSamePage(ranked, query);
}
