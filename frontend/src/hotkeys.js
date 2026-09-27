/**
 * Global shortcuts as people read them. The settings write
 * "cmd+shift+space"; the welcome shows "⌘⇧Space", like a macOS menu.
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

// Keys with a name, translated; any other key is printed uppercased.
const NAMED_KEYS = {
    space: 'key.space',
    return: 'key.return',
    tab: 'key.tab',
    escape: 'key.escape',
};

/**
 * "cmd+shift+space" → "⌘⇧Space". A spec with no modifier or an unknown
 * word comes back as it was: better the raw text than a wrong glyph.
 * @param {string} spec
 * @param {Function} t
 * @returns {string}
 */
export function prettyHotkey(spec, t) {
    const raw = String(spec ?? '');
    const parts = raw.toLowerCase().split('+').map((part) => part.trim()).filter(Boolean);
    if (parts.length < 2) {
        return raw;
    }

    const key = parts.pop();
    const glyphs = new Set();

    for (const part of parts) {
        if (!GLYPHS[part]) {
            return raw;
        }

        glyphs.add(GLYPHS[part]);
    }

    const modifiers = ORDER.filter((glyph) => glyphs.has(glyph)).join('');
    const name = NAMED_KEYS[key] ? t(NAMED_KEYS[key]) : key.toUpperCase();

    return modifiers + name;
}
