/**
 * Global shortcuts as people read them. The settings write
 * "cmd+shift+space"; the welcome shows "⇧⌘Space", like a macOS menu.
 */

// The glyph of each modifier word the Go parser accepts.
const GLYPHS = {
    ctrl: '⌃',
    control: '⌃',
    option: '⌥',
    opt: '⌥',
    alt: '⌥',
    shift: '⇧',
    cmd: '⌘',
    command: '⌘',
};

// The order macOS itself prints modifiers in.
const ORDER = ['⌃', '⌥', '⇧', '⌘'];

// The words of a spec as Windows prints them, and the order they go in.
const WINDOWS_GLYPHS = {
    ctrl: 'Ctrl',
    control: 'Ctrl',
    option: 'Alt',
    opt: 'Alt',
    alt: 'Alt',
    shift: 'Shift',
    cmd: 'Win',
    command: 'Win',
};
// Win sits before Shift because that is how Windows itself writes
// chords such as Win+Shift+S.
const WINDOWS_ORDER = ['Ctrl', 'Alt', 'Win', 'Shift'];

// Keys with a name, translated; any other key is printed uppercased.
const NAMED_KEYS = {
    space: 'key.space',
    return: 'key.return',
    tab: 'key.tab',
    escape: 'key.escape',
};

/**
 * "cmd+shift+space" → "⇧⌘Space" on macOS, "Ctrl+Shift+Space" written as
 * "ctrl+shift+space" on Windows. A spec with no modifier or an unknown
 * word comes back as it was: better the raw text than a wrong glyph.
 * @param {string} spec
 * @param {Function} t
 * @param {string} [platform] darwin by default, or windows
 * @returns {string}
 */
export function prettyHotkey(spec, t, platform = 'darwin') {
    const onWindows = platform === 'windows';
    const table = onWindows ? WINDOWS_GLYPHS : GLYPHS;
    const order = onWindows ? WINDOWS_ORDER : ORDER;
    const raw = String(spec ?? '');
    const parts = raw.toLowerCase().split('+').map((part) => part.trim()).filter(Boolean);
    if (parts.length < 2) {
        return raw;
    }

    const key = parts.pop();
    const glyphs = new Set();

    for (const part of parts) {
        if (!table[part]) {
            return raw;
        }

        glyphs.add(table[part]);
    }

    const modifiers = order.filter((glyph) => glyphs.has(glyph));
    const name = NAMED_KEYS[key] ? t(NAMED_KEYS[key]) : key.toUpperCase();

    // Windows writes the words joined by plus signs; macOS runs the
    // symbols together like a menu does.
    return onWindows ? [...modifiers, name].join('+') : modifiers.join('') + name;
}
