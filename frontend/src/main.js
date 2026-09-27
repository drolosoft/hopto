/**
 * The glue: state, calls to Go, and the wiring between the modules.
 */
import './style.css';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {Items, Categories, Usage, Settings, LibraryStatus, About, Launch, OpenLink, OpenLinkWith, CopyTarget, RevealInFinder, EditLibrary, Hide, TabChanged, Debug, ToggleFavorite, DeleteLink, DeleteApp, HideApp, UnhideApp, RenameCategory, DeleteCategory} from '../wailsjs/go/main/App';
import {decorate, visibleUnder, sections, unifiedSearch, FAVORITES, HIDDEN} from './filter.js';
import {nextIndex} from './keys.js';
import {initialState, chipItems, removalOf} from './state.js';
import {TABS, otherTab} from './tabs.js';
import {resolveLanguage, translator} from './i18n.js';
import {renderAll, renderHelp, showToast, animateAppearance, columns, searchBox, verticalNeighbour, setAbout, renameBox} from './render.js';
import {installKeyboard} from './keyboard.js';
import {addOffer, newDraft, editDraft, adoptDraft} from './draft.js';
import {hideEditor, focusedField} from './editor.js';
import {installEditing, editableCategories, openEditor, closeEditor, saveEditor, chooseCategory, stepCategory, refreshEditorView, editDuplicate, pickApp} from './editing.js';
import {installWelcome, checkWelcome, hideWelcome, presentWelcome} from './welcome.js';

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
 * The "＋ Add" row the search offers when nothing matches, or at the end
 * when the text is an address. It is an entry like any other, so the
 * keyboard, the selection and the footer reach it with no special case.
 * @param {{text: string, url: boolean}} offer
 * @returns {object}
 */
function addEntry(offer) {
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
 * @returns {object}
 */
function pickEntry() {
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
 * @param {object} current
 * @returns {{entries: object[], groups: {title: string, start: number, count: number}[], unified: boolean}}
 */
export function layoutOf(current) {
    if (current.query) {
        const entries = unifiedSearch(current.apps, current.links, current.query);
        const offer = addOffer(current.query, entries.length);
        if (offer) {
            entries.push(addEntry(offer));

            // On the apps tab the search also offers the native dialog
            // (supuesto 4): the add row itself always adds a link.
            if (current.tab === 'apps') {
                entries.push(pickEntry());
            }
        }

        return {entries, groups: [], unified: true};
    }

    const items = current.tab === 'links' ? current.links : current.apps;
    const categories = current.tab === 'links' ? current.linkCategories : current.appCategories;

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

/**
 * Repaints from the state, keeping the selection inside the list.
 */
function render() {
    // While the editor is open the list is not on screen; only the editor
    // repaints, so the search box and chips keep what they had.
    if (state.editing && state.draft) {
        refreshEditorView();
        return;
    }

    hideEditor();

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
 * Drops a pending "delete? ↩ yes · Esc no" question and any chip being
 * renamed in place, without rendering. A stale question is not just a
 * cosmetic leftover: dispatch's own guard treats a pending confirming as
 * "the next Enter answers it", so a mouse click that leaves one armed for
 * an item the user never re-selected would delete or hide the wrong
 * thing. `dispatch`, `selectTab`, `selectCategory` and `openAt` all call
 * this before doing what the key or the click was actually for.
 * @returns {boolean} whether a question or a rename was actually pending
 */
function dropPendingQuestion() {
    const had = state.confirming !== null || state.renaming !== null;
    state.confirming = null;
    state.renaming = null;

    return had;
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

    dropPendingQuestion();
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
    dropPendingQuestion();
    state.category = category;
    state.selected = 0;
    render();
    search.focus();
}

/**
 * A link draft from typed text, with the active chip preselected when the
 * links tab is showing one, and the search text kept for Esc.
 * @param {string} text
 * @returns {object}
 */
function linkDraftFrom(text) {
    return newDraft({
        tab: 'links',
        text,
        category: state.tab === 'links' ? state.category : '',
        categories: editableCategories('links'),
        returnQuery: state.query,
    });
}

/**
 * An empty app draft, with the active chip preselected when the apps tab
 * shows one; the editor opens the dialog on it.
 * @returns {object}
 */
function appDraft() {
    return newDraft({
        tab: 'apps',
        category: state.tab === 'apps' ? state.category : '',
        categories: editableCategories('apps'),
        returnQuery: state.query,
    });
}

/**
 * After a save: back to the list, on the saved item's tab, with the item
 * selected and a short confirmation.
 * @param {string} key "links:<id>" or "apps:<id>"
 * @param {string} toastKey
 */
async function showSaved(key, toastKey) {
    const [tab] = key.split(':');

    search.value = '';
    state.query = '';
    state.category = '';

    if (tab !== state.tab) {
        state.tab = tab;
        TabChanged(tab);
    }

    await refresh();

    const index = layoutOf(state).entries.findIndex((entry) => entry.key === key || entry.web?.key === key);
    state.selected = Math.max(0, index);
    render();
    showToast(t(toastKey));
    search.focus();
}

/**
 * After Esc in the editor: the search box gets its text back and the
 * list its focus, as if the editor had never opened.
 * @param {string} query
 */
function returnToSearch(query) {
    search.value = query;
    state.query = query;
    state.selected = 0;
    render();
    search.focus();
}

/**
 * ⌘E: the editor filled with the selected link or hand-added app; on a
 * discovered app, the editor that adds it by hand, which is how it gets
 * a category.
 * @param {object|undefined} entry
 */
function editEntry(entry) {
    if (!entry || (entry.kind !== 'link' && entry.kind !== 'app')) {
        return;
    }

    if (entry.kind === 'app' && entry.source !== 'library') {
        openEditor(adoptDraft(entry, editableCategories('apps'), state.query));
        return;
    }

    const tab = entry.kind === 'link' ? 'links' : 'apps';
    openEditor(editDraft(entry, editableCategories(tab), state.query));
}

/**
 * The editor's "⌘E edits it": the same editor on the item a duplicate
 * points at.
 * @param {{tab: string, id: string}} ref
 */
function editReference(ref) {
    const key = `${ref.tab}:${ref.id}`;
    editEntry([...state.links, ...state.apps].find((item) => item.key === key));
}

/**
 * ⌘⌫ on an entry. A hidden app comes back at once, nothing is lost by
 * that; anything else waits for Enter with the question in the row and
 * in the footer.
 * @param {object|undefined} entry
 */
function askRemoval(entry) {
    const action = removalOf(entry);
    if (action === '') {
        return;
    }

    if (action === 'unhide') {
        removeWith(() => UnhideApp(entry.id), 'toast.unhidden');
        return;
    }

    const tab = entry.kind === 'link' ? 'links' : 'apps';
    state.confirming = {action, key: entry.key, id: entry.id, tab, name: entry.name};
    render();
}

/**
 * Enter on a pending question: does what it asked.
 */
function confirmPending() {
    const pending = state.confirming;
    state.confirming = null;

    if (!pending) {
        return;
    }

    if (pending.action === 'deleteCategory') {
        removeWith(() => DeleteCategory(pending.tab, pending.id), 'toast.deleted', () => {
            state.category = '';
        });
        return;
    }

    if (pending.action === 'hide') {
        removeWith(() => HideApp(pending.id), 'toast.hidden');
        return;
    }

    const remove = pending.tab === 'links' ? DeleteLink : DeleteApp;
    removeWith(() => remove(pending.id), 'toast.deleted');
}

/**
 * Runs a removal through Go, re-reads everything and says so. The
 * selection keeps its index, so the next item moves under it; the
 * hidden chip, once emptied, gives way to "All".
 * @param {() => Promise<void>} call
 * @param {string} toastKey
 * @param {() => void} [after] what changes in the state once it worked
 */
async function removeWith(call, toastKey, after) {
    try {
        await call();
    } catch (error) {
        Debug(`remove: ${error}`);
        showToast(t('toast.deleteFailed', {error: String(error)}));
        render();
        return;
    }

    after?.();
    await refresh();

    if (state.category === HIDDEN && !state.apps.some((app) => app.hidden)) {
        state.category = '';
        render();
    }

    showToast(t(toastKey));
    search.focus();
}

/**
 * The chip ⌘⇧E and ⌘⇧⌫ act on: the active one, when it is a category of
 * the user (not All, the favourites or a virtual chip) and nothing is
 * typed.
 * @returns {{id: string, name: string}|null}
 */
function activeUserCategory() {
    if (state.query !== '' || state.category === '' || state.category === FAVORITES) {
        return null;
    }

    return editableCategories(state.tab).find((category) => category.id === state.category) ?? null;
}

/**
 * ⌘⇧E: the active chip turns into a field holding its name, selected.
 */
function startRename() {
    const category = activeUserCategory();
    if (!category) {
        return;
    }

    state.renaming = {tab: state.tab, id: category.id, name: category.name};
    render();

    const box = renameBox();
    box?.focus();
    box?.select();
}

/**
 * Enter (keep) or Esc in the chip being renamed. An empty or unchanged
 * name is a cancel; Go's own checks (length, invisible characters) come
 * back as a toast.
 * @param {boolean} keep
 */
async function finishRename(keep) {
    const renaming = state.renaming;
    const name = renameBox()?.value.trim() ?? '';
    state.renaming = null;

    if (keep && renaming && name !== '' && name !== renaming.name) {
        try {
            await RenameCategory(renaming.tab, renaming.id, name);
            await refresh();
            showToast(t('toast.renamed'));
        } catch (error) {
            Debug(`rename ${renaming.id}: ${error}`);
            showToast(t('toast.renameFailed', {error: String(error)}));
        }
    }

    render();
    search.focus();
}

/**
 * ⌘⇧⌫: a chip that still has items is refused on the spot (Go refuses it
 * too); an empty one waits for Enter with the question in the footer.
 */
function askCategoryDeletion() {
    const category = activeUserCategory();
    if (!category) {
        return;
    }

    const items = state.tab === 'links' ? state.links : state.apps;
    if (items.some((item) => item.category === category.id && item.source === 'library')) {
        showToast(t('toast.categoryInUse'));
        return;
    }

    state.confirming = {action: 'deleteCategory', key: `category:${category.id}`, id: category.id, tab: state.tab, name: category.name};
    render();
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

    if (entry.kind === 'add') {
        openEditor(linkDraftFrom(entry.text));
        return;
    }

    if (entry.kind === 'pick') {
        openEditor(appDraft());
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
    if (dropPendingQuestion()) {
        render();
    }

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
 * Runs one keyboard action.
 * @param {{type: string, delta?: number, index?: number}} action
 */
function dispatch(action) {
    // Any key but the one that answers a pending question drops it first
    // (shared with the chip, tab and row click handlers via
    // dropPendingQuestion), so moving away never deletes; Esc only drops
    // it. saveRename and cancelRename read state.renaming themselves, so
    // they are excluded here rather than losing it before they see it.
    const answering = action.type === 'confirm' || action.type === 'saveRename' || action.type === 'cancelRename';
    if (!answering && dropPendingQuestion()) {
        render();

        if (action.type === 'cancelConfirm') {
            return;
        }
    }

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
            openEditor(state.tab === 'apps' ? appDraft() : linkDraftFrom(''));
            break;
        case 'save':
            saveEditor();
            break;
        case 'pickApp':
            pickApp();
            break;
        case 'closeEditor':
            closeEditor();
            break;
        case 'pickCategory':
            chooseCategory(action.index);
            break;
        case 'moveCategory':
            stepCategory(action.delta);
            break;
        case 'edit':
            editEntry(entry);
            break;
        case 'delete':
            askRemoval(entry);
            break;
        case 'confirm':
            confirmPending();
            break;
        case 'renameCategory':
            startRename();
            break;
        case 'saveRename':
            finishRename(true);
            break;
        case 'cancelRename':
            finishRename(false);
            break;
        case 'deleteCategory':
            askCategoryDeletion();
            break;
        case 'editDuplicate':
            editDuplicate();
            break;
    }
}

installKeyboard(
    () => ({query: state.query, editing: state.editing, helpOpen: state.helpOpen, columns: columns(), field: state.editing ? focusedField() : '', confirming: Boolean(state.confirming), renaming: Boolean(state.renaming)}),
    dispatch,
);

installEditing({
    state: () => state,
    t: () => t,
    render,
    saved: showSaved,
    closed: returnToSearch,
    toast: showToast,
    edit: editReference,
});

installWelcome({t: () => t, onClosed: () => search.focus()});

// The broken-file notice's button: the fix is in the file itself.
document.querySelector('#status .fix').addEventListener('click', () => {
    EditLibrary().catch((error) => Debug(`edit library: ${error}`));
});

search.addEventListener('input', () => {
    state.query = search.value;
    state.selected = 0;
    state.confirming = null;
    render();
});

document.querySelectorAll('#tabs [role="tab"]').forEach((button) => {
    button.addEventListener('click', () => selectTab(button.dataset.tab));
});

// Like Cmd+Tab, the overlay goes away as soon as something else takes
// focus, except while the editor is open (a click elsewhere to copy a URL
// must not lose the draft) or a native dialog is up (the dialog itself
// takes the focus away from the page).
window.addEventListener('blur', () => {
    setTimeout(() => {
        if (document.hasFocus() || state.editing || state.dialogOpen) {
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
    hideWelcome();
    search.value = '';
    animateAppearance();
    refresh().then(() => search.focus()).then(checkWelcome);
});

// An icon fetched in the background is on disk: repaint with it.
EventsOn('icons', () => {
    refreshItems();
});

// The menu bar item's Help: Go shows the panel first (which resets the
// state), then asks for the help on top of it.
EventsOn('help', () => {
    state.helpOpen = true;
    renderHelp(state, t);
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

// Once the page has loaded, Go may show the panel for the welcome.
refresh().then(presentWelcome);
