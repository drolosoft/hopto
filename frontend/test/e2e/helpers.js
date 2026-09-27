import {expect} from '@playwright/test';

/**
 * Registers what every spec file shares: a fresh page with the bridge in
 * place before each test, and after it no page error and no CSP
 * violation (the console is the only place a blocked request shows up).
 * @param {import('@playwright/test').TestType} test
 */
export function guardEveryTest(test) {
    const problems = [];

    test.beforeEach(async ({page}) => {
        problems.length = 0;
        page.on('pageerror', (error) => problems.push(String(error)));
        page.on('console', (message) => {
            if (message.text().includes('Content Security Policy')) {
                problems.push(message.text());
            }
        });
        await page.goto(process.env.HARNESS_URL + '/');
        await page.waitForFunction(() => typeof window.emit === 'function');
    });

    test.afterEach(() => {
        expect(problems).toEqual([]);
    });
}

/**
 * Fires `shown` for a tab and waits for the list and the focus.
 * @param {import('@playwright/test').Page} page
 * @param {string} tab
 */
export async function shown(page, tab) {
    await page.evaluate((name) => window.emit(name), tab);
    await expect(page.locator('#grid [role="option"]').first()).toBeVisible();
    await expect(page.locator('#search')).toBeFocused();
}

/**
 * The calls the page made to Go, without the Debug noise.
 * @param {import('@playwright/test').Page} page
 * @returns {Promise<string[]>}
 */
export function calls(page) {
    return page.evaluate(() => window.calls.filter((call) => !call.startsWith('Debug:')));
}

/**
 * What followed "<name>:" in each call to one Go method, oldest first.
 * @param {import('@playwright/test').Page} page
 * @param {string} name
 * @returns {Promise<string[]>}
 */
export async function payloads(page, name) {
    const made = await calls(page);

    return made
        .filter((call) => call.startsWith(`${name}:`))
        .map((call) => call.slice(name.length + 1));
}
