import {test, expect} from '@playwright/test';
import {guardEveryTest, shown} from './helpers.js';

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
