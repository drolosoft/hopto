/**
 * What a chord looks like and which key starts it on each system. The
 * page gets the platform from Go's Settings; anything that is not
 * "windows" reads as macOS, which is what Linux will want too until it
 * has a table of its own.
 */

// How each modifier and named key prints. On Windows the separator
// lives in the glyph ("Ctrl+"), so '{cmd}N' reads "Ctrl+N" without the
// texts knowing which system they are on.
export const GLYPHS = {
    darwin: {cmd: '⌘', alt: '⌥', shift: '⇧', ctrl: '⌃', enter: '↩', backspace: '⌫'},
    windows: {cmd: 'Ctrl+', alt: 'Alt+', shift: 'Shift+', ctrl: 'Ctrl+', enter: 'Enter', backspace: 'Backspace'},
};

/**
 * The glyph table of a platform.
 * @param {string|undefined} platform
 * @returns {Object<string, string>}
 */
export function glyphsFor(platform) {
    return GLYPHS[platform] ?? GLYPHS.darwin;
}

/**
 * Whether the event carries the primary chord key: Command on macOS,
 * Control on Windows.
 * @param {{metaKey: boolean, ctrlKey: boolean}} event
 * @param {string|undefined} platform
 * @returns {boolean}
 */
export function primaryKey(event, platform) {
    return platform === 'windows' ? Boolean(event.ctrlKey) : Boolean(event.metaKey);
}
