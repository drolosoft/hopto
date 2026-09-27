/**
 * Builds the list items of the page: a card for an app on its own tab, a
 * row for a link or for any result of the unified search. Both are
 * `<li role="option">`; what differs is the layout, which the stylesheet
 * picks from the class. Everything is built with createElement: no HTML
 * strings, so a name is only ever text.
 */

/**
 * The icon of an item: the image served by Go, or a tile with the first
 * letter of the name so an item without icon still has a visual anchor.
 * @param {string} iconUrl
 * @param {string} name
 * @returns {HTMLElement}
 */
function iconElement(iconUrl, name) {
    const tile = document.createElement('span');
    tile.className = 'tile';
    const initial = name.trim().charAt(0).toUpperCase();

    if (iconUrl) {
        const image = document.createElement('img');
        image.src = iconUrl;
        image.alt = '';
        image.loading = 'lazy';

        // The file behind iconUrl can go stale (a moved bundle, a fetch
        // that never landed): once the browser gives up loading it, the
        // tile falls back to the initial instead of a broken-image icon.
        image.addEventListener('error', () => {
            image.remove();
            tile.textContent = initial;
        }, {once: true});

        tile.appendChild(image);
        return tile;
    }

    tile.textContent = initial;
    return tile;
}

/**
 * Adds a text span with a class to a parent.
 * @param {HTMLElement} parent
 * @param {string} className
 * @param {string} text
 * @returns {HTMLElement}
 */
function addText(parent, className, text) {
    const span = document.createElement('span');
    span.className = className;
    span.textContent = text;
    parent.appendChild(span);
    return span;
}

/**
 * The favourite star: a real button, so the mouse can mark and unmark
 * without opening the item. The click is stopped there; the item's own
 * click handler is what opens.
 * @param {{favorite: boolean, name: string}} entry
 * @param {Function} t
 * @param {() => void} onToggle
 * @returns {HTMLButtonElement}
 */
function starButton(entry, t, onToggle) {
    const star = document.createElement('button');
    star.type = 'button';
    star.className = 'star';
    star.tabIndex = -1;
    star.setAttribute('aria-pressed', String(Boolean(entry.favorite)));
    star.setAttribute('aria-label', t(entry.favorite ? 'star.remove' : 'star.add', {name: entry.name}));
    star.title = t('star.title');
    star.textContent = entry.favorite ? '★' : '☆';

    star.addEventListener('click', (event) => {
        event.stopPropagation();
        onToggle();
    });

    return star;
}

/**
 * The pencil: the mouse's way into the editor, since ⌘E is only written
 * in the help. Like the star it is a real button whose click stops at
 * the button, so it never opens the item. The stylesheet keeps it out
 * of sight until the pointer or the selection is on the item.
 * @param {{name: string}} entry
 * @param {Function} t
 * @param {() => void} onEdit
 * @returns {HTMLButtonElement}
 */
function editButton(entry, t, onEdit) {
    const pencil = document.createElement('button');
    pencil.type = 'button';
    pencil.className = 'edit';
    pencil.tabIndex = -1;
    pencil.setAttribute('aria-label', t('edit.label', {name: entry.name}));
    pencil.title = t('edit.title');
    pencil.textContent = '✎';

    pencil.addEventListener('click', (event) => {
        event.stopPropagation();
        onEdit();
    });

    return pencil;
}

/**
 * How many times an item was opened, as a small label; nothing at all for
 * an item never opened, so the list stays quiet until it earns a number.
 * @param {number} opens
 * @param {Function} t
 * @returns {HTMLElement|null}
 */
function opensLabel(opens, t) {
    if (!opens) {
        return null;
    }

    const label = document.createElement('span');
    label.className = 'opens';
    label.textContent = `×${opens}`;
    label.title = t.plural('opens', opens);
    return label;
}

/**
 * The second line of an item: what is wrong with it when something is,
 * else its description.
 * @param {object} entry
 * @param {Function} t
 * @returns {string}
 */
function secondLine(entry, t) {
    if (entry.missing) {
        return t('app.missing');
    }

    if (entry.hidden) {
        return t('app.hidden');
    }

    return entry.description || entry.host || '';
}

/**
 * A card: the apps tab when nothing is typed.
 * @param {object} entry
 * @param {Function} t
 * @param {() => void} onToggleFavorite
 * @param {() => void} onEdit
 * @returns {HTMLLIElement}
 */
function card(entry, t, onToggleFavorite, onEdit) {
    const item = document.createElement('li');
    item.className = 'card';

    item.appendChild(editButton(entry, t, onEdit));
    item.appendChild(starButton(entry, t, onToggleFavorite));

    const count = opensLabel(entry.opens, t);
    if (count) {
        item.appendChild(count);
    }

    item.appendChild(iconElement(entry.iconUrl, entry.name));
    addText(item, 'name', entry.name);
    addText(item, 'description', secondLine(entry, t));

    return item;
}

/**
 * A row: the links tab, and every result while searching. While
 * searching a discreet badge says whether the row is an app or a link;
 * an Edge app paired with its link shows both.
 * @param {object} entry
 * @param {Function} t
 * @param {boolean} unified
 * @param {() => void} onToggleFavorite
 * @param {() => void} onEdit
 * @returns {HTMLLIElement}
 */
function row(entry, t, unified, onToggleFavorite, onEdit) {
    const item = document.createElement('li');
    item.className = 'row';

    item.appendChild(iconElement(entry.iconUrl, entry.name));

    const text = document.createElement('span');
    text.className = 'text';
    addText(text, 'name', entry.name);
    addText(text, 'description', secondLine(entry, t));
    item.appendChild(text);

    const aside = document.createElement('span');
    aside.className = 'aside';

    if (unified) {
        addText(aside, 'kind', entry.web ? `${t('kind.app')} · ${t('kind.link')}` : t(`kind.${entry.kind}`));
    }

    const count = opensLabel(entry.opens, t);
    if (count) {
        aside.appendChild(count);
    }

    if (entry.host && entry.kind === 'link') {
        addText(aside, 'host', entry.host);
    }

    aside.appendChild(editButton(entry, t, onEdit));
    aside.appendChild(starButton(entry, t, onToggleFavorite));
    item.appendChild(aside);

    return item;
}

/**
 * A row that does something instead of opening an item: "＋ Add" today,
 * "Search Applications…" on the apps tab. It carries its glyph in the
 * tile and no star, since there is nothing to mark as a favourite.
 * @param {{name: string, description: string, glyph: string}} entry
 * @returns {HTMLLIElement}
 */
function actionRow(entry) {
    const item = document.createElement('li');
    item.className = 'row action';

    const tile = document.createElement('span');
    tile.className = 'tile';
    tile.textContent = entry.glyph;
    item.appendChild(tile);

    const text = document.createElement('span');
    text.className = 'text';
    addText(text, 'name', entry.name);
    addText(text, 'description', entry.description);
    item.appendChild(text);

    return item;
}

/**
 * Builds the element of one entry and stamps the attributes the list and
 * the keyboard rely on: the index in the flat list, a DOM id for
 * aria-activedescendant, the selected state.
 * @param {object} entry
 * @param {{index: number, selected: boolean, layout: string, unified: boolean, t: Function, question: string, onOpen: () => void, onToggleFavorite: () => void, onEdit: () => void}} options
 * @returns {HTMLLIElement}
 */
export function itemElement(entry, options) {
    const {index, selected, layout, unified, t, question, onOpen, onToggleFavorite, onEdit} = options;

    let item;
    if (entry.kind === 'add' || entry.kind === 'pick') {
        item = actionRow(entry);
    } else if (layout === 'cards' && !unified) {
        item = card(entry, t, onToggleFavorite, onEdit);
    } else {
        item = row(entry, t, unified, onToggleFavorite, onEdit);
    }

    item.role = 'option';
    item.id = `item-${entry.key.replace(':', '-')}`;
    item.dataset.index = String(index);
    item.setAttribute('aria-selected', String(selected));

    if (entry.missing || entry.hidden) {
        item.classList.add('dim');
    }

    // A pending "delete? ↩ yes · Esc no" takes the place of the second
    // line, so the question sits on the very row it is about.
    if (question) {
        item.classList.add('confirming');
        const line = item.querySelector('.description');
        if (line) {
            line.textContent = question;
        }
    }

    item.addEventListener('click', onOpen);

    return item;
}

/**
 * The heading of a section (Favourites, Recent, a category) inside the
 * list; not an option, so the keyboard skips it.
 * @param {string} title
 * @returns {HTMLLIElement}
 */
export function sectionHeader(title) {
    const header = document.createElement('li');
    header.className = 'section';
    header.role = 'presentation';
    header.textContent = title;
    return header;
}
