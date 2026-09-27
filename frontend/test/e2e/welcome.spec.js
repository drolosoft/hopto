import {test, expect} from '@playwright/test';
import {guardEveryTest, calls} from './helpers.js';

guardEveryTest(test);

// A first run on a Mac where the links shortcut is taken and the Finder
// keeps ⌘⌥Space.
const firstRun = {
    show: true,
    firstRun: true,
    hotkeys: [
        {tab: 'apps', spec: 'cmd+shift+space', state: 'registered', status: 0},
        {tab: 'links', spec: 'cmd+option+space', state: 'taken', status: -9878},
    ],
    finderConflict: true,
};

/**
 * Sets what Welcome answers and fires `shown`.
 * @param {import('@playwright/test').Page} page
 * @param {object|null} view
 * @param {string} tab
 */
async function showWith(page, view, tab = 'apps') {
    await page.evaluate(([answer, name]) => {
        window.welcome = answer;
        window.emit(name);
    }, [view, tab]);
}

test('the page asks Go to present the welcome once it has loaded', async ({page}) => {
    await expect.poll(() => calls(page)).toContain('PresentWelcome');
});

test('the first run explains the shortcuts, their state and the Finder clash', async ({page}) => {
    await showWith(page, firstRun);

    const welcome = page.locator('#welcome');
    await expect(welcome).toBeVisible();
    await expect(welcome.locator('h2')).toHaveText('hopto is ready');
    await expect(welcome.locator('.intro')).toHaveText('Press ⇧⌘Space to open it · ⌘N adds your first link · ⌥⌘Space opens it on your links');
    await expect(welcome.locator('li.hotkey')).toHaveText(['⇧⌘Space · Apps: ready', '⌥⌘Space · Links: another app already uses it']);
    await expect(welcome.locator('.warning')).toContainText('Show Finder search window');
    await expect(page.locator('#welcome-start')).toBeFocused();

    await page.locator('#welcome-settings').click();
    expect(await calls(page)).toContain('OpenKeyboardSettings');
});

test('the welcome keeps the keyboard; Enter closes it and gives the search box back', async ({page}) => {
    await showWith(page, firstRun);
    await expect(page.locator('#welcome-start')).toBeFocused();

    await page.keyboard.type('mdn');
    await expect(page.locator('#search')).toHaveValue('');
    await page.keyboard.press('Meta+n');
    await expect(page.locator('#editor')).toBeHidden();

    await page.keyboard.press('Enter');
    await expect(page.locator('#welcome')).toBeHidden();
    await expect(page.locator('#search')).toBeFocused();
    expect(await calls(page)).toContain('DismissWelcome');
    expect(await calls(page)).not.toContain('Hide');

    await showWith(page, undefined, 'links');
    await expect(page.locator('#grid [role="option"]').first()).toBeVisible();
    await expect(page.locator('#welcome')).toBeHidden();
});

test('Esc closes it too, and a later run with a problem uses the other title', async ({page}) => {
    await showWith(page, {...firstRun, firstRun: false, finderConflict: false});

    await expect(page.locator('#welcome h2')).toHaveText('A shortcut needs a look');
    await expect(page.locator('#welcome .intro')).toHaveCount(0);
    await expect(page.locator('#welcome-settings')).toHaveCount(0);

    await page.keyboard.press('Escape');
    await expect(page.locator('#welcome')).toBeHidden();
    expect(await calls(page)).not.toContain('Hide');
});

test('an empty links tab says how to add one', async ({page}) => {
    await page.evaluate(() => {
        window.noLinks = true;
        window.emit('links');
    });

    await expect(page.locator('#empty')).toHaveText('No links yet · ⌘N adds one');
});

test('a broken library shows its line and a button that opens the file', async ({page}) => {
    await page.evaluate(() => {
        window.libraryStatus = {path: '/Users/someone/Library/Application Support/hopto/library.toml', error: 'expected key', line: 12, readOnly: true};
        window.emit('links');
    });

    await expect(page.locator('#status .text')).toHaveText('library.toml, line 12: expected key');
    await page.locator('#status .fix').click();
    expect(await calls(page)).toContain('EditLibrary');
});
