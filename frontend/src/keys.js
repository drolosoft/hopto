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
 * @param {{query: string, editing: boolean, helpOpen: boolean, columns: number}} context
 * @returns {{type: string, delta?: number, index?: number}|null}
 */
export function actionFor(event, context) {
    if (event.isComposing || event.key === 'Process') {
        return null;
    }

    const key = event.key.length === 1 ? event.key.toLowerCase() : event.key;

    if (context.editing) {
        return key === 'Escape' ? {type: 'closeEditor'} : null;
    }

    if (context.helpOpen) {
        return key === 'Escape' || key === '?' || (event.metaKey && key === '/') ? {type: 'help'} : null;
    }

    if (event.metaKey && /^[1-9]$/.test(key)) {
        return {type: 'category', index: Number(key) - 1};
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
