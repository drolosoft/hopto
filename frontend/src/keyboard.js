/**
 * Routes key presses to actions. The search box keeps the focus, so plain
 * typing always goes to it; the keys `actionFor` recognises are taken
 * before the box sees them.
 */
import {actionFor} from './keys.js';

/**
 * Installs the document-wide key handler.
 * @param {() => {query: string, editing: boolean, helpOpen: boolean, columns: number}} contextOf
 * @param {(action: {type: string, delta?: number, index?: number}) => void} dispatch
 */
export function installKeyboard(contextOf, dispatch) {
    document.addEventListener('keydown', (event) => {
        const action = actionFor(event, contextOf());
        if (!action) {
            return;
        }

        event.preventDefault();
        dispatch(action);
    });
}
