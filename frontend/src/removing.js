/**
 * The removal flow: ⌘⌫ on a row and ⌘⇧⌫ on a chip ask first and wait for
 * Enter, ⌘⇧E renames a chip in place, and every answer goes through Go,
 * re-reads the library and says what happened. The pending question and
 * the chip being renamed live in the page state, so `shown` wipes them
 * along with everything else; this module keeps nothing of its own.
 */
import {DeleteLink, DeleteApp, HideApp, UnhideApp, RenameCategory, DeleteCategory, Debug} from '../wailsjs/go/app/App';
import {FAVORITES, HIDDEN} from './filter.js';
import {removalOf} from './state.js';
import {renameBox} from './render.js';
import {editableCategories} from './editing.js';

// What the page gives this flow: its state, the translator, a repaint,
// a full re-read from Go, a toast, and the way back to the search box.
let host = null;

/**
 * Connects the removal flow to the page.
 * @param {{state: () => object, t: () => Function, render: () => void, refresh: () => Promise<void>, toast: (text: string) => void, focusSearch: () => void}} pageHost
 */
export function installRemoving(pageHost) {
    host = pageHost;
}

/**
 * ⌘⌫ on an entry. A hidden app comes back at once, nothing is lost by
 * that; anything else waits for Enter with the question in the row and
 * in the footer.
 * @param {object|undefined} entry
 */
export function askRemoval(entry) {
    const action = removalOf(entry);
    if (action === '') {
        return;
    }

    if (action === 'unhide') {
        removeWith(() => UnhideApp(entry.id), 'toast.unhidden');
        return;
    }

    const tab = entry.kind === 'link' ? 'links' : 'apps';
    host.state().confirming = {action, key: entry.key, id: entry.id, tab, name: entry.name};
    host.render();
}

/**
 * Enter on a pending question: does what it asked.
 */
export function confirmPending() {
    const state = host.state();
    const pending = state.confirming;
    state.confirming = null;

    if (!pending) {
        return;
    }

    // The chip goes back to "All" only once Go has deleted the category,
    // and on the state of that moment: a `shown` may have replaced it
    // while Go was writing.
    if (pending.action === 'deleteCategory') {
        removeWith(() => DeleteCategory(pending.tab, pending.id), 'toast.deleted', () => {
            host.state().category = '';
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
        host.toast(host.t()('toast.deleteFailed', {error: String(error)}));
        host.render();
        return;
    }

    after?.();
    await host.refresh();

    // Read after the refresh: it may bring a new translator, and a
    // `shown` in the meantime brings a new state.
    const state = host.state();
    if (state.category === HIDDEN && !state.apps.some((app) => app.hidden)) {
        state.category = '';
        host.render();
    }

    host.toast(host.t()(toastKey));
    host.focusSearch();
}

/**
 * The chip ⌘⇧E and ⌘⇧⌫ act on: the active one, when it is a category of
 * the user (not All, the favourites or a virtual chip) and nothing is
 * typed.
 * @returns {{id: string, name: string}|null}
 */
function activeUserCategory() {
    const state = host.state();
    if (state.query !== '' || state.category === '' || state.category === FAVORITES) {
        return null;
    }

    return editableCategories(state.tab).find((category) => category.id === state.category) ?? null;
}

/**
 * ⌘⇧E: the active chip turns into a field holding its name, selected.
 */
export function startRename() {
    const category = activeUserCategory();
    if (!category) {
        return;
    }

    const state = host.state();
    state.renaming = {tab: state.tab, id: category.id, name: category.name};
    host.render();

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
export async function finishRename(keep) {
    const state = host.state();
    const renaming = state.renaming;
    const name = renameBox()?.value.trim() ?? '';
    state.renaming = null;

    if (keep && renaming && name !== '' && name !== renaming.name) {
        try {
            await RenameCategory(renaming.tab, renaming.id, name);
            await host.refresh();
            host.toast(host.t()('toast.renamed'));
        } catch (error) {
            Debug(`rename ${renaming.id}: ${error}`);
            host.toast(host.t()('toast.renameFailed', {error: String(error)}));
        }
    }

    host.render();
    host.focusSearch();
}

/**
 * ⌘⇧⌫: a chip that still has items is refused on the spot (Go refuses it
 * too); an empty one waits for Enter with the question in the footer.
 */
export function askCategoryDeletion() {
    const category = activeUserCategory();
    if (!category) {
        return;
    }

    const state = host.state();
    const items = state.tab === 'links' ? state.links : state.apps;
    if (items.some((item) => item.category === category.id && item.source === 'library')) {
        host.toast(host.t()('toast.categoryInUse'));
        return;
    }

    state.confirming = {action: 'deleteCategory', key: `category:${category.id}`, id: category.id, tab: state.tab, name: category.name};
    host.render();
}
