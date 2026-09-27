import {test, expect} from '@playwright/test';
import {guardEveryTest, shown, calls, payloads} from './helpers.js';

guardEveryTest(test);

test('supuesto 1: a system app is found by typing, and only then', async ({page}) => {
    await shown(page, 'apps');

    await expect(page.locator('#grid .card')).toHaveCount(3);
    await expect(page.locator('#tab-apps .count')).toHaveText('3');
    await expect(page.locator('#totals')).toHaveText('3 apps · 3 links');

    await page.locator('#categories .chip', {hasText: 'Applications'}).click();
    await expect(page.locator('#grid [role="option"] .name')).toHaveText(['Mail']);

    await page.keyboard.type('calc');
    await expect(page.locator('#grid [role="option"]').first().locator('.name')).toHaveText('Calculator');
    await expect(page.locator('#tab-apps .count')).toHaveText('1');
});

const selected = (page) => page.locator('#grid [aria-selected="true"]');

test('supuesto 4: on the apps tab the search offers Search Applications…, which adds an app', async ({page}) => {
    await shown(page, 'apps');
    await page.keyboard.type('qqq');

    const rows = page.locator('#grid [role="option"] .name');
    await expect(rows).toHaveText(['＋ Add “qqq”', 'Search Applications…']);

    await page.keyboard.press('ArrowDown');
    await expect(page.locator('#action')).toHaveText('↩ Choose');
    await page.keyboard.press('Enter');

    await expect.poll(() => calls(page)).toContain('PickApp');
    await expect(page.locator('#editor h2')).toHaveText('New app');
    await expect(page.locator('#editor-path')).toHaveValue('/Applications/Example.app');
    await expect(page.locator('#editor-name')).toHaveValue('Example');
    await expect(page.locator('#editor-name')).toBeFocused();

    await page.keyboard.press('Meta+1');
    await page.keyboard.press('Enter');

    const [saved] = await payloads(page, 'AddApp');
    expect(JSON.parse(saved)).toEqual({path: '/Applications/Example.app', bundleId: 'org.example.app', name: 'Example', description: '', category: 'tools'});
    await expect(page.locator('#toast')).toHaveText('Added');
    await expect(selected(page).locator('.name')).toHaveText('Example');
});

test('⌘N on the apps tab opens the dialog; a cancel closes the editor again', async ({page}) => {
    await page.evaluate(() => {
        window.nextPick = {path: '', bundleId: '', name: '', description: '', iconDataUrl: '', problem: '', duplicate: null};
    });
    await shown(page, 'apps');
    await page.keyboard.press('Meta+n');

    await expect.poll(() => calls(page)).toContain('PickApp');
    await expect(page.locator('#editor')).toBeHidden();
    await expect(page.locator('#search')).toBeFocused();
    expect(await payloads(page, 'AddApp')).toEqual([]);
});

test('Review Focus 1: a blur while the dialog is up does not hide the panel', async ({page}) => {
    await page.evaluate(() => {
        window.pickDelay = 800;
        document.hasFocus = () => false;
    });
    await shown(page, 'apps');
    await page.keyboard.press('Meta+n');
    await expect.poll(() => calls(page)).toContain('PickApp');

    await page.evaluate(() => window.dispatchEvent(new Event('blur')));
    await page.waitForTimeout(300);
    expect(await calls(page)).not.toContain('Hide');

    await expect(page.locator('#editor-path')).toHaveValue('/Applications/Example.app');
});

test('a bundle outside the app folders is reported and cannot be saved', async ({page}) => {
    await page.evaluate(() => {
        window.nextPick = {path: '/Users/someone/Downloads/Loose.app', bundleId: '', name: 'Loose', description: '', iconDataUrl: '', problem: 'app.path', duplicate: null};
    });
    await shown(page, 'apps');
    await page.keyboard.press('Meta+n');

    await expect(page.locator('#problem-path')).toHaveText('Only apps in /Applications, /System/Applications or ~/Applications');
    await page.keyboard.press('Enter');
    expect(await payloads(page, 'AddApp')).toEqual([]);
    await expect(page.locator('#editor')).toBeVisible();
});

test('⌘E edits a hand-added app; ⌘O picks again and keeps the typed name', async ({page}) => {
    await shown(page, 'apps');
    await expect(selected(page).locator('.name')).toHaveText('Mine');
    await page.keyboard.press('Meta+e');

    await expect(page.locator('#editor h2')).toHaveText('Edit app');
    await expect(page.locator('#editor-path')).toHaveValue('/Applications/Mine.app');
    expect(await calls(page)).not.toContain('PickApp');

    await page.locator('#editor-name').fill('Kept');
    await page.keyboard.press('Meta+o');
    await expect(page.locator('#editor-path')).toHaveValue('/Applications/Example.app');
    await expect(page.locator('#editor-name')).toHaveValue('Kept');

    await page.keyboard.press('Enter');
    const [update] = await payloads(page, 'UpdateApp');
    expect(update.startsWith('mine:')).toBe(true);
    expect(JSON.parse(update.slice('mine:'.length))).toMatchObject({path: '/Applications/Example.app', name: 'Kept', category: 'tools'});
    await expect(page.locator('#toast')).toHaveText('Saved');
});

test('⌘E on a found app offers to add it by hand, under a category of the user', async ({page}) => {
    await shown(page, 'apps');
    await page.keyboard.press('ArrowRight');
    await expect(selected(page).locator('.name')).toHaveText('Mail');
    await page.keyboard.press('Meta+e');

    await expect(page.locator('#editor h2')).toHaveText('New app');
    await expect(page.locator('#editor-path')).toHaveValue('/Applications/Mail.app');
    await expect(page.locator('#editor .notes')).toContainText('Found on disk as Mail');
    expect(await calls(page)).not.toContain('PickApp');

    await page.keyboard.press('Enter');
    const [saved] = await payloads(page, 'AddApp');
    expect(JSON.parse(saved)).toEqual({path: '/Applications/Mail.app', bundleId: 'com.apple.mail', name: 'Mail', description: '', category: 'tools'});
});
