import {test, expect} from '@playwright/test';
import {guardEveryTest, shown} from './helpers.js';

// Every other spec runs in dark mode, the config's default; this one
// follows a Mac set to Light.
test.use({colorScheme: 'light'});

guardEveryTest(test);

/**
 * Three tokens of the stylesheet resolved to colours, the colour scheme
 * in force, and the computed colours of the parts painted with those
 * tokens. A token is resolved through a probe painted with it, since
 * the build minifies how the stylesheet writes it.
 * @param {import('@playwright/test').Page} page
 * @returns {Promise<{tokens: Object<string, string>, scheme: string, launcher: string, text: string, muted: string}>}
 */
function colours(page) {
    return page.evaluate(() => {
        const probe = document.createElement('span');
        document.body.appendChild(probe);

        const resolve = (name) => {
            probe.style.color = `var(${name})`;
            return getComputedStyle(probe).color;
        };
        const tokens = {panel: resolve('--panel'), text: resolve('--text'), textMuted: resolve('--text-muted')};
        probe.remove();

        return {
            tokens,
            scheme: getComputedStyle(document.documentElement).colorScheme,
            launcher: getComputedStyle(document.getElementById('launcher')).backgroundColor,
            text: getComputedStyle(document.body).color,
            muted: getComputedStyle(document.getElementById('hints')).color,
        };
    });
}

test('in light mode the page renders with the light tokens', async ({page}) => {
    await shown(page, 'links');
    await expect(page.locator('#grid .section')).toHaveText(['Recent', 'Ecosystem', 'Docs']);

    const painted = await colours(page);

    // The light values of :root in style.css, as the browser computes them.
    expect(painted.scheme).toBe('light');
    expect(painted.tokens).toEqual({panel: 'rgba(246, 246, 248, 0.97)', text: 'rgb(29, 29, 31)', textMuted: 'rgb(92, 92, 96)'});

    // And the panel, the text and the hints are painted with them.
    expect(painted.launcher).toBe(painted.tokens.panel);
    expect(painted.text).toBe(painted.tokens.text);
    expect(painted.muted).toBe(painted.tokens.textMuted);
});
