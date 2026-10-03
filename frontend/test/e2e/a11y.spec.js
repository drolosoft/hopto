import {test, expect} from '@playwright/test';
import {guardEveryTest, shown} from './helpers.js';

guardEveryTest(test);

// What the style card calls a control: anything the user can press,
// type into or pick. Every one has to opt out of the window drag, or a
// click on it would move the window instead.
const CONTROLS = 'button, input, textarea, select, [role="tab"], [role="option"], [tabindex]';

/**
 * The controls on screen whose --wails-draggable is not no-drag, by
 * their id, class or tag, so a failure names them.
 * @param {import('@playwright/test').Page} page
 * @returns {Promise<string[]>}
 */
function draggableControls(page) {
    return page.evaluate((selector) => [...document.querySelectorAll(selector)]
        .filter((control) => control.checkVisibility())
        .filter((control) => getComputedStyle(control).getPropertyValue('--wails-draggable').trim() !== 'no-drag')
        .map((control) => control.id || control.className || control.tagName), CONTROLS);
}

test('the help is named after its heading, takes the focus and gives it back', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('mdn');

    // With a query typed, ? is text; ⌘/ is the help.
    await page.keyboard.press('Meta+/');
    const help = page.locator('#help');
    await expect(help).toBeVisible();
    await expect(help).toHaveAccessibleName('Shortcuts');
    await expect(help).toBeFocused();

    await page.keyboard.press('Escape');
    await expect(help).toBeHidden();
    await expect(page.locator('#search')).toBeFocused();
    await expect(page.locator('#search')).toHaveValue('mdn');
});

test('the menu bar Help takes the focus too', async ({page}) => {
    await shown(page, 'apps');

    await page.evaluate(() => window.listeners.help(true));
    await expect(page.locator('#help')).toBeFocused();

    await page.keyboard.press('?');
    await expect(page.locator('#help')).toBeHidden();
    await expect(page.locator('#search')).toBeFocused();
});

test('the tabs control a tab panel named after the selected tab', async ({page}) => {
    await shown(page, 'apps');

    const panel = page.locator('[role="tabpanel"]');
    await expect(panel).toHaveCount(1);
    await expect(page.locator('#tab-apps')).toHaveAttribute('aria-controls', await panel.getAttribute('id'));
    await expect(page.locator('#tab-links')).toHaveAttribute('aria-controls', await panel.getAttribute('id'));
    await expect(panel).toHaveAccessibleName(/^Apps/);

    await page.keyboard.press('Tab');
    await expect(panel).toHaveAttribute('aria-labelledby', 'tab-links');
    await expect(panel).toHaveAccessibleName(/^Links/);
});

test('every control on screen opts out of the window drag', async ({page}) => {
    await page.evaluate(() => {
        window.libraryStatus = {path: '/x/library.toml', error: 'bad', line: 3, readOnly: true};
        window.welcome = {show: true, firstRun: false, hotkeys: [], finderConflict: true};
        window.emit('links');
    });
    await expect(page.locator('#welcome-settings')).toBeVisible();
    await expect(page.locator('#status .fix')).toBeVisible();
    expect(await draggableControls(page)).toEqual([]);

    await page.keyboard.press('Escape');
    await expect(page.locator('#welcome')).toBeHidden();
    expect(await draggableControls(page)).toEqual([]);

    await page.keyboard.type('zzz');
    await expect(page.locator('#grid .row.action').first()).toBeVisible();
    expect(await draggableControls(page)).toEqual([]);

    await page.keyboard.press('Escape');
    await expect(page.locator('#search')).toHaveValue('');
    await page.keyboard.press('?');
    await expect(page.locator('#help')).toBeVisible();
    expect(await draggableControls(page)).toEqual([]);
});

test('every control of the editor opts out of the window drag', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('example.org');
    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeVisible();

    expect(await draggableControls(page)).toEqual([]);
});
