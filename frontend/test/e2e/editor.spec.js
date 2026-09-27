import {test, expect} from '@playwright/test';
import {guardEveryTest, shown, calls, payloads} from './helpers.js';

guardEveryTest(test);

test('a domain with no results offers the add row; Enter, ⌘2 and Enter add the link', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('example.org');

    const rows = page.locator('#grid [role="option"]');
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toHaveAttribute('aria-selected', 'true');
    await expect(rows.first().locator('.name')).toHaveText('＋ Add “example.org”');
    await expect(page.locator('#action')).toHaveText('↩ Add');

    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeVisible();
    await expect(page.locator('#content')).toBeHidden();
    await expect(page.locator('#editor-url')).toHaveValue('https://example.org');
    await expect(page.locator('#editor-url')).toBeFocused();
    await expect(page.locator('#editor-name')).toHaveValue('Example Domain');
    expect(await calls(page)).toContain('InspectURL:https://example.org');
    await expect(page.locator('#editor-categories [aria-checked="true"]')).toHaveText('Ecosystem');

    await page.keyboard.press('Meta+2');
    await expect(page.locator('#editor-categories [aria-checked="true"]')).toHaveText('Docs');

    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeHidden();

    const [saved] = await payloads(page, 'AddLink');
    expect(JSON.parse(saved)).toEqual({url: 'https://example.org', name: 'Example Domain', description: 'Illustrative', category: 'docs', keywords: [], icon: ''});
    await expect(page.locator('#toast')).toHaveText('Added');
    await expect(page.locator('#search')).toHaveValue('');
    await expect(page.locator('#grid [aria-selected="true"] .name')).toHaveText('Example Domain');
    await expect(page.locator('#search')).toBeFocused();
});

test('a plain word becomes the name, the URL waits empty and Enter says why it cannot save', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('Grafana');
    await page.keyboard.press('Enter');

    await expect(page.locator('#editor-name')).toHaveValue('Grafana');
    await expect(page.locator('#editor-url')).toHaveValue('');
    await expect(page.locator('#editor-url')).toBeFocused();

    await page.keyboard.press('Enter');
    await expect(page.locator('#problem-url')).toHaveText('Type a web address, like example.org');
    expect(await payloads(page, 'AddLink')).toEqual([]);

    await page.keyboard.type('grafana.example.org');
    await expect(page.locator('#problem-url')).toBeHidden();
    await expect.poll(() => calls(page)).toContain('InspectURL:https://grafana.example.org');
    await expect(page.locator('#editor-name')).toHaveValue('Grafana');

    await page.keyboard.press('Enter');
    const [saved] = await payloads(page, 'AddLink');
    expect(JSON.parse(saved)).toMatchObject({url: 'https://grafana.example.org', name: 'Grafana'});
});

test('an address with results still offers the add row, last', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('github.com');

    const rows = page.locator('#grid [role="option"]');
    expect(await rows.count()).toBeGreaterThan(1);
    await expect(rows.last().locator('.name')).toHaveText('＋ Add “github.com”');
    await expect(rows.first()).toHaveAttribute('aria-selected', 'true');
});

test('Esc closes only the editor and gives the text back to the search box', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('nothing here');
    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeVisible();

    await page.keyboard.press('Escape');
    await expect(page.locator('#editor')).toBeHidden();
    await expect(page.locator('#search')).toHaveValue('nothing here');
    await expect(page.locator('#search')).toBeFocused();
    await expect(page.locator('#grid [role="option"]').first()).toHaveAttribute('aria-selected', 'true');
    expect(await calls(page)).not.toContain('Hide');

    await page.keyboard.press('Escape');
    await expect(page.locator('#search')).toHaveValue('');
    await page.keyboard.press('Escape');
    expect(await calls(page)).toContain('Hide');
});

test('⌘N, a new category typed in line, and Enter create both', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+n');
    await expect(page.locator('#editor-url')).toBeFocused();

    await page.keyboard.type('https://new.example.org');
    await page.keyboard.press('Meta+3');
    await expect(page.locator('#editor-new-category')).toBeFocused();

    await page.keyboard.type('Tools');
    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeHidden();

    expect(await payloads(page, 'AddCategory')).toEqual(['links:Tools']);
    const [saved] = await payloads(page, 'AddLink');
    expect(JSON.parse(saved).category).toBe('tools');
});

test('Esc in the new-category field drops the name, the second Esc closes', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+n');
    await page.keyboard.press('Meta+3');
    await expect(page.locator('#editor-new-category')).toBeFocused();

    await page.keyboard.press('Escape');
    await expect(page.locator('#editor')).toBeVisible();
    await expect(page.locator('#editor-new-category')).toHaveCount(0);
    await expect(page.locator('#editor-categories')).toBeFocused();

    await page.keyboard.press('ArrowRight');
    await expect(page.locator('#editor-categories [aria-checked="true"]')).toHaveText('Docs');

    await page.keyboard.press('Escape');
    await expect(page.locator('#editor')).toBeHidden();
});

test('an exact duplicate blocks the save and says which link it is', async ({page}) => {
    await page.evaluate(() => {
        window.inspectDraft = {url: 'https://twin.example.org', host: 'twin.example.org', name: 'Twin', description: '', iconDataUrl: '', insecure: false, duplicate: {tab: 'links', id: 'github', name: 'GitHub'}, sameHost: []};
    });
    await shown(page, 'links');
    await page.keyboard.type('twin.example.org');
    await page.keyboard.press('Enter');

    await expect(page.locator('#editor .notes')).toContainText('You already have it: GitHub');
    await page.keyboard.press('Enter');
    expect(await payloads(page, 'AddLink')).toEqual([]);
    await expect(page.locator('#editor')).toBeVisible();
});

test('a problem from Go lands under its field', async ({page}) => {
    await page.evaluate(() => {
        window.nextSave = {id: '', problems: {name: 'name.long'}, duplicate: null};
    });
    await shown(page, 'links');
    await page.keyboard.type('long.example.org');
    await page.keyboard.press('Enter');
    await page.keyboard.press('Enter');

    await expect(page.locator('#problem-name')).toHaveText('At most 80 characters');
    await expect(page.locator('#editor')).toBeVisible();
});

test('shown closes the editor and a late inspection changes nothing', async ({page}) => {
    await page.evaluate(() => {
        window.inspectDelay = 600;
    });
    await shown(page, 'links');
    await page.keyboard.type('late.example.org');
    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeVisible();

    await page.evaluate(() => window.emit('apps'));
    await expect(page.locator('#editor')).toBeHidden();
    await expect(page.locator('#search')).toHaveValue('');

    await page.waitForTimeout(800);
    await expect(page.locator('#editor')).toBeHidden();
    await expect(page.locator('#grid [role="option"]').first()).toBeVisible();
});

test('plan 3 minor (b): the menu bar Help closes the editor before it shows', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.type('example.org');
    await page.keyboard.press('Enter');
    await expect(page.locator('#editor')).toBeVisible();

    await page.evaluate(() => window.listeners.help(true));
    await expect(page.locator('#editor')).toBeHidden();
    await expect(page.locator('#help')).toBeVisible();
    await expect(page.locator('#search')).toHaveValue('example.org');

    await page.keyboard.press('Escape');
    await expect(page.locator('#help')).toBeHidden();
    expect(await payloads(page, 'AddLink')).toEqual([]);
    expect(await calls(page)).not.toContain('Hide');
});
