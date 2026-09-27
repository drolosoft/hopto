import {test, expect} from '@playwright/test';
import {guardEveryTest, shown, calls, payloads} from './helpers.js';

guardEveryTest(test);

// The links tab lists Gitea (recent), GitHub (Ecosystem), MDN (Docs).
const selected = (page) => page.locator('#grid [aria-selected="true"]');

test('⌘E opens the editor filled; Enter saves through UpdateLink with the keywords', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+e');

    await expect(page.locator('#editor h2')).toHaveText('Edit link');
    await expect(page.locator('#editor-url')).toHaveValue('https://repos.example.com');
    await expect(page.locator('#editor-name')).toHaveValue('Gitea');
    await expect(page.locator('#editor-categories [aria-checked="true"]')).toHaveText('Ecosystem');
    expect(await payloads(page, 'InspectURL')).toEqual([]);

    await page.locator('#editor-name').fill('Gitea at home');
    await page.keyboard.press('Enter');

    const [update] = await payloads(page, 'UpdateLink');
    expect(update.startsWith('gitea:')).toBe(true);
    expect(JSON.parse(update.slice('gitea:'.length))).toEqual({url: 'https://repos.example.com', name: 'Gitea at home', description: 'Repos', category: 'eco', keywords: ['git'], icon: ''});
    await expect(page.locator('#toast')).toHaveText('Saved');
    await expect(selected(page).locator('.name')).toHaveText('Gitea at home');
});

test('Review Focus 4: ⌘⌫ asks in the row and the footer, Esc keeps, Enter deletes', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowDown');
    await expect(selected(page).locator('.name')).toHaveText('MDN');

    await page.keyboard.press('Meta+Backspace');
    await expect(selected(page).locator('.description')).toHaveText('Delete MDN? ↩ yes · Esc no');
    await expect(page.locator('#action')).toHaveText('Delete MDN? ↩ yes · Esc no');

    await page.keyboard.press('Escape');
    await expect(selected(page).locator('.description')).toHaveText('Web reference');
    expect(await calls(page)).not.toContain('Hide');

    await page.keyboard.press('Meta+Backspace');
    await page.keyboard.press('Enter');
    await expect(page.locator('#toast')).toHaveText('Deleted');
    expect(await payloads(page, 'DeleteLink')).toEqual(['mdn']);
    expect(await calls(page)).not.toContain('OpenLink:mdn');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(2);
});

test('moving away, typing or a new shown drop the question without deleting', async ({page}) => {
    await shown(page, 'links');

    await page.keyboard.press('Meta+Backspace');
    await page.keyboard.press('ArrowDown');
    await expect(page.locator('#grid .confirming')).toHaveCount(0);
    await expect(selected(page).locator('.name')).toHaveText('GitHub');

    await page.keyboard.press('Meta+Backspace');
    await page.keyboard.type('g');
    await expect(page.locator('#grid .confirming')).toHaveCount(0);

    await page.keyboard.press('Meta+Backspace');
    await shown(page, 'links');
    await expect(page.locator('#grid .confirming')).toHaveCount(0);
    await page.keyboard.press('Enter');

    expect(await payloads(page, 'DeleteLink')).toEqual([]);
});

test('a found app is hidden after asking, and comes back from the Hidden chip at once', async ({page}) => {
    await shown(page, 'apps');
    await page.keyboard.press('ArrowRight');
    await expect(selected(page).locator('.name')).toHaveText('Mail');

    await page.keyboard.press('Meta+Backspace');
    await expect(page.locator('#action')).toHaveText('Hide Mail? ↩ yes · Esc no');
    await page.keyboard.press('Enter');

    expect(await payloads(page, 'HideApp')).toEqual(['app-mail']);
    await expect(page.locator('#toast')).toHaveText('Hidden');
    await expect(page.locator('#grid .card')).toHaveCount(2);

    await page.locator('#categories .chip', {hasText: 'Hidden'}).click();
    await expect(page.locator('#grid .card')).toHaveCount(1);
    await expect(selected(page).locator('.description')).toHaveText('Hidden');

    await page.keyboard.press('Meta+Backspace');
    expect(await payloads(page, 'UnhideApp')).toEqual(['app-mail']);
    await expect(page.locator('#toast')).toHaveText('Back in the list');
    await expect(page.locator('#categories .chip', {hasText: 'Hidden'})).toHaveCount(0);
    await expect(page.locator('#categories .chip[aria-pressed="true"] .label')).toHaveText('All');
});

test('a hand-added app is deleted, not hidden', async ({page}) => {
    await shown(page, 'apps');
    await page.keyboard.press('Meta+Backspace');
    await expect(page.locator('#action')).toHaveText('Delete Mine? ↩ yes · Esc no');

    await page.keyboard.press('Enter');
    expect(await payloads(page, 'DeleteApp')).toEqual(['mine']);
    expect(await payloads(page, 'HideApp')).toEqual([]);
});

test('⌘⇧E renames the active chip in place; Esc leaves it alone', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+4');
    await expect(page.locator('#categories .chip[aria-pressed="true"] .label')).toHaveText('Docs');

    await page.keyboard.press('Meta+Shift+e');
    await expect(page.locator('#rename-category')).toBeFocused();
    await expect(page.locator('#rename-category')).toHaveValue('Docs');
    await page.keyboard.press('Escape');
    await expect(page.locator('#rename-category')).toHaveCount(0);
    expect(await payloads(page, 'RenameCategory')).toEqual([]);

    await page.keyboard.press('Meta+Shift+e');
    await page.keyboard.type('Reference');
    await page.keyboard.press('Enter');

    expect(await payloads(page, 'RenameCategory')).toEqual(['links:docs:Reference']);
    await expect(page.locator('#categories .chip[aria-pressed="true"] .label')).toHaveText('Reference');
    await expect(page.locator('#toast')).toHaveText('Renamed');
    await expect(page.locator('#search')).toBeFocused();
});

test('⌘⇧⌫ refuses a chip with items and deletes an empty one after asking', async ({page}) => {
    await shown(page, 'links');
    await page.keyboard.press('Meta+4');

    await page.keyboard.press('Meta+Shift+Backspace');
    await expect(page.locator('#toast')).toHaveText('The category still has items');
    expect(await payloads(page, 'DeleteCategory')).toEqual([]);

    await page.keyboard.press('Meta+Backspace');
    await page.keyboard.press('Enter');
    await expect(page.locator('#grid [role="option"]')).toHaveCount(0);

    await page.keyboard.press('Meta+Shift+Backspace');
    await expect(page.locator('#action')).toHaveText('Delete the category Docs? ↩ yes · Esc no');
    await page.keyboard.press('Enter');

    expect(await payloads(page, 'DeleteCategory')).toEqual(['links:docs']);
    await expect(page.locator('#categories .chip', {hasText: 'Docs'})).toHaveCount(0);
    await expect(page.locator('#categories .chip[aria-pressed="true"] .label')).toHaveText('All');
});

test('the duplicate note offers ⌘E, which opens the existing link', async ({page}) => {
    await page.evaluate(() => {
        window.inspectDraft = {url: 'https://github.com', host: 'github.com', name: 'GitHub', description: '', iconDataUrl: '', insecure: false, duplicate: {tab: 'links', id: 'github', name: 'GitHub'}, sameHost: []};
    });
    await shown(page, 'links');
    await page.keyboard.press('Meta+n');
    await page.keyboard.type('https://github.com');

    await expect(page.locator('#editor .notes')).toContainText('You already have it: GitHub · ⌘E edits it');
    await page.keyboard.press('Meta+e');

    await expect(page.locator('#editor h2')).toHaveText('Edit link');
    await expect(page.locator('#editor-name')).toHaveValue('GitHub');
    await expect(page.locator('#editor-description')).toHaveValue('Pull requests');
});
