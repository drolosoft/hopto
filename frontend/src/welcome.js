/**
 * The welcome: over the panel on the first run, or when a global
 * shortcut cannot work. It is modal and owns the keyboard while open
 * (Enter and Esc close it, Tab walks its buttons), through a listener in
 * the capture phase, so keys.js never needs to know it exists.
 */
import {Welcome, PresentWelcome, DismissWelcome, OpenKeyboardSettings, Debug} from '../wailsjs/go/app/App';
import {prettyHotkey} from './hotkeys.js';
import {TABS} from './tabs.js';

// The section the welcome is built into.
const panel = document.getElementById('welcome');

// The Finder's own shortcut, named in the warning.
const FINDER_SPEC = 'cmd+option+space';

// What the page gives the welcome: the current translator, and what to
// do once the user closed it.
let host = null;

// Whether the welcome is on screen and holding the keyboard.
let open = false;

/**
 * Connects the welcome to the page and starts listening for its keys.
 * @param {{t: () => Function, onClosed: () => void}} pageHost
 */
export function installWelcome(pageHost) {
    host = pageHost;

    // Capture on the document runs before the page's own key handler,
    // which listens in the bubbling phase.
    document.addEventListener('keydown', takeKey, {capture: true});
}

/**
 * Whether the welcome is showing.
 * @returns {boolean}
 */
export function isWelcomeOpen() {
    return open;
}

/**
 * The keys while the welcome is open: Tab moves between its buttons,
 * Enter presses the focused one (Start unless Settings has the focus),
 * Esc is Start; nothing reaches the search box or the list.
 * @param {KeyboardEvent} event
 */
function takeKey(event) {
    if (!open) {
        return;
    }

    event.stopImmediatePropagation();

    if (event.key === 'Tab') {
        return;
    }

    event.preventDefault();

    if (event.isComposing) {
        return;
    }

    if (event.key === 'Enter' && document.activeElement?.id === 'welcome-settings') {
        openSettings();
        return;
    }

    if (event.key === 'Enter' || event.key === 'Escape') {
        dismiss();
    }
}

/**
 * Asks Go whether the welcome is due and shows it if so, with the focus
 * on Start. Called after every `shown`.
 */
export async function checkWelcome() {
    let view = null;
    try {
        view = await Welcome();
    } catch (error) {
        Debug(`welcome: ${error}`);
    }

    if (!view?.show) {
        return;
    }

    paint(view);
    open = true;
    panel.hidden = false;
    document.getElementById('welcome-start').focus();
}

/**
 * Takes the welcome down without telling Go (a `shown` does this; the
 * next check puts it back while it is still due).
 */
export function hideWelcome() {
    open = false;
    panel.hidden = true;
    panel.replaceChildren();
}

/**
 * Asks Go to show the panel for the welcome, once the page has loaded.
 */
export function presentWelcome() {
    PresentWelcome().catch((error) => Debug(`present welcome: ${error}`));
}

/**
 * Start (or Esc): the welcome goes for the rest of this run.
 */
function dismiss() {
    hideWelcome();
    DismissWelcome().catch((error) => Debug(`dismiss welcome: ${error}`));
    host.onClosed();
}

/**
 * Opens the Keyboard pane of System Settings; the welcome stays for when
 * the user comes back.
 */
function openSettings() {
    OpenKeyboardSettings().catch((error) => Debug(`keyboard settings: ${error}`));
}

/**
 * Builds the welcome from Go's view: the title, how to open hopto (on
 * the first run), one line per shortcut with its state, the Finder
 * warning with its button, and Start.
 * @param {{firstRun: boolean, hotkeys: {tab: string, spec: string, state: string, status: number}[], finderConflict: boolean}} view
 */
function paint(view) {
    const t = host.t();
    const apps = view.hotkeys.find((hotkey) => hotkey.tab === 'apps');
    const links = view.hotkeys.find((hotkey) => hotkey.tab === 'links');

    panel.replaceChildren();
    panel.setAttribute('aria-label', t('welcome.label'));

    const title = document.createElement('h2');
    title.textContent = t(view.firstRun ? 'welcome.title' : 'welcome.problem');
    panel.appendChild(title);

    if (view.firstRun && apps && links) {
        const intro = document.createElement('p');
        intro.className = 'intro';
        intro.textContent = `${t('welcome.open', {apps: prettyHotkey(apps.spec, t, t.platform)})} · ${t('welcome.links', {links: prettyHotkey(links.spec, t, t.platform)})}`;
        panel.appendChild(intro);
    }

    const list = document.createElement('ul');
    for (const hotkey of view.hotkeys) {
        const line = document.createElement('li');
        line.className = `hotkey ${hotkey.state}`;
        line.textContent = t(`welcome.hotkey.${hotkey.state}`, {
            spec: prettyHotkey(hotkey.spec, t, t.platform),
            tab: t(TABS[hotkey.tab]?.label ?? hotkey.tab),
            status: hotkey.status,
        });
        list.appendChild(line);
    }
    panel.appendChild(list);

    const actions = document.createElement('div');
    actions.className = 'actions';

    if (view.finderConflict) {
        const warning = document.createElement('p');
        warning.className = 'warning';
        warning.textContent = t('welcome.finder', {spec: prettyHotkey(FINDER_SPEC, t, t.platform)});
        panel.appendChild(warning);

        actions.appendChild(button('welcome-settings', t('welcome.settings'), openSettings));
    }

    actions.appendChild(button('welcome-start', t('welcome.start'), dismiss));
    panel.appendChild(actions);
}

/**
 * One button of the welcome.
 * @param {string} id
 * @param {string} text
 * @param {() => void} onPress
 * @returns {HTMLButtonElement}
 */
function button(id, text, onPress) {
    const element = document.createElement('button');
    element.type = 'button';
    element.id = id;
    element.textContent = text;
    element.addEventListener('click', onPress);

    return element;
}
