/**
 * Paints the page from the state. Every function here reads the state
 * and a layout and writes the DOM; nothing here decides anything.
 */
import {itemElement, sectionHeader} from './cards.js';
import {counts, emptyMessage, totalsText, footerAction} from './state.js';
import {TABS, otherTab} from './tabs.js';
import {FAVORITES} from './filter.js';

// The pieces of the page the renderer touches.
const elements = {
    launcher: document.getElementById('launcher'),
    tabs: Array.from(document.querySelectorAll('#tabs [role="tab"]')),
    search: document.getElementById('search'),
    categories: document.getElementById('categories'),
    status: document.getElementById('status'),
    content: document.getElementById('content'),
    grid: document.getElementById('grid'),
    empty: document.getElementById('empty'),
    totals: document.getElementById('totals'),
    action: document.getElementById('action'),
    hints: document.getElementById('hints'),
    toast: document.getElementById('toast'),
    help: document.getElementById('help'),
};

// How long the toast stays: enough to read one word.
const TOAST_MS = 800;

// The pending toast timer, so a second toast replaces the first.
let toastTimer = 0;

// The build and library facts for the help panel, from Go's About();
// null until the first answer arrives.
let about = null;

/**
 * Keeps what About() said, for the next paint of the help panel.
 * @param {{version: string, commit: string, builtAt: string, goVersion: string, libraryPath: string}} view
 */
export function setAbout(view) {
    about = view;
}

/**
 * The name of a chip: the library's, or the translation of a virtual one.
 * @param {{id: string, name: string, virtual: boolean}} category
 * @param {Function} t
 * @returns {string}
 */
function categoryName(category, t) {
    return category.virtual ? t(`category.${category.id}`) : category.name;
}

/**
 * The tab labels carry a number: the size of the tab, or, while searching,
 * how many items of that tab match the query. That is what makes the
 * search global: typing on one tab tells if the other one has what you
 * look for.
 * @param {object} state
 * @param {Function} t
 */
function renderTabs(state, t) {
    const totals = counts(state, state.query);

    elements.tabs.forEach((button) => {
        const tab = button.dataset.tab;
        button.setAttribute('aria-selected', String(tab === state.tab));
        button.querySelector('.label').textContent = t(TABS[tab].label);
        button.querySelector('.count').textContent = String(totals[tab]);
    });
}

/**
 * The category chips of the current tab: "All", the favourites, then one
 * per category. Hidden while searching: the unified list ignores chips.
 * @param {object} state
 * @param {Function} t
 * @param {{onChip: (id: string) => void}} handlers
 */
function renderCategories(state, t, handlers) {
    const categories = state.tab === 'links' ? state.linkCategories : state.appCategories;
    const chips = [
        {id: '', name: t('chip.all')},
        {id: FAVORITES, name: t('chip.favorites')},
        ...categories.map((category) => ({id: category.id, name: categoryName(category, t)})),
    ];

    elements.categories.replaceChildren();
    elements.categories.hidden = Boolean(state.query);

    chips.forEach((category, index) => {
        const chip = document.createElement('button');
        chip.type = 'button';
        chip.className = 'chip';
        chip.tabIndex = -1;
        chip.setAttribute('aria-pressed', String(category.id === state.category));
        chip.textContent = category.name;

        // ⌘1 is "All", ⌘2 the favourites, ⌘3 the first category, and so on.
        if (index < 9) {
            chip.title = `⌘${index + 1}`;
        }

        chip.addEventListener('click', () => handlers.onChip(category.id));
        elements.categories.appendChild(chip);
    });
}

/**
 * The broken-file notice: the line of a TOML that does not parse, or why
 * the file cannot be used; nothing when all is well.
 * @param {object} state
 * @param {Function} t
 */
function renderStatus(state, t) {
    const status = state.status;
    elements.status.hidden = !status.readOnly;

    if (!status.readOnly) {
        elements.status.textContent = '';
        return;
    }

    elements.status.textContent = status.line > 0
        ? t('status.broken', {line: status.line, error: status.error})
        : t('status.readOnly', {error: status.error});
}

/**
 * Rebuilds the list from the layout: section headers and items, the
 * selected one marked and scrolled into view, the search box pointing at
 * it for screen readers.
 * @param {object} state
 * @param {{entries: object[], groups: {title: string, start: number, count: number}[], unified: boolean}} layout
 * @param {Function} t
 * @param {{onOpen: (index: number) => void, onToggleFavorite: (entry: object) => void}} handlers
 */
function renderList(state, layout, t, handlers) {
    const {entries, groups, unified} = layout;
    const cards = TABS[state.tab].layout === 'cards' && !unified;

    elements.grid.className = cards ? 'cards' : 'rows';
    elements.grid.replaceChildren();

    const build = (entry, index) => itemElement(entry, {
        index,
        selected: index === state.selected,
        layout: TABS[state.tab].layout,
        unified,
        t,
        onOpen: () => handlers.onOpen(index),
        onToggleFavorite: () => handlers.onToggleFavorite(entry),
    });

    if (groups.length === 0) {
        entries.forEach((entry, index) => elements.grid.appendChild(build(entry, index)));
    }

    for (const group of groups) {
        elements.grid.appendChild(sectionHeader(group.title));

        // Cards flow in a grid; a header must take the whole row, so each
        // group gets its own sub-list.
        const holder = cards ? document.createElement('li') : elements.grid;
        if (cards) {
            holder.className = 'group';
            holder.role = 'presentation';
            elements.grid.appendChild(holder);
        }

        const list = cards ? document.createElement('ul') : holder;
        if (cards) {
            list.className = 'cards';
            list.role = 'presentation';
            holder.appendChild(list);
        }

        for (let index = group.start; index < group.start + group.count; index += 1) {
            list.appendChild(build(entries[index], index));
        }
    }

    elements.empty.hidden = entries.length > 0;
    if (entries.length === 0) {
        elements.empty.textContent = emptyMessage(state, t);
    }

    const current = elements.grid.querySelector(`[data-index="${state.selected}"]`);
    if (current) {
        current.scrollIntoView({block: 'nearest'});
        elements.search.setAttribute('aria-activedescendant', current.id);
    } else {
        elements.search.removeAttribute('aria-activedescendant');
    }

    // The fade at the bottom only when there is more below.
    elements.content.classList.toggle('scrollable', elements.content.scrollHeight > elements.content.clientHeight);
}

/**
 * The footer: totals, what Enter does, and the three hints that matter.
 * @param {object} state
 * @param {object} layout
 * @param {Function} t
 */
function renderFooter(state, layout, t) {
    elements.totals.textContent = totalsText(state, t);
    elements.action.textContent = footerAction(layout.entries[state.selected], t);
    elements.hints.textContent = t('footer.hints', {tab: t(TABS[otherTab(state.tab)].label)});
}

/**
 * Texts that do not depend on the list: the dialog label and the search
 * box, in the current language.
 * @param {Function} t
 */
function renderChrome(t) {
    elements.launcher.setAttribute('aria-label', t('dialog.label'));
    elements.search.placeholder = t('search.placeholder');
    elements.search.setAttribute('aria-label', t('search.label'));
    document.documentElement.lang = t.language;
}

/**
 * The help panel: every shortcut, and the two global ones from settings.
 * @param {object} state
 * @param {Function} t
 */
export function renderHelp(state, t) {
    elements.help.hidden = !state.helpOpen;
    elements.help.querySelector('h2').textContent = t('help.title');
    elements.help.querySelector('.close').textContent = t('help.close');

    const list = elements.help.querySelector('ul');
    list.replaceChildren();

    const lines = [
        t('help.navigate'),
        t('help.open'),
        t('help.copy'),
        t('help.edit'),
        t('help.escape'),
        t('help.global', {apps: state.settings.hotkeyApps, links: state.settings.hotkeyLinks}),
    ];

    for (const line of lines) {
        const item = document.createElement('li');
        item.textContent = line;
        list.appendChild(item);
    }

    // The build and the library file close the panel, quieter than the
    // shortcuts: they are for bug reports, not for daily use.
    if (about) {
        const facts = [
            t('help.version', {version: about.version, commit: about.commit || '—', date: about.builtAt || '—', go: about.goVersion}),
            t('help.library', {path: about.libraryPath}),
        ];

        for (const fact of facts) {
            const item = document.createElement('li');
            item.className = 'about';
            item.textContent = fact;
            list.appendChild(item);
        }
    }
}

/**
 * Shows a one-word confirmation for a moment.
 * @param {string} text
 */
export function showToast(text) {
    elements.toast.textContent = text;
    elements.toast.hidden = false;

    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => {
        elements.toast.hidden = true;
    }, TOAST_MS);
}

/**
 * Plays the appearance animation once per showing.
 */
export function animateAppearance() {
    elements.launcher.classList.remove('appear');
    // Forcing a reflow restarts the animation when it is already applied.
    void elements.launcher.offsetWidth;
    elements.launcher.classList.add('appear');
}

/**
 * Repaints the whole page.
 * @param {object} state
 * @param {object} layout
 * @param {Function} t
 * @param {{onChip: Function, onOpen: Function, onToggleFavorite: Function}} handlers
 */
export function renderAll(state, layout, t, handlers) {
    renderChrome(t);
    renderTabs(state, t);
    renderCategories(state, t, handlers);
    renderStatus(state, t);
    renderList(state, layout, t, handlers);
    renderFooter(state, layout, t);
    renderHelp(state, t);
}

/**
 * How many cards fit in one row of the grid, so ↑↓ move vertically on the
 * apps tab. Rows are a single column.
 * @returns {number}
 */
export function columns() {
    const list = elements.grid.querySelector('ul.cards') ?? elements.grid;
    if (!list.classList.contains('cards')) {
        return 1;
    }

    return getComputedStyle(list).gridTemplateColumns.split(' ').length;
}

/**
 * The card ArrowUp/ArrowDown should land on in the cards layout. Column
 * counting breaks down once the grid holds several sections (Tools,
 * Applications, Edge apps…): a short section can end mid-row, so moving by
 * a fixed number of positions can land in the wrong section entirely.
 * Comparing the actual boxes on screen instead finds the nearest row in
 * the given direction, then the option in it closest to the current
 * horizontal centre.
 * @param {1|-1} direction
 * @returns {number|null} the target's data-index, or null past either end
 */
export function verticalNeighbour(direction) {
    const current = elements.grid.querySelector('[aria-selected="true"]');
    if (!current) {
        return null;
    }

    const currentRect = current.getBoundingClientRect();
    const currentCentre = currentRect.left + currentRect.width / 2;

    // Half a card height is the tolerance for "same row": two cards laid
    // out side by side share a top exactly, but rounding across sections
    // can be off by a pixel or two.
    const rowTolerance = currentRect.height / 2;

    const options = Array.from(elements.grid.querySelectorAll('[role="option"]'))
        .filter((option) => option !== current)
        .map((option) => ({option, rect: option.getBoundingClientRect()}))
        .filter(({rect}) => direction > 0
            ? rect.top > currentRect.top + rowTolerance
            : rect.top < currentRect.top - rowTolerance);

    if (options.length === 0) {
        return null;
    }

    const nearestTop = options.reduce(
        (best, {rect}) => Math.abs(rect.top - currentRect.top) < Math.abs(best - currentRect.top) ? rect.top : best,
        options[0].rect.top,
    );

    const row = options.filter(({rect}) => Math.abs(rect.top - nearestTop) < rowTolerance);
    const nearest = row.reduce((best, candidate) => {
        const centre = candidate.rect.left + candidate.rect.width / 2;
        const distance = Math.abs(centre - currentCentre);
        return best === null || distance < best.distance ? {candidate, distance} : best;
    }, null);

    return Number(nearest.candidate.option.dataset.index);
}

/**
 * The search box, for the glue to focus and read.
 * @returns {HTMLInputElement}
 */
export function searchBox() {
    return elements.search;
}
