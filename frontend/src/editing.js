/**
 * The editor's flow: opening it from the add row or ⌘N, reading the page
 * of a typed URL, saving through Go and going back to the list. The draft
 * lives in the page state, so `shown` wipes it along with everything
 * else; this module only keeps a timer and a counter of inspections.
 */
import {InspectURL, AddLink, UpdateLink, AddCategory, Debug} from '../wailsjs/go/main/App';
import {looksLikeURL, withScheme, withInput, withInspection, pickCategory, moveCategory, localProblems, linkInput, withSaveResult} from './draft.js';
import {mountEditor, refreshEditor, hideEditor, focusField, focusedField} from './editor.js';

// How long the URL field has to rest before the page is read: long
// enough not to fire on every keystroke, short enough to feel immediate.
const INSPECT_DELAY_MS = 400;

// What the page gives the editor: its state, the translator, a repaint,
// and what to do once a save lands or the editor closes.
let host = null;

// The pending "read the page" timer.
let inspectTimer = 0;

// Counts inspections: an answer is only used when no newer inspection
// (or a close) has happened since it was asked for.
let inspection = 0;

// The callbacks the editor's DOM calls back into.
const handlers = {
    onInput: (field, value) => typed(field, value),
    onChip: (index) => chooseCategory(index),
};

/**
 * Connects the editor to the page.
 * @param {{state: () => object, t: () => Function, render: () => void, saved: (key: string, toastKey: string) => Promise<void>, closed: (query: string) => void, toast: (text: string) => void}} pageHost
 */
export function installEditing(pageHost) {
    host = pageHost;
}

/**
 * The real (not virtual) categories of a tab, in chip order.
 * @param {string} tab
 * @returns {{id: string, name: string}[]}
 */
export function editableCategories(tab) {
    const state = host.state();
    const categories = tab === 'links' ? state.linkCategories : state.appCategories;

    return categories.filter((category) => !category.virtual);
}

/**
 * The current draft, or null when the editor is closed.
 * @returns {object|null}
 */
function currentDraft() {
    return host.state().draft;
}

/**
 * Replaces the draft in the page state.
 * @param {object} draft
 */
function setDraft(draft) {
    host.state().draft = draft;
}

/**
 * Opens the editor on a draft, focus on the URL, and reads the page at
 * once when the draft already has an address.
 * @param {object} draft
 */
export function openEditor(draft) {
    const state = host.state();
    state.editing = true;
    state.confirming = null;
    state.helpOpen = false;
    setDraft(draft);

    mountEditor(draft, editableCategories(draft.tab), host.t(), handlers);
    focusField('url');

    if (draft.tab === 'links' && draft.mode === 'add' && looksLikeURL(draft.url)) {
        inspectNow();
    }
}

/**
 * Repaints the open editor from the draft.
 */
export function refreshEditorView() {
    const draft = currentDraft();
    if (draft) {
        refreshEditor(draft, editableCategories(draft.tab), host.t(), handlers);
    }
}

/**
 * The user typed in a field: store it, and read the page again once a
 * new URL has settled.
 * @param {string} field
 * @param {string} value
 */
function typed(field, value) {
    setDraft(withInput(currentDraft(), field, value));

    if (field === 'url') {
        scheduleInspection();
    }

    host.render();
}

/**
 * Restarts the wait before reading the page, and makes any answer still
 * on its way for the previous URL count for nothing.
 */
function scheduleInspection() {
    clearTimeout(inspectTimer);
    inspection += 1;

    if (looksLikeURL(currentDraft().url)) {
        inspectTimer = setTimeout(inspectNow, INSPECT_DELAY_MS);
    }
}

/**
 * Asks Go what the page at the draft's URL says about itself. The answer
 * is dropped if the editor closed, a newer inspection started, or the
 * URL changed meanwhile (withInspection checks that last one).
 */
async function inspectNow() {
    const draft = currentDraft();
    if (!host.state().editing || !draft || draft.tab !== 'links') {
        return;
    }

    inspection += 1;
    const ticket = inspection;

    setDraft({...draft, inspecting: true});
    host.render();

    let info = null;
    try {
        info = await InspectURL(withScheme(draft.url));
    } catch (error) {
        Debug(`inspect: ${error}`);
    }

    const latest = currentDraft();
    if (ticket !== inspection || !host.state().editing || !latest) {
        return;
    }

    setDraft(info ? withInspection(latest, info) : {...latest, inspecting: false});
    host.render();
}

/**
 * Picks a category by position (⌘1-9 or a click); the position past the
 * last category opens the new-category field and focuses it.
 * @param {number} index
 */
export function chooseCategory(index) {
    const draft = currentDraft();
    const categories = editableCategories(draft.tab);

    setDraft(pickCategory(draft, categories, index));
    host.render();

    if (currentDraft().newCategory !== null) {
        focusField('newCategory');
    }
}

/**
 * Moves the category one chip left or right (arrows on the chip row).
 * @param {number} delta
 */
export function stepCategory(delta) {
    const draft = currentDraft();

    setDraft(moveCategory(draft, editableCategories(draft.tab), delta));
    host.render();

    focusField(currentDraft().newCategory !== null ? 'newCategory' : 'category');
}

/**
 * Creates the new category first when one is being typed, and puts it on
 * the page's own list so the chip exists even if the save then fails.
 * @param {object} draft
 * @returns {Promise<object>} the draft pointing at a real category
 */
async function withCategory(draft) {
    if (draft.newCategory === null) {
        return draft;
    }

    const view = await AddCategory(draft.tab, draft.newCategory.trim());
    const state = host.state();
    const list = draft.tab === 'links' ? state.linkCategories : state.appCategories;
    list.push(view);

    return {...draft, category: view.id, newCategory: null};
}

/**
 * Sends the draft to Go: a new link, or the edit of an existing one.
 * @param {object} draft
 * @returns {Promise<{id: string, problems: Object<string, string>, duplicate: object|null}>}
 */
function persist(draft) {
    const input = linkInput(draft);

    return draft.mode === 'edit' ? UpdateLink(draft.id, input) : AddLink(input);
}

/**
 * Enter: saves when the draft is complete, otherwise shows what is
 * missing under its field. A duplicate blocks; Go's own problems come
 * back under their fields; a failure to write is a toast.
 */
export async function saveEditor() {
    const draft = currentDraft();
    if (!draft || draft.saving) {
        return;
    }

    const missing = localProblems(draft);
    if (Object.keys(missing).length > 0 || draft.duplicate) {
        setDraft({...draft, problems: {...draft.problems, ...missing}});
        host.render();
        return;
    }

    setDraft({...draft, saving: true});

    try {
        const ready = await withCategory(currentDraft());
        const result = await persist(ready);

        // A `shown` may have wiped the editor while Go was writing.
        if (!host.state().editing) {
            return;
        }

        const {draft: answered, savedId} = withSaveResult(ready, result);
        if (!savedId) {
            setDraft(answered);
            host.render();
            return;
        }

        finish();
        await host.saved(`${ready.tab}:${savedId}`, ready.mode === 'edit' ? 'toast.saved' : 'toast.added');
    } catch (error) {
        Debug(`save: ${error}`);

        if (currentDraft()) {
            setDraft({...currentDraft(), saving: false});
        }

        host.toast(host.t()('toast.saveFailed', {error: String(error)}));
        host.render();
    }
}

/**
 * Esc: while a new category name is being typed and there are chips to
 * go back to, Esc only drops that name; otherwise it closes the editor
 * and gives the search box its text back.
 */
export function closeEditor() {
    const draft = currentDraft();
    const canDropName = draft
        && draft.newCategory !== null
        && editableCategories(draft.tab).length > 0
        && focusedField() === 'newCategory';

    if (canDropName) {
        setDraft({...draft, newCategory: null});
        host.render();
        focusField('category');
        return;
    }

    finish();
    host.closed(draft?.returnQuery ?? '');
}

/**
 * Takes the editor down without saying where to go next.
 */
function finish() {
    clearTimeout(inspectTimer);
    inspection += 1;

    const state = host.state();
    state.editing = false;
    state.draft = null;

    hideEditor();
}
