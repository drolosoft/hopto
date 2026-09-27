/**
 * The editor panel: fields, category chips, problems and notes, built
 * with createElement in the place of the list. It decides nothing;
 * editing.js owns the draft and calls in here to paint it.
 */
import {problemText, hostOf} from './draft.js';

// The parts of the page the editor takes over while it is open.
const elements = {
    editor: document.getElementById('editor'),
    content: document.getElementById('content'),
    categories: document.getElementById('categories'),
};

// The text fields of a link, top to bottom, in the order the spec fixes.
const LINK_FIELDS = ['url', 'name', 'description'];

// What mountEditor built, so refreshEditor can update it in place instead
// of rebuilding the inputs under the user's cursor. Null while closed.
let parts = null;

/**
 * The text fields a draft shows.
 * @param {{tab: string}} draft
 * @returns {string[]}
 */
function fieldsOf(draft) {
    return LINK_FIELDS;
}

/**
 * One labelled text field with its problem line underneath. The URL is a
 * text input with a URL keyboard hint, not type="url": WebKit refuses
 * setSelectionRange on url inputs, and the caret has to go to the end.
 * @param {string} name
 * @param {Function} t
 * @param {{onInput: (field: string, value: string) => void}} handlers
 * @returns {{wrapper: HTMLDivElement, input: HTMLInputElement, problem: HTMLParagraphElement}}
 */
function textField(name, t, handlers) {
    const wrapper = document.createElement('div');
    wrapper.className = 'field';

    const input = document.createElement('input');
    input.id = `editor-${name}`;
    input.type = 'text';
    input.inputMode = name === 'url' ? 'url' : 'text';
    input.autocomplete = 'off';
    input.spellcheck = false;

    const label = document.createElement('label');
    label.className = 'caption';
    label.htmlFor = input.id;
    label.textContent = t(`editor.${name}`);

    const problem = document.createElement('p');
    problem.className = 'problem';
    problem.id = `problem-${name}`;
    problem.hidden = true;
    input.setAttribute('aria-describedby', problem.id);

    input.addEventListener('input', () => handlers.onInput(name, input.value));

    wrapper.append(label, input, problem);

    return {wrapper, input, problem};
}

/**
 * Shows or hides a problem line.
 * @param {HTMLParagraphElement} line
 * @param {string|undefined} key
 * @param {Function} t
 */
function showProblem(line, key, t) {
    line.hidden = !key;
    line.textContent = problemText(t, key ?? '');
}

/**
 * A fingerprint of the chip row: the chips are rebuilt only when it
 * changes, so the new-category input keeps its focus while typing.
 * @param {object} draft
 * @param {{id: string}[]} categories
 * @returns {string}
 */
function chipsKey(draft, categories) {
    return `${categories.map((category) => category.id).join(',')}|${draft.newCategory !== null}`;
}

/**
 * Rebuilds the chips: one per category, then "New category…", which
 * turns into a text input while a new name is being typed.
 * @param {object} draft
 * @param {{id: string, name: string}[]} categories
 * @param {Function} t
 * @param {{onInput: Function, onChip: (index: number) => void}} handlers
 */
function buildChips(draft, categories, t, handlers) {
    const group = parts.chips;
    group.replaceChildren();

    categories.forEach((category, index) => {
        group.appendChild(chipButton(category.name, index, handlers));
    });

    if (draft.newCategory === null) {
        const fresh = chipButton(t('editor.newCategory'), categories.length, handlers);
        fresh.classList.add('new');
        group.appendChild(fresh);
    } else {
        const input = document.createElement('input');
        input.id = 'editor-new-category';
        input.type = 'text';
        input.className = 'chip new';
        input.autocomplete = 'off';
        input.spellcheck = false;
        input.placeholder = t('editor.newCategoryName');
        input.setAttribute('aria-label', t('editor.newCategoryName'));
        input.value = draft.newCategory;
        input.addEventListener('input', () => handlers.onInput('newCategory', input.value));
        group.appendChild(input);
    }

    parts.chipsKey = chipsKey(draft, categories);
}

/**
 * One category chip. The number shortcut is in the tooltip; the chips
 * themselves are skipped by Tab, the row is the single stop.
 * @param {string} text
 * @param {number} index
 * @param {{onChip: (index: number) => void}} handlers
 * @returns {HTMLButtonElement}
 */
function chipButton(text, index, handlers) {
    const chip = document.createElement('button');
    chip.type = 'button';
    chip.className = 'chip';
    chip.role = 'radio';
    chip.tabIndex = -1;
    chip.textContent = text;

    if (index < 9) {
        chip.title = `⌘${index + 1}`;
    }

    chip.addEventListener('click', () => handlers.onChip(index));

    return chip;
}

/**
 * Marks the chosen chip for the eye and for screen readers.
 * @param {object} draft
 * @param {{id: string}[]} categories
 */
function markChips(draft, categories) {
    parts.chips.querySelectorAll('[role="radio"]').forEach((chip, index) => {
        const chosen = draft.newCategory === null && categories[index]?.id === draft.category;
        chip.setAttribute('aria-checked', String(chosen));
    });
}

/**
 * The lines under the fields: reading the page, the exact duplicate (it
 * blocks the save), the links on the same site (they only warn), and
 * plain http. Rebuilt on every refresh; none of it takes focus.
 * @param {object} draft
 * @param {Function} t
 */
function renderNotes(draft, t) {
    const lines = [];

    if (draft.inspecting) {
        lines.push({text: t('editor.inspecting'), kind: 'quiet'});
    }

    if (draft.duplicate) {
        lines.push({text: t('editor.duplicate', {name: draft.duplicate.name}), kind: 'blocking'});
    }

    if (draft.sameHost.length > 0) {
        lines.push({text: t('editor.sameHost', {names: draft.sameHost.map((ref) => ref.name).join(', ')}), kind: 'quiet'});
    }

    if (draft.insecure) {
        lines.push({text: t('editor.insecure'), kind: 'warning'});
    }

    parts.notes.replaceChildren();

    for (const line of lines) {
        const paragraph = document.createElement('p');
        paragraph.className = line.kind;
        paragraph.textContent = line.text;
        parts.notes.appendChild(paragraph);
    }
}

/**
 * The icon preview: the inspected icon, else the initial of the name or
 * of the host, in the same tile the list uses.
 * @param {object} draft
 */
function renderPreview(draft) {
    const tile = parts.preview;
    tile.replaceChildren();

    if (draft.iconDataUrl) {
        const image = document.createElement('img');
        image.src = draft.iconDataUrl;
        image.alt = '';
        tile.appendChild(image);
        return;
    }

    const label = draft.name.trim() || hostOf(draft.url);
    tile.textContent = label.charAt(0).toUpperCase();
}

/**
 * Builds the editor in the place of the list and paints the draft.
 * @param {object} draft
 * @param {{id: string, name: string}[]} categories the real chips of the draft's tab
 * @param {Function} t
 * @param {{onInput: (field: string, value: string) => void, onChip: (index: number) => void}} handlers
 */
export function mountEditor(draft, categories, t, handlers) {
    elements.content.hidden = true;
    elements.categories.hidden = true;
    elements.editor.replaceChildren();
    elements.editor.hidden = false;
    elements.editor.setAttribute('aria-label', t('editor.label'));

    const heading = document.createElement('div');
    heading.className = 'heading';

    const preview = document.createElement('span');
    preview.className = 'tile preview';

    const title = document.createElement('h2');
    title.textContent = t(draft.mode === 'edit' ? 'editor.title.edit' : 'editor.title.add');

    heading.append(preview, title);
    elements.editor.appendChild(heading);

    const fields = {};
    for (const name of fieldsOf(draft)) {
        const field = textField(name, t, handlers);
        field.input.value = draft[name] ?? '';
        fields[name] = field;
        elements.editor.appendChild(field.wrapper);
    }

    const categoryField = document.createElement('div');
    categoryField.className = 'field';

    const caption = document.createElement('span');
    caption.className = 'caption';
    caption.textContent = t('editor.category');

    const chips = document.createElement('div');
    chips.id = 'editor-categories';
    chips.role = 'radiogroup';
    chips.tabIndex = 0;
    chips.setAttribute('aria-label', t('editor.category'));

    const categoryProblem = document.createElement('p');
    categoryProblem.className = 'problem';
    categoryProblem.id = 'problem-category';
    categoryProblem.hidden = true;

    categoryField.append(caption, chips, categoryProblem);
    elements.editor.appendChild(categoryField);

    const notes = document.createElement('div');
    notes.className = 'notes';
    notes.setAttribute('aria-live', 'polite');
    elements.editor.appendChild(notes);

    const hints = document.createElement('p');
    hints.className = 'hints';
    hints.textContent = t('editor.hints');
    elements.editor.appendChild(hints);

    parts = {fields, chips, categoryProblem, notes, preview, chipsKey: ''};
    refreshEditor(draft, categories, t, handlers);
}

/**
 * Paints the draft into the mounted editor. A value is only written back
 * into a field the user is not typing in: the inspection fills name and
 * description, never under the caret.
 * @param {object} draft
 * @param {{id: string, name: string}[]} categories
 * @param {Function} t
 * @param {{onInput: Function, onChip: Function}} handlers
 */
export function refreshEditor(draft, categories, t, handlers) {
    if (!parts) {
        return;
    }

    for (const [name, field] of Object.entries(parts.fields)) {
        const value = draft[name] ?? '';
        if (document.activeElement !== field.input && field.input.value !== value) {
            field.input.value = value;
        }

        showProblem(field.problem, draft.problems[name], t);
        field.input.setAttribute('aria-invalid', String(Boolean(draft.problems[name])));
    }

    if (chipsKey(draft, categories) !== parts.chipsKey) {
        buildChips(draft, categories, t, handlers);
    }

    markChips(draft, categories);
    showProblem(parts.categoryProblem, draft.problems.category, t);
    renderNotes(draft, t);
    renderPreview(draft);
}

/**
 * Takes the editor down and gives the place back to the list. The chip
 * bar is left hidden: the next render of the list decides on it.
 */
export function hideEditor() {
    if (!parts && elements.editor.hidden) {
        return;
    }

    elements.editor.hidden = true;
    elements.editor.replaceChildren();
    elements.content.hidden = false;
    parts = null;
}

/**
 * Moves the focus to a field ("url", "name", "description", "category"
 * for the chip row, "newCategory" for its input), caret at the end.
 * @param {string} name
 */
export function focusField(name) {
    if (!parts) {
        return;
    }

    let target = parts.fields[name]?.input;
    if (name === 'category') {
        target = parts.chips;
    } else if (name === 'newCategory') {
        target = document.getElementById('editor-new-category');
    }

    if (!target) {
        return;
    }

    target.focus();

    if (target instanceof HTMLInputElement) {
        target.setSelectionRange(target.value.length, target.value.length);
    }
}

/**
 * Which field has the focus, for the keyboard: arrows move chips only on
 * the chip row.
 * @returns {string}
 */
export function focusedField() {
    if (!parts) {
        return '';
    }

    const active = document.activeElement;

    for (const [name, field] of Object.entries(parts.fields)) {
        if (field.input === active) {
            return name;
        }
    }

    if (active?.id === 'editor-new-category') {
        return 'newCategory';
    }

    if (active === parts.chips || parts.chips.contains(active)) {
        return 'category';
    }

    return '';
}
