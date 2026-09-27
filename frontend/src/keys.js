/**
 * Keyboard arithmetic of the page, kept free of the DOM so it runs under
 * `node --test`.
 */

/**
 * The index after moving the selection by delta positions in a list of
 * count items, wrapping around at both ends. The double modulo keeps the
 * result in range even when the jump is larger than the list, which happens
 * on the apps grid when a search leaves fewer results than columns.
 * @param {number} selected
 * @param {number} delta
 * @param {number} count
 * @returns {number}
 */
export function nextIndex(selected, delta, count) {
    if (count === 0) {
        return 0;
    }

    return (((selected + delta) % count) + count) % count;
}

/**
 * Turns a key press into the action the page should take, or null when
 * the search box should keep the key. Letters are compared lowercased so
 * Caps Lock changes nothing; a press during IME composition is ignored.
 * @param {{key: string, metaKey: boolean, altKey: boolean, shiftKey: boolean, ctrlKey: boolean, isComposing: boolean}} event
 * @param {{query: string, editing: boolean, helpOpen: boolean, columns: number, field?: string}} context
 * @returns {{type: string, delta?: number, index?: number}|null}
 */
export function actionFor(event, context) {
    if (event.isComposing || event.key === 'Process') {
        return null;
    }

    const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;

    if (context.editing) {
        return editorAction(event, key, context.field);
    }

    if (context.helpOpen) {
        return key === 'Escape' || key === '?' || (event.metaKey && key === '/') ? {type: 'help'} : null;
    }

    if (event.metaKey && /^[1-9]$/.test(key)) {
        // A query already typed means the chips are not what the digit is
        // about; ⌘1-9 only picks a category while the search box is empty.
        return context.query ? null : {type: 'category', index: Number(key) - 1};
    }

    if (event.metaKey) {
        switch (key) {
            case 'f': return {type: 'favorite'};
            case 'c': return {type: 'copy'};
            case 'n': return {type: 'new'};
            case 'e': return {type: 'edit'};
            case '/': return {type: 'help'};
            case 'Backspace': return {type: 'delete'};
            case 'Enter': return event.shiftKey ? {type: 'openAll'} : {type: 'openAlt'};
            default: return null;
        }
    }

    switch (key) {
        case 'ArrowRight': return {type: 'move', delta: 1};
        case 'ArrowLeft': return {type: 'move', delta: -1};
        case 'ArrowDown': return {type: 'move', delta: context.columns};
        case 'ArrowUp': return {type: 'move', delta: -context.columns};
        case 'Tab': return {type: 'tab'};
        case 'Enter': return event.altKey ? {type: 'reveal'} : {type: 'open'};
        case 'Escape': return context.query ? {type: 'clear'} : {type: 'hide'};
        case '?': return context.query ? null : {type: 'help'};
        default: return null;
    }
}

/**
 * The keys of the open editor. Typing and Tab stay with the fields; Enter
 * saves from any of them, ⌘1-9 picks a category, and the arrows move
 * between chips only while the chip row has the focus (in a text field
 * they belong to the caret).
 * @param {{key: string, metaKey: boolean, shiftKey: boolean}} event
 * @param {string} key the key, lowercased when it is a letter
 * @param {string|undefined} field the focused field of the editor
 * @returns {{type: string, delta?: number, index?: number}|null}
 */
function editorAction(event, key, field) {
    if (key === 'Escape') {
        return {type: 'closeEditor'};
    }

    if (key === 'Enter' && !event.shiftKey) {
        return {type: 'save'};
    }

    if (event.metaKey && /^[1-9]$/.test(key)) {
        return {type: 'pickCategory', index: Number(key) - 1};
    }

    if (field !== 'category') {
        return null;
    }

    switch (key) {
        case 'ArrowRight':
        case 'ArrowDown':
            return {type: 'moveCategory', delta: 1};
        case 'ArrowLeft':
        case 'ArrowUp':
            return {type: 'moveCategory', delta: -1};
        default:
            return null;
    }
}
