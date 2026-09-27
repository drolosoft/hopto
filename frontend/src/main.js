/**
 * The glue: state, calls to Go, and the wiring between the modules.
 */
import './style.css';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {Items, Categories, Usage, Settings, LibraryStatus, About, Launch, OpenLink, OpenLinkWith, CopyTarget, RevealInFinder, Hide, TabChanged, Debug, ToggleFavorite} from '../wailsjs/go/main/App';
import {decorate, filterByCategory, sections, unifiedSearch} from './filter.js';
import {nextIndex} from './keys.js';
import {initialState, chipItems} from './state.js';
import {TABS, otherTab} from './tabs.js';
import {resolveLanguage, translator} from './i18n.js';
import {renderAll, renderHelp, showToast, animateAppearance, columns, searchBox, verticalNeighbour, setAbout} from './render.js';
import {installKeyboard} from './keyboard.js';

// How many "Recent" items the empty-query layout shows.
const RECENT_LIMIT = 5;

// ⌘⇧↩ opens every item of the active chip at once; past this many, that
// is a wall of windows rather than a shortcut, so it refuses instead.
const MAX_CHIP_OPEN = 10;

// How long to wait after a blur before hiding: the first appearance after
// launch fires a blur while the window is being activated, and if the
// focus is back by then the launcher stays.
const BLUR_GRACE_MS = 150;

// The state and the translator; both are rebuilt on every appearance.
let state = initialState('apps');
let t = translator('en');

const search = searchBox();

/**
 * What the list shows for the state: the unified search while typing, a
 * flat list under a chip, or the sections of the tab.
 * @param {object} current
 * @returns {{entries: object[], groups: {title: string, start: number, count: number}[], unified: boolean}}
 */
export function layoutOf(current) {
    if (current.query) {
        return {entries: unifiedSearch(current.apps, current.links, current.query), groups: [], unified: true};
    }

    const items = current.tab === 'links' ? current.links : current.apps;
    const categories = current.tab === 'links' ? current.linkCategories : current.appCategories;

    if (current.category) {
        return {entries: filterByCategory(items.filter((item) => !item.hidden), current.category), groups: [], unified: false};
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

/**
 * Repaints from the state, keeping the selection inside the list.
 */
function render() {
    const layout = layoutOf(state);

    if (state.selected >= layout.entries.length) {
        state.selected = Math.max(0, layout.entries.length - 1);
    }

    renderAll(state, layout, t, {onChip: selectCategory, onOpen: openAt, onToggleFavorite: toggleFavorite});
}

/**
 * The entry under the selection, if any.
 * @returns {object|undefined}
 */
function selectedEntry() {
    return layoutOf(state).entries[state.selected];
}

/**
 * Switches tab, resets the filter of the previous one and tells Go, so the
 * shortcuts know which tab is on screen.
 * @param {string} tab
 */
function selectTab(tab) {
    if (tab === state.tab) {
        return;
    }

    state.tab = tab;
    state.category = '';
    state.selected = 0;
    TabChanged(tab);
    render();
    search.focus();
}

/**
 * Applies a category chip and starts the selection from the top.
 * @param {string} category
 */
function selectCategory(category) {
    state.category = category;
    state.selected = 0;
    render();
    search.focus();
}

/**
 * Opens an entry: an app through Launch, a link through OpenLink. Go hides
 * the launcher on success; a failure is shown as a toast.
 * @param {object|undefined} entry
 * @param {'default'|'alt'} how
 */
async function openEntry(entry, how = 'default') {
    if (!entry) {
        return;
    }

    try {
        if (how === 'alt' && entry.web) {
            await OpenLink(entry.web.id);
        } else if (how === 'alt' && entry.kind === 'link') {
            if (!state.settings.secondaryBrowser) {
                showToast(t('toast.noBrowser'));
                return;
            }

            await OpenLinkWith(entry.id);
        } else if (entry.kind === 'link') {
            await OpenLink(entry.id);
        } else {
            await Launch(entry.id);
        }
    } catch (error) {
        showToast(t('toast.openFailed', {error: String(error)}));
    }
}

/**
 * Opens the entry at an index (a mouse click).
 * @param {number} index
 */
function openAt(index) {
    state.selected = index;
    openEntry(selectedEntry());
}

/**
 * Marks or unmarks an item as favourite through Go, then mirrors the answer
 * in the local usage so the star and the favourites chip update at once.
 * @param {{key: string}} entry
 */
async function toggleFavorite(entry) {
    try {
        const on = await ToggleFavorite(entry.key);
        const others = state.usage.favorites.filter((key) => key !== entry.key);
        state.usage.favorites = on ? [...others, entry.key] : others;
        decorateAll();
    } catch (error) {
        Debug(`favorite ${entry.key}: ${error}`);
    }

    render();
    search.focus();
}

/**
 * Re-attaches the usage to both catalogs after it changed.
 */
function decorateAll() {
    state.apps = decorate(state.apps, state.usage);
    state.links = decorate(state.links, state.usage);
}

/**
 * Asks Go for everything and repaints. Called on every appearance, so an
 * app installed in the meantime shows up without restarting the launcher.
 */
async function refresh() {
    try {
        const [apps, appCategories, links, linkCategories, usage, settings, status] = await Promise.all([
            Items('apps'),
            Categories('apps'),
            Items('links'),
            Categories('links'),
            Usage(),
            Settings(),
            LibraryStatus(),
        ]);

        // Belt and braces: the page never trusts the shape of what it gets.
        state.usage = {opens: usage?.opens ?? {}, lastOpened: usage?.lastOpened ?? {}, favorites: usage?.favorites ?? []};
        state.settings = settings ?? state.settings;
        state.status = status ?? state.status;
        state.appCategories = appCategories ?? [];
        state.linkCategories = linkCategories ?? [];
        state.apps = decorate(apps ?? [], state.usage);
        state.links = decorate(links ?? [], state.usage);

        t = translator(resolveLanguage(state.settings.language, navigator.language));
    } catch (error) {
        // A failed call must not strand the caller's .then(): the search
        // box still has to get focus, and the page still has to paint
        // whatever it already knew before this refresh.
        Debug(`refresh: ${error}`);
    }

    render();
}

/**
 * Re-reads only the items, keeping query and selection: an icon landed.
 */
async function refreshItems() {
    try {
        const [apps, links] = await Promise.all([Items('apps'), Items('links')]);
        state.apps = decorate(apps ?? [], state.usage);
        state.links = decorate(links ?? [], state.usage);
    } catch (error) {
        Debug(`refreshItems: ${error}`);
    }

    render();
}

/**
 * Copies the target of the selected entry and says so.
 * @param {object|undefined} entry
 */
async function copyEntry(entry) {
    if (!entry) {
        return;
    }

    try {
        await CopyTarget(entry.id);
        showToast(t('toast.copied'));
    } catch (error) {
        Debug(`copy ${entry.id}: ${error}`);
    }
}

/**
 * Runs one keyboard action. The editor keys are accepted and logged until
 * the editor exists.
 * @param {{type: string, delta?: number, index?: number}} action
 */
function dispatch(action) {
    const entry = selectedEntry();

    switch (action.type) {
        case 'move': {
            const layout = layoutOf(state);
            if (layout.entries.length === 0) {
                break;
            }

            // A vertical move (|delta| > 1) on the grouped cards grid goes
            // by geometry instead of by index: see verticalNeighbour for
            // why a fixed column count cannot be trusted there.
            const cards = TABS[state.tab].layout === 'cards' && !layout.unified;
            if (cards && Math.abs(action.delta) > 1) {
                const target = verticalNeighbour(Math.sign(action.delta));
                if (target !== null) {
                    state.selected = target;
                    render();
                }
                break;
            }

            state.selected = nextIndex(state.selected, action.delta, layout.entries.length);
            render();
            break;
        }
        case 'open':
            openEntry(entry);
            break;
        case 'openAlt':
            openEntry(entry, 'alt');
            break;
        case 'openAll': {
            // Scoped to a chip on purpose: with the search box empty and a
            // category picked, "open all" means that category, never the
            // unified search results or a whole tab.
            if (state.query !== '' || state.category === '') {
                break;
            }

            const items = chipItems(state);
            if (items.length > MAX_CHIP_OPEN) {
                showToast(t('toast.tooMany'));
                break;
            }

            items.forEach((item) => openEntry(item));
            break;
        }
        case 'reveal':
            if (entry && entry.kind === 'app') {
                RevealInFinder(entry.id).catch((error) => Debug(`reveal ${entry.id}: ${error}`));
            }
            break;
        case 'copy':
            copyEntry(entry);
            break;
        case 'favorite':
            if (entry) {
                toggleFavorite(entry);
            }
            break;
        case 'category': {
            const chips = document.querySelectorAll('#categories .chip');
            if (chips[action.index]) {
                chips[action.index].click();
            }
            break;
        }
        case 'tab':
            selectTab(otherTab(state.tab));
            break;
        case 'clear':
            search.value = '';
            state.query = '';
            state.selected = 0;
            render();
            break;
        case 'hide':
            Hide();
            break;
        case 'help':
            state.helpOpen = !state.helpOpen;
            renderHelp(state, t);
            break;
        case 'new':
        case 'edit':
        case 'delete':
        case 'closeEditor':
            Debug(`editor action ${action.type} (not available yet)`);
            break;
    }
}

installKeyboard(
    () => ({query: state.query, editing: state.editing, helpOpen: state.helpOpen, columns: columns()}),
    dispatch,
);

search.addEventListener('input', () => {
    state.query = search.value;
    state.selected = 0;
    render();
});

document.querySelectorAll('#tabs [role="tab"]').forEach((button) => {
    button.addEventListener('click', () => selectTab(button.dataset.tab));
});

// Like Cmd+Tab, the overlay goes away as soon as something else takes
// focus. `state.editing` keeps it up instead: the flag a native dialog
// will set, and the same one the future editor will set once it exists.
window.addEventListener('blur', () => {
    setTimeout(() => {
        if (document.hasFocus() || state.editing) {
            return;
        }

        Debug('window blur');
        Hide();
    }, BLUR_GRACE_MS);
});

// A blocked request never throws in the page; without this, a stricter
// CSP than the page expects would fail silently instead of showing up in
// the log a developer actually reads.
document.addEventListener('securitypolicyviolation', (event) => {
    Debug(`csp ${event.violatedDirective} ${event.blockedURI}`);
});

// Every time a shortcut shows the window, the page starts clean on the tab
// of that shortcut, with the search box ready to type into.
EventsOn('shown', (tab) => {
    Debug(`shown ${JSON.stringify(tab)}`);
    state = initialState(tab);
    search.value = '';
    animateAppearance();
    refresh().then(() => search.focus());
});

// An icon fetched in the background is on disk: repaint with it.
EventsOn('icons', () => {
    refreshItems();
});

/**
 * Asks Go once for the build facts the help panel shows; they do not
 * change while the app runs.
 */
async function loadAbout() {
    try {
        setAbout(await About());
        renderHelp(state, t);
    } catch (error) {
        Debug(`about: ${error}`);
    }
}

loadAbout();

refresh();
