import {test, expect} from '@playwright/test';

// Every test starts from a fresh page with the `shown` event fired for a
// tab, and ends with no page error at all: a thrown exception in the page
// is a failure whatever else passed.
const errors = [];

test.beforeEach(async ({page}) => {
    errors.length = 0;
    page.on('pageerror', (error) => errors.push(String(error)));
    await page.goto(process.env.HARNESS_URL + '/');
    await page.waitForFunction(() => typeof window.emit === 'function');
});

test.afterEach(() => {
    expect(errors).toEqual([]);
});

/**
 * Fires `shown` for a tab and waits for the list.
 * @param {import('@playwright/test').Page} page
 * @param {string} tab
 */
async function shown(page, tab) {
    await page.evaluate((name) => window.emit(name), tab);
    await expect(page.locator('#grid [role="option"]').first()).toBeVisible();
    await expect(page.locator('#search')).toBeFocused();
}

/**
 * The calls the page made to Go.
 * @param {import('@playwright/test').Page} page
 * @returns {Promise<string[]>}
 */
function calls(page) {
    return page.evaluate(() => window.calls.filter((call) => !call.startsWith('Debug:')));
}

test('shown on links lists sections and Enter opens the first favourite-less recent link', async ({page}) => {
    await shown(page, 'links');

    await expect(page.locator('#grid .section')).toHaveText(['Recent', 'Ecosystem', 'Docs']);
    await expect(page.locator('#grid [role="option"]').first()).toHaveAttribute('aria-selected', 'true');
    await expect(page.locator('#action')).toHaveText('↩ Open Gitea');

    await page.keyboard.press('Enter');
    expect(await calls(page)).toContain('OpenLink:gitea');
});

test('Tab switches to apps and tells Go; cards show the virtual chips', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Tab');

    expect(await calls(page)).toContain('TabChanged:apps');
    await expect(page.locator('#grid .card')).toHaveCount(3);
    await expect(page.locator('#categories .chip')).toHaveText(['All', '★ Favourites', 'Tools', 'Applications', 'Edge apps']);
});

test('⌘F toggles the favourite and the favourites section appears', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+f');

    expect(await calls(page)).toContain('ToggleFavorite:links:gitea');
    await expect(page.locator('#grid .section').first()).toHaveText('Favourites');
    await expect(page.locator('#grid [role="option"]').first().locator('.star')).toHaveAttribute('aria-pressed', 'true');
});

test('typing runs the unified search with badges and pairs the Edge app with its link', async ({page}) => {
    await shown(page, 'apps');
    await page.keyboard.type('git');

    const rows = page.locator('#grid [role="option"]');
    await expect(rows).toHaveCount(2);
    await expect(rows.nth(0).locator('.name')).toHaveText('Gitea');
    await expect(rows.nth(1).locator('.kind')).toHaveText('app · link');
    await expect(page.locator('#categories')).toBeHidden();
    await expect(page.locator('#tab-links .count')).toHaveText('2');

    await page.keyboard.press('ArrowDown');
    await expect(page.locator('#action')).toHaveText('↩ app · ⌘↩ web');

    await page.keyboard.press('Meta+Enter');
    expect(await calls(page)).toContain('OpenLink:github');
});

test('Escape clears the search first and hides on the second press', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('mdn');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(1);

    await page.keyboard.press('Escape');
    await expect(page.locator('#search')).toHaveValue('');
    expect(await calls(page)).not.toContain('Hide');

    await page.keyboard.press('Escape');
    expect(await calls(page)).toContain('Hide');
});

test('chips filter, ⌘3 picks the first category, ⌘C copies with a toast', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+3');

    await expect(page.locator('#categories .chip[aria-pressed="true"]')).toHaveText('Ecosystem');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(2);

    await page.keyboard.press('Meta+c');
    expect(await calls(page)).toContain('CopyTarget:gitea');
    await expect(page.locator('#toast')).toHaveText('Copied');
});

test('nothing matching points at the other tab', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('zzz');
    await expect(page.locator('#empty')).toHaveText('Nothing matches');

    await page.keyboard.press('Escape');
    await page.keyboard.press('Tab');
    await page.keyboard.type('mdn');
    await expect(page.locator('#empty')).toBeHidden();
    await expect(page.locator('#grid [role="option"]')).toHaveCount(1);
});

test('? opens the help and Esc closes it; a broken library shows its line', async ({page}) => {
    await page.evaluate(() => {
        window.libraryStatus = {path: '/x/library.toml', error: 'expected key', line: 12, readOnly: true};
    });
    await shown(page, 'links');

    await expect(page.locator('#status')).toHaveText('library.toml, line 12: expected key');

    await page.keyboard.press('?');
    await expect(page.locator('#help')).toBeVisible();
    await expect(page.locator('#help li').last()).toContainText('cmd+shift+space');

    await page.keyboard.press('Escape');
    await expect(page.locator('#help')).toBeHidden();
});

test('the page speaks Spanish when the settings say so', async ({page}) => {
    await page.evaluate(() => {
        window.language = 'es';
    });
    await shown(page, 'apps');

    await expect(page.locator('#tab-apps .label')).toHaveText('Mis apps');
    await expect(page.locator('#categories .chip').last()).toHaveText('Apps de Edge');
    await expect(page.locator('html')).toHaveAttribute('lang', 'es');
});

test('a second shown resets query, chip and selection', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('mdn');
    await page.keyboard.press('Meta+3');

    await shown(page, 'apps');
    await expect(page.locator('#search')).toHaveValue('');
    await expect(page.locator('#categories .chip[aria-pressed="true"]')).toHaveText('All');
    await expect(page.locator('#tab-apps')).toHaveAttribute('aria-selected', 'true');
});
