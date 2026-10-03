import {defineConfig, devices} from '@playwright/test';

// The page is a WKWebView in the real app, so WebKit is the browser that
// matters; the harness is started once by the global setup on a free port.
// Two workers, because Playwright's WebKit stops loading pages after 63 in
// one browser: the 64th page.goto never sends its request. Each worker
// has its own browser, and keeps a whole spec file.
export default defineConfig({
    testDir: './test/e2e',
    globalSetup: './test/e2e/global-setup.js',
    timeout: 15000,
    fullyParallel: false,
    workers: 2,
    reporter: process.env.CI ? 'github' : 'list',
    use: {
        baseURL: process.env.HARNESS_URL,
        viewport: {width: 760, height: 520},
        colorScheme: 'dark',
    },
    projects: [
        {name: 'webkit', use: {...devices['Desktop Safari']}},
    ],
});
