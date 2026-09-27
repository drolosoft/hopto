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
        'empty.links': 'No links yet · ⌘N adds one',
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

        // The broken-file button, the welcome and the key names.
        'status.edit': 'Open library.toml',
        'welcome.label': 'Welcome',
        'welcome.title': 'hopto is ready',
        'welcome.problem': 'A shortcut needs a look',
        'welcome.open': 'Press {apps} to open it · ⌘N adds your first link',
        'welcome.links': '{links} opens it on your links',
        'welcome.hotkey.registered': '{spec} · {tab}: ready',
        'welcome.hotkey.taken': '{spec} · {tab}: another app already uses it',
        'welcome.hotkey.failed': '{spec} · {tab}: macOS refused it (error {status})',
        'welcome.hotkey.pending': '{spec} · {tab}: registering…',
        'welcome.finder': 'macOS keeps {spec} for the Finder search window. Turn off “Show Finder search window” in Keyboard Shortcuts → Spotlight and hopto gets it.',
        'welcome.settings': 'Open Keyboard Settings',
        'welcome.start': 'Start ↩',
        'key.space': 'Space',
        'key.return': 'Return',
        'key.tab': 'Tab',
        'key.escape': 'Esc',
        'app.missing': 'Not found',
        'app.hidden': 'Hidden',

        // Editing, deleting and hiding from the list.
        'category.hidden': 'Hidden',
        'confirm.delete': 'Delete {name}? ↩ yes · Esc no',
        'confirm.hide': 'Hide {name}? ↩ yes · Esc no',
        'confirm.deleteCategory': 'Delete the category {name}? ↩ yes · Esc no',
        'toast.deleted': 'Deleted',
        'toast.hidden': 'Hidden',
        'toast.unhidden': 'Back in the list',
        'toast.renamed': 'Renamed',
        'toast.categoryInUse': 'The category still has items',
        'toast.deleteFailed': 'Could not delete: {error}',
        'toast.renameFailed': 'Could not rename: {error}',
        'editor.editDuplicate': '⌘E edits it',
        'rename.label': 'Category name',
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
        'help.edit': '⌘N new · ⌘E edit · ⌘⌫ delete or hide · ⌘⇧E ⌘⇧⌫ rename or delete the chip',
        'help.escape': 'Esc clears the search, then hides · ? or ⌘/ this help',
        'help.global': 'Global: {apps} apps · {links} links',

        // The add row and the editor.
        'add.row': '＋ Add “{text}”',
        'add.asURL': 'New link to this address',
        'add.asName': 'New link with this name',
        'footer.add': '↩ Add',
        'editor.label': 'Link editor',
        'editor.title.add': 'New link',
        'editor.title.edit': 'Edit link',
        'editor.url': 'URL',
        'editor.name': 'Name',
        'editor.description': 'Description',
        'editor.category': 'Category',
        'editor.newCategory': 'New category…',
        'editor.newCategoryName': 'Category name',
        'editor.hints': '↩ Save · Esc Cancel · ⌘1-9 Category',
        'editor.inspecting': 'Reading the page…',
        'editor.duplicate': 'You already have it: {name}',
        'editor.sameHost': 'Same site: {names}',
        'editor.insecure': 'Plain http: the connection is not encrypted',
        'toast.added': 'Added',
        'toast.saved': 'Saved',
        'toast.saveFailed': 'Could not save: {error}',
        'problem.url.invalid': 'Type a web address, like example.org',
        'problem.name.required': 'The name is required',
        'problem.name.long': 'At most 80 characters',
        'problem.name.control': 'The name has invisible characters',
        'problem.description.long': 'At most 200 characters',
        'problem.description.control': 'The description has invisible characters',
        'problem.category.unknown': 'Pick a category',
        'problem.category.name': 'Type the name of the new category',
        'problem.other': 'Check this field',

        // The apps editor and the native dialog.
        'pick.row': 'Search Applications…',
        'pick.hint': 'Pick a .app to add it by hand',
        'footer.pick': '↩ Choose',
        'editor.labelApp': 'App editor',
        'editor.title.addApp': 'New app',
        'editor.title.editApp': 'Edit app',
        'editor.path': 'App',
        'editor.pick': 'Choose .app… ⌘O',
        'editor.hintsApp': '↩ Save · Esc Cancel · ⌘O Choose .app · ⌘1-9 Category',
        'editor.twin': 'Found on disk as {name}: once added it shows once, under the category you pick',
        'toast.pickFailed': 'Could not open the dialog: {error}',
        'problem.app.target': 'Choose the .app',
        'problem.app.path': 'Only apps in /Applications, /System/Applications or ~/Applications',
        'problem.app.bundle': 'The bundle id is not valid',
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
        'empty.links': 'Aún no hay links · ⌘N para añadir uno',
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

        // The broken-file button, the welcome and the key names.
        'status.edit': 'Abrir library.toml',
        'welcome.label': 'Bienvenida',
        'welcome.title': 'hopto está listo',
        'welcome.problem': 'Hay un atajo que revisar',
        'welcome.open': 'Pulsa {apps} para abrirlo · ⌘N añade tu primer link',
        'welcome.links': '{links} lo abre en tus links',
        'welcome.hotkey.registered': '{spec} · {tab}: listo',
        'welcome.hotkey.taken': '{spec} · {tab}: otra app ya lo usa',
        'welcome.hotkey.failed': '{spec} · {tab}: macOS lo rechazó (error {status})',
        'welcome.hotkey.pending': '{spec} · {tab}: registrando…',
        'welcome.finder': 'macOS guarda {spec} para la ventana de búsqueda del Finder. Desactiva «Mostrar ventana de búsqueda del Finder» en Funciones rápidas de teclado → Spotlight y le llegará a hopto.',
        'welcome.settings': 'Abrir Ajustes de Teclado',
        'welcome.start': 'Empezar ↩',
        'key.space': 'Espacio',
        'key.return': 'Retorno',
        'key.tab': 'Tab',
        'key.escape': 'Esc',
        'app.missing': 'No encontrada',
        'app.hidden': 'Oculta',

        // Editing, deleting and hiding from the list.
        'category.hidden': 'Ocultas',
        'confirm.delete': '¿Borrar {name}? ↩ sí · Esc no',
        'confirm.hide': '¿Ocultar {name}? ↩ sí · Esc no',
        'confirm.deleteCategory': '¿Borrar la categoría {name}? ↩ sí · Esc no',
        'toast.deleted': 'Borrado',
        'toast.hidden': 'Oculta',
        'toast.unhidden': 'De vuelta en la lista',
        'toast.renamed': 'Renombrada',
        'toast.categoryInUse': 'La categoría aún tiene elementos',
        'toast.deleteFailed': 'No se pudo borrar: {error}',
        'toast.renameFailed': 'No se pudo renombrar: {error}',
        'editor.editDuplicate': '⌘E lo edita',
        'rename.label': 'Nombre de la categoría',
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
        'help.edit': '⌘N nuevo · ⌘E editar · ⌘⌫ borrar u ocultar · ⌘⇧E ⌘⇧⌫ renombrar o borrar el chip',
        'help.escape': 'Esc vacía la búsqueda, luego esconde · ? o ⌘/ esta ayuda',
        'help.global': 'Globales: {apps} apps · {links} links',

        // The add row and the editor.
        'add.row': '＋ Añadir «{text}»',
        'add.asURL': 'Link nuevo con esta dirección',
        'add.asName': 'Link nuevo con este nombre',
        'footer.add': '↩ Añadir',
        'editor.label': 'Editor de links',
        'editor.title.add': 'Link nuevo',
        'editor.title.edit': 'Editar link',
        'editor.url': 'URL',
        'editor.name': 'Nombre',
        'editor.description': 'Descripción',
        'editor.category': 'Categoría',
        'editor.newCategory': 'Nueva categoría…',
        'editor.newCategoryName': 'Nombre de la categoría',
        'editor.hints': '↩ Guardar · Esc Cancelar · ⌘1-9 Categoría',
        'editor.inspecting': 'Leyendo la página…',
        'editor.duplicate': 'Ya lo tienes: {name}',
        'editor.sameHost': 'Mismo sitio: {names}',
        'editor.insecure': 'http sin cifrar: la conexión no va protegida',
        'toast.added': 'Añadido',
        'toast.saved': 'Guardado',
        'toast.saveFailed': 'No se pudo guardar: {error}',
        'problem.url.invalid': 'Escribe una dirección web, como example.org',
        'problem.name.required': 'Falta el nombre',
        'problem.name.long': 'Como mucho 80 caracteres',
        'problem.name.control': 'El nombre tiene caracteres invisibles',
        'problem.description.long': 'Como mucho 200 caracteres',
        'problem.description.control': 'La descripción tiene caracteres invisibles',
        'problem.category.unknown': 'Elige una categoría',
        'problem.category.name': 'Escribe el nombre de la categoría nueva',
        'problem.other': 'Revisa este campo',

        // The apps editor and the native dialog.
        'pick.row': 'Buscar en Aplicaciones…',
        'pick.hint': 'Elige una .app para añadirla a mano',
        'footer.pick': '↩ Elegir',
        'editor.labelApp': 'Editor de apps',
        'editor.title.addApp': 'App nueva',
        'editor.title.editApp': 'Editar app',
        'editor.path': 'App',
        'editor.pick': 'Elegir .app… ⌘O',
        'editor.hintsApp': '↩ Guardar · Esc Cancelar · ⌘O Elegir .app · ⌘1-9 Categoría',
        'editor.twin': 'Encontrada en el disco como {name}: al añadirla sale una vez, en la categoría que elijas',
        'toast.pickFailed': 'No se pudo abrir el diálogo: {error}',
        'problem.app.target': 'Elige la .app',
        'problem.app.path': 'Solo apps de /Applications, /System/Applications o ~/Applications',
        'problem.app.bundle': 'El bundle id no es válido',
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
