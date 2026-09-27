/**
 * The editor's draft, without the DOM: how the search text becomes a
 * draft, how typing and the page inspection change it, and what is sent
 * to Go on save. Every function returns a new draft; none mutates.
 */
import {nextIndex} from './keys.js';

// A typed web scheme, in any case.
const WEB_SCHEME = /^https?:\/\//i;

// Any scheme at all, so withScheme never stacks https:// on top of one
// ("ftp://" is left for the validator to refuse with its own message).
const ANY_SCHEME = /^[a-z][a-z0-9+.-]*:\/\//i;

// The spec's rule for "looks like a domain": something with a dot and no
// spaces, with no empty piece around a dot. "notes.txt" passes too; the
// price of that false positive is one page read that finds nothing.
const DOTTED = /^[^\s.]+(\.[^\s.]+)+$/;

/**
 * Whether the text is an address the editor should put in the URL field.
 * @param {string} text
 * @returns {boolean}
 */
export function looksLikeURL(text) {
    const trimmed = String(text ?? '').trim();
    if (trimmed === '' || /\s/.test(trimmed)) {
        return false;
    }

    return WEB_SCHEME.test(trimmed) || DOTTED.test(trimmed);
}

/**
 * Puts https:// in front of a bare address; any typed scheme stays.
 * @param {string} text
 * @returns {string}
 */
export function withScheme(text) {
    const trimmed = String(text ?? '').trim();
    if (trimmed === '' || ANY_SCHEME.test(trimmed)) {
        return trimmed;
    }

    return `https://${trimmed}`;
}

/**
 * The host of an address, "" when it does not parse.
 * @param {string} url
 * @returns {string}
 */
export function hostOf(url) {
    try {
        return new URL(url).hostname;
    } catch {
        return '';
    }
}

/**
 * Whether the search should offer the "＋ Add" row: always when nothing
 * matches, and at the end of the results when the text is an address
 * (pasting a URL you half have should still be one Enter away).
 * @param {string} query
 * @param {number} resultCount
 * @returns {{text: string, url: boolean}|null}
 */
export function addOffer(query, resultCount) {
    const text = String(query ?? '').trim();
    if (text === '') {
        return null;
    }

    const url = looksLikeURL(text);
    if (resultCount > 0 && !url) {
        return null;
    }

    return {text, url};
}

/**
 * A fresh draft from what was typed. An address fills the URL, anything
 * else the name; the active chip is preselected when it is a real
 * category, else the first one; with no categories at all the
 * new-category field opens straight away.
 * @param {{tab?: string, text?: string, category?: string, categories?: {id: string}[], returnQuery?: string}} options
 * @returns {object}
 */
export function newDraft({tab = 'links', text = '', category = '', categories = [], returnQuery} = {}) {
    const typed = String(text ?? '').trim();
    const isURL = looksLikeURL(typed);
    const known = categories.some((chip) => chip.id === category);

    return {
        mode: 'add',
        tab,
        id: '',
        url: isURL ? withScheme(typed) : '',
        name: isURL ? '' : typed,
        description: '',
        keywords: [],
        path: '',
        bundleId: '',
        category: known ? category : (categories[0]?.id ?? ''),
        newCategory: categories.length === 0 ? '' : null,
        iconDataUrl: '',
        insecure: false,
        duplicate: null,
        sameHost: [],
        twin: null,
        problems: {},
        touched: isURL || typed === '' ? [] : ['name'],
        inspecting: false,
        saving: false,
        returnQuery: returnQuery ?? String(text ?? ''),
    };
}

/**
 * A draft for the item under the selection, for ⌘E. The three text
 * fields count as typed, so a new inspection (after the URL changes)
 * fills only the icon and the notes, never over what was saved. The
 * list's icon URL is the preview until an inspection brings another.
 * @param {object} entry an ItemView from the list
 * @param {{id: string}[]} categories the real chips of its tab
 * @param {string} [returnQuery] the search text to give back on Esc
 * @returns {object}
 */
export function editDraft(entry, categories, returnQuery = '') {
    const tab = entry.kind === 'link' ? 'links' : 'apps';
    const base = newDraft({tab, category: entry.category, categories, returnQuery});

    return {
        ...base,
        mode: 'edit',
        id: entry.id,
        url: entry.url ?? '',
        name: entry.name ?? '',
        description: entry.description ?? '',
        keywords: [...(entry.keywords ?? [])],
        path: entry.path ?? '',
        bundleId: entry.bundleId ?? '',
        iconDataUrl: entry.iconUrl ?? '',
        touched: ['url', 'name', 'description'],
    };
}

/**
 * The draft after the user typed in a field. The field's problem goes
 * (the user is fixing it); a new URL drops what the old one said about
 * duplicates and neighbours, and stops a pending "reading the page".
 * @param {object} draft
 * @param {string} field
 * @param {string} value
 * @returns {object}
 */
export function withInput(draft, field, value) {
    const problems = {...draft.problems};
    delete problems[field === 'newCategory' ? 'category' : field];

    const touched = draft.touched.includes(field) ? draft.touched : [...draft.touched, field];
    const next = {...draft, [field]: value, problems, touched};

    if (field === 'url') {
        next.duplicate = null;
        next.sameHost = [];
        next.inspecting = false;
        next.insecure = /^http:\/\//i.test(String(value).trim());
    }

    return next;
}

/**
 * The draft with what InspectURL said about its page. An answer for a
 * URL the field no longer holds is dropped whole; a field the user typed
 * in is never overwritten; in edit mode the link itself is not its own
 * duplicate or neighbour.
 * @param {object} draft
 * @param {{url: string, name: string, description: string, iconDataUrl: string, insecure: boolean, duplicate: object|null, sameHost: object[]}} info
 * @returns {object}
 */
export function withInspection(draft, info) {
    if (!info || info.url !== withScheme(draft.url)) {
        return draft;
    }

    const isSelf = (ref) => draft.mode === 'edit' && ref?.id === draft.id;

    return {
        ...draft,
        name: draft.touched.includes('name') ? draft.name : (info.name || draft.name),
        description: draft.touched.includes('description') ? draft.description : (info.description || draft.description),
        iconDataUrl: info.iconDataUrl || draft.iconDataUrl,
        insecure: Boolean(info.insecure),
        duplicate: info.duplicate && !isSelf(info.duplicate) ? info.duplicate : null,
        sameHost: (info.sameHost ?? []).filter((ref) => !isSelf(ref)),
        inspecting: false,
    };
}

/**
 * Picks the chip at index; the one past the last category is "New
 * category…", which opens the name field. Anything further is ignored.
 * @param {object} draft
 * @param {{id: string}[]} categories
 * @param {number} index
 * @returns {object}
 */
export function pickCategory(draft, categories, index) {
    const problems = {...draft.problems};
    delete problems.category;

    if (index >= 0 && index < categories.length) {
        return {...draft, category: categories[index].id, newCategory: null, problems};
    }

    if (index === categories.length) {
        return {...draft, newCategory: draft.newCategory ?? '', problems};
    }

    return draft;
}

/**
 * Moves the category choice one chip left or right, wrapping around and
 * counting "New category…" as the last chip.
 * @param {object} draft
 * @param {{id: string}[]} categories
 * @param {number} delta
 * @returns {object}
 */
export function moveCategory(draft, categories, delta) {
    const current = draft.newCategory !== null
        ? categories.length
        : Math.max(0, categories.findIndex((chip) => chip.id === draft.category));

    return pickCategory(draft, categories, nextIndex(current, delta, categories.length + 1));
}

/**
 * The problems the page can see without asking Go: no address yet, or a
 * new category without a name.
 * @param {object} draft
 * @returns {Object<string, string>}
 */
export function localProblems(draft) {
    const problems = {};

    if (draft.tab === 'links' && !looksLikeURL(draft.url)) {
        problems.url = 'url.invalid';
    }

    // An app needs its .app; a path the dialog already refused (outside
    // the app folders) stays refused until another one is picked.
    if (draft.tab === 'apps' && !draft.path) {
        problems.path = 'app.target';
    } else if (draft.tab === 'apps' && draft.problems.path) {
        problems.path = draft.problems.path;
    }

    if (draft.newCategory !== null && draft.newCategory.trim() === '') {
        problems.category = 'category.name';
    }

    return problems;
}

/**
 * Whether Enter may save: nothing missing locally, no exact duplicate
 * (that one blocks), and no save already on its way.
 * @param {object} draft
 * @returns {boolean}
 */
export function canSave(draft) {
    return !draft.saving && !draft.duplicate && Object.keys(localProblems(draft)).length === 0;
}

/**
 * What AddLink and UpdateLink receive. A link saved without a name is
 * named after its host, which is what the row would show anyway.
 * @param {object} draft
 * @returns {{url: string, name: string, description: string, category: string, keywords: string[], icon: string}}
 */
export function linkInput(draft) {
    const url = withScheme(draft.url);

    return {
        url,
        name: draft.name.trim() || hostOf(url),
        description: draft.description.trim(),
        category: draft.category,
        keywords: [...draft.keywords],
        icon: '',
    };
}

/**
 * Reads Go's answer to a save: an id means done; otherwise the problems
 * and the duplicate go back on the draft for the user to fix.
 * @param {object} draft
 * @param {{id: string, problems: Object<string, string>, duplicate: object|null}} result
 * @returns {{draft: object, savedId: string}}
 */
export function withSaveResult(draft, result) {
    if (result?.id) {
        return {draft: {...draft, saving: false}, savedId: result.id};
    }

    return {
        draft: {
            ...draft,
            saving: false,
            problems: result?.problems ?? {},
            duplicate: result?.duplicate ?? draft.duplicate,
        },
        savedId: '',
    };
}

/**
 * The sentence for a problem key from Go ("name.required"); a key the
 * dictionary does not know gets a generic line rather than the raw key.
 * @param {Function} t
 * @param {string} key
 * @returns {string}
 */
export function problemText(t, key) {
    if (!key) {
        return '';
    }

    const text = t(`problem.${key}`);

    return text === `problem.${key}` ? t('problem.other') : text;
}

// The id prefixes Go gives the apps it discovers; the library refuses
// them for anything the user adds.
const DISCOVERED_ID = /^(app|edge)-/;

/**
 * Whether an id belongs to an app found on disk rather than to the
 * user's library.
 * @param {string|undefined} id
 * @returns {boolean}
 */
export function isDiscoveredID(id) {
    return DISCOVERED_ID.test(id ?? '');
}

/**
 * The draft after the native dialog answered. A cancel (no path) leaves
 * it as it was. A typed name or description is kept. A duplicate that is
 * a hand-added app blocks, like a link's; one only found on disk is the
 * twin: adding it by hand is how it gets a category, so it only warns.
 * @param {object} draft
 * @param {{path: string, bundleId: string, name: string, description: string, iconDataUrl: string, problem: string, duplicate: object|null}} pick
 * @returns {object}
 */
export function withPick(draft, pick) {
    if (!pick?.path) {
        return draft;
    }

    const problems = {...draft.problems};
    delete problems.path;
    if (pick.problem) {
        problems.path = pick.problem;
    }

    const found = pick.duplicate && isDiscoveredID(pick.duplicate.id) ? pick.duplicate : null;
    const isSelf = draft.mode === 'edit' && pick.duplicate?.id === draft.id;
    const blocking = pick.duplicate && !found && !isSelf ? pick.duplicate : null;

    return {
        ...draft,
        path: pick.path,
        bundleId: pick.bundleId ?? '',
        name: draft.touched.includes('name') ? draft.name : (pick.name || draft.name),
        description: draft.touched.includes('description') ? draft.description : (pick.description || draft.description),
        iconDataUrl: pick.iconDataUrl || draft.iconDataUrl,
        duplicate: blocking,
        twin: found,
        problems,
    };
}

/**
 * A draft that adds a discovered app by hand, for ⌘E on it: the same
 * bundle, the name it shows, and itself as the twin the note explains.
 * @param {object} entry an ItemView of a discovered app
 * @param {{id: string}[]} categories the real chips of the apps tab
 * @param {string} [returnQuery]
 * @returns {object}
 */
export function adoptDraft(entry, categories, returnQuery = '') {
    const base = newDraft({tab: 'apps', category: entry.category, categories, returnQuery});

    return {
        ...base,
        path: entry.path ?? '',
        bundleId: entry.bundleId ?? '',
        name: entry.name ?? '',
        description: entry.description ?? '',
        iconDataUrl: entry.iconUrl ?? '',
        twin: {tab: 'apps', id: entry.id, name: entry.name},
        touched: ['name'],
    };
}

/**
 * What AddApp and UpdateApp receive. An app saved without a name takes
 * the bundle's folder name, as the Finder shows it.
 * @param {object} draft
 * @returns {{path: string, bundleId: string, name: string, description: string, category: string}}
 */
export function appInput(draft) {
    const folder = draft.path.split('/').pop() ?? '';

    return {
        path: draft.path,
        bundleId: draft.bundleId,
        name: draft.name.trim() || folder.replace(/\.app$/, ''),
        description: draft.description.trim(),
        category: draft.category,
    };
}
