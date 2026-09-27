/**
 * The two languages of the page. Keys are dotted; `{name}` placeholders
 * take parameters; plurals are `key.one` and `key.other`.
 */

// Every text of the page, in both languages. Keep the two objects with
// the same keys: a test checks it.
export const MESSAGES = {
    en: {
        'dialog.label': 'Apps and links',
        'tab.apps': 'Apps',
        'tab.links': 'Links',
        'search.placeholder': 'Search…',
        'search.label': 'Search',
        'chip.all': 'All',
        'chip.favorites': '★ Favourites',
        'category.applications': 'Applications',
        'category.edge': 'Edge apps',
        'section.favorites': 'Favourites',
        'section.recent': 'Recent',
        'empty.none': 'Nothing matches',
        'empty.elsewhere': 'Nothing here · {count} in {tab} (⇥)',
        'empty.links': 'No links yet',
        'empty.apps': 'No apps in /Applications',
        'apps.one': '1 app',
        'apps.other': '{count} apps',
        'links.one': '1 link',
        'links.other': '{count} links',
        'footer.open': '↩ Open {name}',
        'footer.pair': '↩ app · ⌘↩ web',
        'footer.hints': '⌘F ★ · ⇥ {tab} · ? help',
        'toast.copied': 'Copied',
        'toast.noBrowser': 'No secondary_browser in library.toml',
        'toast.openFailed': 'Could not open: {error}',
        'toast.tooMany': 'Too many items to open at once',
        'status.broken': 'library.toml, line {line}: {error}',
        'status.readOnly': 'library.toml cannot be used: {error}',
        'app.missing': 'Not found',
        'app.hidden': 'Hidden',
        'kind.app': 'app',
        'kind.link': 'link',
        'star.add': 'Add {name} to favourites',
        'star.remove': 'Remove {name} from favourites',
        'star.title': 'Favourite (⌘F)',
        'opens.one': 'Opened once',
        'opens.other': 'Opened {count} times',
        'help.title': 'Shortcuts',
        'help.close': 'Esc closes',
        'help.version': 'hopto {version} · {commit} · built {date} · {go}',
        'help.library': 'Library: {path}',
        'help.navigate': '↑ ↓ ← → move · ↩ open · ⇥ other tab',
        'help.open': '⌘↩ secondary browser (links) or web (Edge app) · ⌥↩ reveal in Finder · ⌘⇧↩ open every item of the chip',
        'help.copy': '⌘C copy URL or path · ⌘F favourite · ⌘1-9 chip',
        'help.edit': '⌘N new · ⌘E edit · ⌘⌫ delete (next release)',
        'help.escape': 'Esc clears the search, then hides · ? or ⌘/ this help',
        'help.global': 'Global: {apps} apps · {links} links',
    },
    es: {
        'dialog.label': 'Apps y links',
        'tab.apps': 'Mis apps',
        'tab.links': 'Mis links',
        'search.placeholder': 'Buscar…',
        'search.label': 'Buscar',
        'chip.all': 'Todo',
        'chip.favorites': '★ Favoritos',
        'category.applications': 'Aplicaciones',
        'category.edge': 'Apps de Edge',
        'section.favorites': 'Favoritos',
        'section.recent': 'Recientes',
        'empty.none': 'Nada que coincida',
        'empty.elsewhere': 'Nada aquí · {count} en {tab} (⇥)',
        'empty.links': 'Aún no hay links',
        'empty.apps': 'No hay apps en /Applications',
        'apps.one': '1 app',
        'apps.other': '{count} apps',
        'links.one': '1 link',
        'links.other': '{count} links',
        'footer.open': '↩ Abrir {name}',
        'footer.pair': '↩ app · ⌘↩ web',
        'footer.hints': '⌘F ★ · ⇥ {tab} · ? ayuda',
        'toast.copied': 'Copiado',
        'toast.noBrowser': 'No hay secondary_browser en library.toml',
        'toast.openFailed': 'No se pudo abrir: {error}',
        'toast.tooMany': 'Demasiados elementos para abrir a la vez',
        'status.broken': 'library.toml, línea {line}: {error}',
        'status.readOnly': 'library.toml no se puede usar: {error}',
        'app.missing': 'No encontrada',
        'app.hidden': 'Oculta',
        'kind.app': 'app',
        'kind.link': 'link',
        'star.add': 'Añadir {name} a favoritos',
        'star.remove': 'Quitar {name} de favoritos',
        'star.title': 'Favorito (⌘F)',
        'opens.one': 'Abierto 1 vez',
        'opens.other': 'Abierto {count} veces',
        'help.title': 'Atajos',
        'help.close': 'Esc cierra',
        'help.version': 'hopto {version} · {commit} · compilado el {date} · {go}',
        'help.library': 'Biblioteca: {path}',
        'help.navigate': '↑ ↓ ← → mover · ↩ abrir · ⇥ otra pestaña',
        'help.open': '⌘↩ navegador secundario (links) o web (app de Edge) · ⌥↩ mostrar en el Finder · ⌘⇧↩ abrir todo el chip',
        'help.copy': '⌘C copiar URL o ruta · ⌘F favorito · ⌘1-9 chip',
        'help.edit': '⌘N nuevo · ⌘E editar · ⌘⌫ borrar (próxima versión)',
        'help.escape': 'Esc vacía la búsqueda, luego esconde · ? o ⌘/ esta ayuda',
        'help.global': 'Globales: {apps} apps · {links} links',
    },
};

// The language used when nothing else applies.
const FALLBACK = 'en';

/**
 * Picks the page language: an explicit setting wins, "auto" follows the
 * browser (which follows macOS), and only es and en exist.
 * @param {string} setting
 * @param {string|undefined} navigatorLanguage
 * @returns {string}
 */
export function resolveLanguage(setting, navigatorLanguage) {
    if (setting === 'es' || setting === 'en') {
        return setting;
    }

    if (setting !== 'auto' && setting !== undefined && setting !== '') {
        return FALLBACK;
    }

    return String(navigatorLanguage ?? '').toLowerCase().startsWith('es') ? 'es' : FALLBACK;
}

/**
 * Builds the translate function of a language. A missing key comes back
 * as the key itself, which is visible in tests and harmless on screen.
 * @param {string} language
 * @returns {((key: string, params?: Object<string, string|number>) => string) & {plural: (key: string, count: number) => string, language: string}}
 */
export function translator(language) {
    const messages = MESSAGES[language] ?? MESSAGES[FALLBACK];

    const t = (key, params = {}) => {
        const template = messages[key] ?? MESSAGES[FALLBACK][key] ?? key;
        return template.replace(/\{(\w+)\}/g, (match, name) => (name in params ? String(params[name]) : match));
    };

    t.plural = (key, count) => t(count === 1 ? `${key}.one` : `${key}.other`, {count});
    t.language = language;

    return t;
}
