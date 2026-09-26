/**
 * Builds the list items of the page: a card for an app, a row for a link.
 * Both are `<li role="option">`; what differs is the layout, which the
 * stylesheet picks from the class.
 */

/**
 * Builds the icon of an item: the bundled image when there is one, or a
 * tile with the first letter of the name so a link without icon still has
 * a visual anchor.
 * @param {string|undefined} src
 * @param {string} name
 * @returns {HTMLElement}
 */
function iconElement(src, name) {
    if (src) {
        const image = document.createElement('img');
        image.src = src;
        image.alt = '';
        return image;
    }

    const tile = document.createElement('span');
    tile.className = 'tile';
    tile.textContent = name.trim().charAt(0).toUpperCase();
    return tile;
}

/**
 * Adds a text span with a class to an item.
 * @param {HTMLElement} item
 * @param {string} className
 * @param {string} text
 * @returns {HTMLElement}
 */
function addText(item, className, text) {
    const span = document.createElement('span');
    span.className = className;
    span.textContent = text;
    item.appendChild(span);
    return span;
}

/**
 * The favourite star of an item: a real button, so the mouse can mark and
 * unmark without opening the item. The click is stopped there; the item's
 * own click handler is what opens.
 * @param {{favorite: boolean, name: string}} entry
 * @param {() => void} onToggle
 * @returns {HTMLButtonElement}
 */
function starButton(entry, onToggle) {
    const star = document.createElement('button');
    star.type = 'button';
    star.className = 'star';
    star.setAttribute('aria-pressed', String(Boolean(entry.favorite)));
    star.setAttribute('aria-label', entry.favorite ? `Quitar ${entry.name} de favoritos` : `Añadir ${entry.name} a favoritos`);
    star.title = entry.favorite ? 'Quitar de favoritos (⌘F)' : 'Añadir a favoritos (⌘F)';
    star.textContent = entry.favorite ? '★' : '☆';

    star.addEventListener('click', (event) => {
        event.stopPropagation();
        onToggle();
    });

    return star;
}

/**
 * How many times an item was opened, as a small label; nothing at all for
 * an item never opened, so the list stays quiet until it earns a number.
 * @param {number} opens
 * @returns {HTMLElement|null}
 */
function opensLabel(opens) {
    if (!opens) {
        return null;
    }

    const label = document.createElement('span');
    label.className = 'opens';
    label.textContent = `×${opens}`;
    label.title = opens === 1 ? 'Abierto 1 vez' : `Abierto ${opens} veces`;
    return label;
}

/**
 * A card for an app. Apps that are not built or installed stay visible but
 * greyed out, so the list also tells what is missing.
 * @param {{id: string, name: string, description: string, path: string, favorite: boolean, opens: number}} app
 * @param {string|undefined} icon
 * @param {() => void} onToggleFavorite
 * @returns {HTMLLIElement}
 */
export function appCard(app, icon, onToggleFavorite) {
    const item = document.createElement('li');
    item.role = 'option';
    item.className = app.path ? 'app' : 'app missing';

    item.appendChild(starButton(app, onToggleFavorite));

    const count = opensLabel(app.opens);
    if (count) {
        item.appendChild(count);
    }

    item.appendChild(iconElement(icon, app.name));
    addText(item, 'name', app.name);
    addText(item, 'description', app.path ? app.description : 'No compilada');

    return item;
}

/**
 * A row for a link: icon, name and description, the host on the right so
 * the eye can tell two links with similar names apart, the opening count
 * and the star.
 * @param {{id: string, name: string, description: string, host: string, favorite: boolean, opens: number}} link
 * @param {string|undefined} icon
 * @param {() => void} onToggleFavorite
 * @returns {HTMLLIElement}
 */
export function linkRow(link, icon, onToggleFavorite) {
    const item = document.createElement('li');
    item.role = 'option';
    item.className = 'link';

    item.appendChild(iconElement(icon, link.name));

    const text = document.createElement('span');
    text.className = 'text';
    addText(text, 'name', link.name);
    addText(text, 'description', link.description);
    item.appendChild(text);

    const aside = document.createElement('span');
    aside.className = 'aside';

    const count = opensLabel(link.opens);
    if (count) {
        aside.appendChild(count);
    }

    addText(aside, 'host', link.host);
    aside.appendChild(starButton(link, onToggleFavorite));
    item.appendChild(aside);

    return item;
}
