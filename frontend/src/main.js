import './style.css';
import {EventsOn} from '../wailsjs/runtime/runtime';
import {Apps, AppCategories, Links, LinkCategories, Launch, OpenLink, Hide, TabChanged, Debug, Usage, ToggleFavorite} from '../wailsjs/go/main/App';
import {filterItems, decorate, sortByUse, FAVORITES} from './filter.js';
import {nextIndex} from './keys.js';
import {appCard, linkRow} from './cards.js';

// Icons are bundled by Vite; the key is the id from the Go catalogs.
const appIcons = import.meta.glob('./assets/icons/*.png', {eager: true, import: 'default'});
const linkIcons = import.meta.glob('./assets/links/*.png', {eager: true, import: 'default'});

// The pieces of the page the script touches.
const tabButtons = Array.from(document.querySelectorAll('#tabs [role="tab"]'));
const search = document.getElementById('search');
const categoriesNav = document.getElementById('categories');
const grid = document.getElementById('grid');
const empty = document.getElementById('empty');
const totals = document.getElementById('totals');

// Everything the user can change while the launcher is on screen. `shown`
// puts it back to the initial values, so the launcher never reappears with
// last time's search or filter.
const state = {
    tab: 'apps',
    category: '',
    query: '',
    selected: 0,
    apps: [],
    links: [],
    appCategories: [],
    linkCategories: [],
    usage: {opens: {}, favorites: []},
};

/**
 * The catalog and categories of a tab, so the rest of the code does not
 * branch on the tab name everywhere. Items come decorated with their usage
 * and sorted by it: the most opened first.
 * @param {string} tab
 * @returns {{items: object[], categories: object[]}}
 */
function catalogOf(tab) {
    const raw = tab === 'links' ? state.links : state.apps;
    const categories = tab === 'links' ? state.linkCategories : state.appCategories;

    return {items: sortByUse(decorate(raw, tab, state.usage)), categories};
}

/**
 * The items of the current tab that pass the category chip and the search.
 * @returns {object[]}
 */
function visibleItems() {
    return filterItems(catalogOf(state.tab).items, state.category, state.query);
}

/**
 * The tab labels carry a number: the size of the tab, or, while searching,
 * how many items of that tab match the query. That is what makes the search
 * global: typing on one tab tells if the other one has what you look for.
 */
function renderTabs() {
    tabButtons.forEach((button) => {
        const tab = button.dataset.tab;
        button.setAttribute('aria-selected', String(tab === state.tab));
        button.querySelector('.count').textContent = String(filterItems(catalogOf(tab).items, '', state.query).length);
    });
}

/**
 * The category chips of the current tab: "Todos" plus one per category,
 * each with how many items it holds for the current query.
 */
function renderCategories() {
    const {items, categories} = catalogOf(state.tab);
    const chips = [
        {id: '', name: state.tab === 'links' ? 'Todos' : 'Todas'},
        {id: FAVORITES, name: '★ Favoritos'},
        ...categories,
    ];

    categoriesNav.innerHTML = '';

    chips.forEach((category, index) => {
        const chip = document.createElement('button');
        chip.type = 'button';
        chip.className = 'chip';
        chip.setAttribute('aria-pressed', String(category.id === state.category));
        chip.textContent = category.name;

        const count = document.createElement('span');
        count.className = 'count';
        count.textContent = String(filterItems(items, category.id, state.query).length);
        chip.appendChild(count);

        // ⌘1 is "Todos", ⌘2 the first category, and so on.
        chip.title = `⌘${index + 1}`;

        chip.addEventListener('click', () => {
            selectCategory(category.id);
        });

        categoriesNav.appendChild(chip);
    });
}

/**
 * Rebuilds the list from the visible items and keeps the selection inside
 * it. The apps tab is a grid of cards; the links tab a list of rows.
 */
function renderItems() {
    const items = visibleItems();

    if (state.selected >= items.length) {
        state.selected = Math.max(0, items.length - 1);
    }

    grid.className = state.tab === 'links' ? 'rows' : 'cards';
    grid.innerHTML = '';

    items.forEach((entry, index) => {
        const toggle = () => toggleFavorite(entry);
        const item = state.tab === 'links'
            ? linkRow(entry, linkIcons[`./assets/links/${entry.id}.png`], toggle)
            : appCard(entry, appIcons[`./assets/icons/${entry.id}.png`] ?? entry.icon, toggle);

        item.dataset.index = String(index);
        item.setAttribute('aria-selected', String(index === state.selected));

        item.addEventListener('click', () => {
            state.selected = index;
            open();
        });

        grid.appendChild(item);
    });

    empty.hidden = items.length > 0;
    if (items.length === 0) {
        empty.textContent = emptyMessage();
    }

    const current = grid.children[state.selected];
    if (current) {
        current.scrollIntoView({block: 'nearest'});
    }
}

/**
 * What to say when nothing matches: point at the other tab when the same
 * query has results there.
 * @returns {string}
 */
function emptyMessage() {
    const other = state.tab === 'links' ? 'apps' : 'links';
    const elsewhere = filterItems(catalogOf(other).items, '', state.query).length;

    if (elsewhere > 0) {
        const label = other === 'links' ? 'Mis links' : 'Mis apps';
        return `Nada aquí · ${elsewhere} en ${label} (⇥)`;
    }

    return 'Nada que coincida';
}

/**
 * The footer always shows the size of both catalogs, whatever the filter.
 */
function renderTotals() {
    const apps = state.apps.length === 1 ? '1 app' : `${state.apps.length} apps`;
    const links = state.links.length === 1 ? '1 link' : `${state.links.length} links`;
    totals.textContent = `${apps} · ${links}`;
}

/**
 * Repaints the whole page from the state.
 */
function render() {
    renderTabs();
    renderCategories();
    renderItems();
    renderTotals();
}

/**
 * Switches tab, resets the filter of the previous one and tells Go, so the
 * shortcuts know which tab is on screen.
 * @param {string} tab
 */
function selectTab(tab) {
    if (tab === state.tab) {
        return;
    }

    state.tab = tab;
    state.category = '';
    state.selected = 0;
    TabChanged(tab);
    render();
    search.focus();
}

/**
 * Applies a category chip and starts the selection from the top.
 * @param {string} category
 */
function selectCategory(category) {
    state.category = category;
    state.selected = 0;
    render();
    search.focus();
}

/**
 * How many cards fit in one row of the grid, so ↑↓ move vertically on the
 * apps tab. The links tab is a single column.
 * @returns {number}
 */
function columns() {
    if (state.tab === 'links') {
        return 1;
    }

    return getComputedStyle(grid).gridTemplateColumns.split(' ').length;
}

/**
 * Moves the selection by a number of positions, wrapping around.
 * @param {number} delta
 */
function move(delta) {
    const count = visibleItems().length;
    if (count === 0) {
        return;
    }

    state.selected = nextIndex(state.selected, delta, count);
    renderItems();
}

/**
 * Opens the selected item: an app through Launch, a link through OpenLink.
 * Go hides the launcher on success; a failure (for example an app that is
 * not built) is shown on the item itself.
 */
async function open() {
    const entry = visibleItems()[state.selected];
    if (!entry) {
        return;
    }

    try {
        if (state.tab === 'links') {
            await OpenLink(entry.id);
        } else {
            await Launch(entry.id);
        }
    } catch (error) {
        const item = grid.children[state.selected];
        item.querySelector('.description').textContent = String(error);
    }
}

/**
 * Marks or unmarks an item as favourite through Go, then mirrors the answer
 * in the local usage so the star and the favourites chip update at once.
 * @param {{key: string}} entry
 */
async function toggleFavorite(entry) {
    try {
        const on = await ToggleFavorite(entry.key);
        const others = state.usage.favorites.filter((key) => key !== entry.key);
        state.usage.favorites = on ? [...others, entry.key] : others;
    } catch (error) {
        Debug(`favorite ${entry.key}: ${error}`);
    }

    render();
    search.focus();
}

/**
 * Asks Go for both catalogs and repaints. Called on every appearance, so an
 * app built in the meantime shows up without restarting the launcher.
 */
async function refresh() {
    [state.apps, state.appCategories, state.links, state.linkCategories, state.usage] = await Promise.all([
        Apps(),
        AppCategories(),
        Links(),
        LinkCategories(),
        Usage(),
    ]);

    // Belt and braces: the page never trusts the shape of the usage file.
    state.usage.opens ??= {};
    state.usage.favorites ??= [];

    render();
}

/**
 * Handles a key press. The search box keeps the focus, so plain typing
 * always goes to it; the keys below are taken before the box sees them.
 * @param {KeyboardEvent} event
 */
function onKeydown(event) {
    if (event.metaKey && event.key === 'f') {
        event.preventDefault();
        const entry = visibleItems()[state.selected];
        if (entry) {
            toggleFavorite(entry);
        }
        return;
    }

    if (event.metaKey && /^[1-9]$/.test(event.key)) {
        const chip = categoriesNav.children[Number(event.key) - 1];
        if (chip) {
            event.preventDefault();
            chip.click();
        }
        return;
    }

    switch (event.key) {
        case 'ArrowRight':
            event.preventDefault();
            move(1);
            break;
        case 'ArrowLeft':
            event.preventDefault();
            move(-1);
            break;
        case 'ArrowDown':
            event.preventDefault();
            move(columns());
            break;
        case 'ArrowUp':
            event.preventDefault();
            move(-columns());
            break;
        case 'Tab':
            event.preventDefault();
            selectTab(state.tab === 'apps' ? 'links' : 'apps');
            break;
        case 'Enter':
            event.preventDefault();
            open();
            break;
        case 'Escape':
            event.preventDefault();
            Hide();
            break;
    }
}

document.addEventListener('keydown', onKeydown);

search.addEventListener('input', () => {
    state.query = search.value;
    state.selected = 0;
    render();
});

tabButtons.forEach((button) => {
    button.addEventListener('click', () => {
        selectTab(button.dataset.tab);
    });
});

// Like Cmd+Tab, the overlay goes away as soon as something else takes focus.
// The check is delayed a little because the first appearance after launch
// fires a blur while the window is being activated; if the focus is back by
// then, the launcher stays.
window.addEventListener('blur', () => {
    setTimeout(() => {
        if (document.hasFocus()) {
            return;
        }

        Debug('window blur');
        Hide();
    }, 150);
});

// Every time a shortcut shows the window, the page starts clean on the tab
// of that shortcut, with the search box ready to type into.
EventsOn('shown', (tab) => {
    Debug(`shown ${JSON.stringify(tab)}`);
    state.tab = tab === 'links' ? 'links' : 'apps';
    state.category = '';
    state.query = '';
    state.selected = 0;
    search.value = '';
    refresh().then(() => search.focus());
});

refresh();
