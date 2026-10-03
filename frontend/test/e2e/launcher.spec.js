import {test, expect} from '@playwright/test';
import {guardEveryTest, shown, calls, onWindows} from './helpers.js';

guardEveryTest(test);

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
    await expect(page.locator('#categories .chip .label')).toHaveText(['All', '★ Favourites', 'Tools', 'Applications', 'Edge apps']);
});

test('category chips carry their own count, like the tabs do', async ({page}) => {
    await shown(page, 'links');

    // The fixture's links tab: 3 links, none favourited yet, 2 in
    // Ecosystem (Gitea, GitHub) and 1 in Docs (mdn).
    await expect(page.locator('#categories .chip .label')).toHaveText(['All', '★ Favourites', 'Ecosystem', 'Docs']);
    await expect(page.locator('#categories .chip .count')).toHaveText(['3', '0', '2', '1']);

    await page.keyboard.press('Meta+f');
    await expect(page.locator('#categories .chip').nth(1).locator('.count')).toHaveText('1');
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

    await expect(page.locator('#categories .chip[aria-pressed="true"] .label')).toHaveText('Ecosystem');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(2);

    await page.keyboard.press('Meta+c');
    expect(await calls(page)).toContain('CopyTarget:gitea');
    await expect(page.locator('#toast')).toHaveText('Copied');
});

test('nothing matching offers to add it instead of an empty list', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('zzz');
    await expect(page.locator('#empty')).toBeHidden();
    await expect(page.locator('#grid [role="option"]')).toHaveCount(1);
    await expect(page.locator('#grid .row.action .name')).toHaveText('Add “zzz”');

    await page.keyboard.press('Escape');
    await page.keyboard.press('Tab');
    await page.keyboard.type('mdn');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(1);
    await expect(page.locator('#grid .row.action')).toHaveCount(0);
});

test('? opens the help and Esc closes it; a broken library shows its line', async ({page}) => {
    await page.evaluate(() => {
        window.libraryStatus = {path: '/x/library.toml', error: 'expected key', line: 12, readOnly: true};
    });
    await shown(page, 'links');

    await expect(page.locator('#status .text')).toHaveText('library.toml, line 12: expected key');

    await page.keyboard.press('?');
    await expect(page.locator('#help')).toBeVisible();
    // The last shortcut line, not the about facts that now close the
    // panel below it.
    await expect(page.locator('#help li:not(.about)').last()).toContainText('cmd+shift+space');

    await page.keyboard.press('Escape');
    await expect(page.locator('#help')).toBeHidden();
});

test('the page speaks Spanish when the settings say so', async ({page}) => {
    await page.evaluate(() => {
        window.language = 'es';
    });
    await shown(page, 'apps');

    await expect(page.locator('#tab-apps .label')).toHaveText('Mis apps');
    await expect(page.locator('#tab-apps .icon')).toBeVisible();
    await expect(page.locator('#categories .chip .label').last()).toHaveText('Apps de Edge');
    await expect(page.locator('html')).toHaveAttribute('lang', 'es');
});

test('⌘⇧↩ opens nothing with no chip picked, and only the chip once one is', async ({page}) => {
    await shown(page, 'links');

    await page.keyboard.press('Meta+Shift+Enter');
    let made = await calls(page);
    expect(made.filter((call) => call.startsWith('Launch:') || call.startsWith('OpenLink'))).toEqual([]);

    // The "Docs" chip holds one link, mdn; ⌘3 is "All", so Docs is further
    // along, picked by clicking it directly instead of counting chips.
    await page.locator('#categories .chip', {hasText: 'Docs'}).click();
    await page.keyboard.press('Meta+Shift+Enter');

    made = await calls(page);
    expect(made.filter((call) => call === 'OpenLink:mdn')).toHaveLength(1);
});

test('ArrowDown on the grouped apps grid follows the visual column, not the flat index', async ({page}) => {
    await shown(page, 'apps');

    await expect(page.locator('#grid .section')).toHaveText(['Tools', 'Applications', 'Edge apps']);
    await expect(page.locator('#grid [role="option"]')).toHaveCount(3);
    await expect(page.locator('#grid [role="option"]').nth(0)).toHaveAttribute('aria-selected', 'true');

    await page.keyboard.press('ArrowDown');
    await expect(page.locator('#grid [role="option"]').nth(1)).toHaveAttribute('aria-selected', 'true');

    await page.keyboard.press('ArrowDown');
    await expect(page.locator('#grid [role="option"]').nth(2)).toHaveAttribute('aria-selected', 'true');
});

test('grouped card sections span the grid: a header sits above its group, not beside it', async ({page}) => {
    await shown(page, 'apps');

    // The fixture puts one card in each of three sections (Tools,
    // Applications, Edge apps); every card should start at the same x as
    // the grid's single column, and the second header should sit below
    // the first card rather than squeezed into a neighbouring column.
    const cards = page.locator('#grid .card');
    await expect(cards).toHaveCount(3);

    const firstBox = await cards.nth(0).boundingBox();
    for (let index = 1; index < 3; index += 1) {
        const box = await cards.nth(index).boundingBox();
        expect(Math.abs(box.x - firstBox.x)).toBeLessThanOrEqual(2);
    }

    const secondHeaderBox = await page.locator('#grid .section').nth(1).boundingBox();
    expect(secondHeaderBox.y).toBeGreaterThan(firstBox.y + firstBox.height);
});

test('a second shown resets query, chip and selection', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('mdn');
    await page.keyboard.press('Meta+3');

    await shown(page, 'apps');
    await expect(page.locator('#search')).toHaveValue('');
    await expect(page.locator('#categories .chip[aria-pressed="true"] .label')).toHaveText('All');
    await expect(page.locator('#tab-apps')).toHaveAttribute('aria-selected', 'true');
});

test('the help names the build and the library file', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('?');

    const facts = page.locator('#help li.about');
    await expect(facts).toHaveText([
        'hopto v0.3.0-test · abc1234 · built 2026-09-27 · go1.27.1',
        'Library: /Users/someone/Library/Application Support/hopto/library.toml',
    ]);
});

test('plan 3 minor (e): the library path in the help wraps only where it has to', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('?');

    const about = page.locator('#help li.about').last();
    await expect(about).toBeVisible();
    expect(await about.evaluate((item) => getComputedStyle(item).overflowWrap)).toBe('anywhere');
    expect(await about.evaluate((item) => getComputedStyle(item).wordBreak)).not.toBe('break-all');
    expect(await page.locator('#help').evaluate((help) => help.scrollWidth <= help.clientWidth)).toBe(true);
});

test('the menu bar Help opens the help panel over a fresh list', async ({page}) => {
    await shown(page, 'apps');
    await page.evaluate(() => window.listeners.help(true));

    await expect(page.locator('#help')).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.locator('#help')).toBeHidden();
    expect(await calls(page)).not.toContain('Hide');
});

test('supuesto 3: ↑ and ↓ stop at the edges of the cards grid, ← and → wrap', async ({page}) => {
    await shown(page, 'apps');
    const options = page.locator('#grid [role="option"]');

    await page.keyboard.press('ArrowUp');
    await expect(options.nth(0)).toHaveAttribute('aria-selected', 'true');

    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    await expect(options.nth(2)).toHaveAttribute('aria-selected', 'true');

    await page.keyboard.press('ArrowRight');
    await expect(options.nth(0)).toHaveAttribute('aria-selected', 'true');
});

test('on Windows the footer and the help speak Control, and Ctrl+Enter opens elsewhere', async ({page}) => {
    await onWindows(page);
    await shown(page, 'apps');
    await page.keyboard.type('git');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(2);

    await page.keyboard.press('ArrowDown');
    await expect(page.locator('#action')).toHaveText('Enter app · Ctrl+Enter web');

    await page.keyboard.press('Control+Enter');
    expect(await calls(page)).toContain('OpenLink:github');

    await page.keyboard.press('?');
    await expect(page.locator('#help')).toContainText('show in Explorer');
    await expect(page.locator('#help')).not.toContainText('⌘');
});

test('on Windows the Tab key is written as a word, not the macOS glyph', async ({page}) => {
    await onWindows(page);
    await shown(page, 'links');

    await expect(page.locator('#hints')).toHaveText('Ctrl+F ★ · Tab Apps · ? help');
    await page.keyboard.press('?');
    await expect(page.locator('#help')).toContainText('Tab other tab');
    await expect(page.locator('#help')).not.toContainText('⇥');
});

test('the add row carries its plus once, in the tile', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('zzz');

    const row = page.locator('#grid .row.action');
    await expect(row.locator('.tile')).toHaveText('＋');
    await expect(row.locator('.name')).toHaveText('Add “zzz”');
});
