import test from 'node:test';
import assert from 'node:assert/strict';
import {normalize, score, decorate, rankItems, filterByCategory, visibleUnder, HIDDEN, sections, unifiedSearch, pairSamePage, pageKey, FAVORITES} from '../src/filter.js';

const links = [
    {id: 'gitea', key: 'links:gitea', kind: 'link', name: 'Gitea', description: 'Repos en el servidor', host: 'repos.example.com', url: 'https://repos.example.com', category: 'eco', keywords: ['git']},
    {id: 'go-doc', key: 'links:go-doc', kind: 'link', name: 'Go', description: 'Documentación oficial de Go', host: 'go.dev', url: 'https://go.dev', category: 'docs', keywords: []},
    {id: 'github', key: 'links:github', kind: 'link', name: 'GitHub', description: 'Pull requests', host: 'github.com', url: 'https://github.com', category: 'eco', keywords: []},
];

const apps = [
    {id: 'app-mail', key: 'apps:app-mail', kind: 'app', source: 'applications', name: 'Mail', description: '', category: 'applications', keywords: []},
    {id: 'edge-github', key: 'apps:edge-github', kind: 'app', source: 'edge', name: 'GitHub', description: 'github.com', host: 'github.com', url: 'https://github.com/', category: 'edge', keywords: []},
];

test('normalize strips accents and case', () => {
    assert.equal(normalize('Cronómetro Música'), 'cronometro musica');
    assert.equal(normalize(undefined), '');
});

test('score ranks name prefix over word prefix over substring, and text fields at half', () => {
    assert.equal(score(links[0], 'git'), 3);
    assert.equal(score({...links[0], name: 'My Gitea'}, 'git'), 2);
    // links[0] keeps its keywords: ['git'], so the exact match in that
    // field also scores (a text field at half of NAME_PREFIX, 3 × 0.5 =
    // 1.5), which outranks the name's own substring hit (1).
    assert.equal(score({...links[0], name: 'Agitea'}, 'git'), 1.5);
    assert.equal(score(links[1], 'oficial'), 1, 'word prefix in a text field: 2 × 0.5');
    assert.equal(score(links[0], 'repos'), 1.5, 'prefix of the host or the description: 3 × 0.5');
    assert.equal(score(links[0], 'nothing'), 0);
    assert.equal(score(links[0], ''), 0);
});

test('every word of the query has to match somewhere, scores add up', () => {
    assert.equal(score(links[0], 'gitea servidor'), 4);
    assert.equal(score(links[0], 'gitea vps'), 0);
});

test('score also searches the id, like the old launcher did', () => {
    // Neither the name nor the description mentions "doc": only the id
    // does, so this proves the id itself is searched, not some other
    // field that happens to contain the query.
    const golang = {id: 'go-doc', key: 'links:go-doc', name: 'Golang', description: 'Programming language site', host: 'golang.org', keywords: []};

    assert.ok(score(golang, 'go-doc') > 0, 'the full id matches');
    assert.ok(score(golang, 'doc') > 0, 'a word inside the id matches');
});

test('decorate adds favourite, opens and lastOpened from the usage file', () => {
    const usage = {opens: {'links:gitea': 4}, lastOpened: {'links:gitea': '2026-09-26T10:00:00Z'}, favorites: ['links:go-doc']};
    const decorated = decorate(links, usage);

    assert.deepEqual(decorated.map((item) => [item.opens, item.favorite, item.lastOpened > 0]), [
        [4, false, true],
        [0, true, false],
        [0, false, false],
    ]);
    assert.equal(links[0].opens, undefined, 'the catalog must not be touched');
});

test('rankItems orders by score, then favourite, then recency, then use', () => {
    const decorated = decorate(links, {opens: {'links:github': 9, 'links:gitea': 1}, lastOpened: {'links:gitea': '2026-09-26T10:00:00Z'}, favorites: []});
    assert.deepEqual(rankItems(decorated, 'git').map((item) => item.id), ['gitea', 'github'], 'recency beats use on a tie; "go" does not contain "git"');
    assert.deepEqual(rankItems(decorated, 'g').map((item) => item.id), ['gitea', 'github', 'go-doc']);

    const favourite = decorate(links, {opens: {}, lastOpened: {}, favorites: ['links:github']});
    assert.deepEqual(rankItems(favourite, 'g').map((item) => item.id), ['github', 'gitea', 'go-doc']);
    assert.deepEqual(rankItems(favourite, 'zzz'), []);
});

test('filterByCategory keeps everything, the favourites, or one chip', () => {
    const decorated = decorate(links, {opens: {}, lastOpened: {}, favorites: ['links:go-doc']});
    assert.equal(filterByCategory(decorated, '').length, 3);
    assert.deepEqual(filterByCategory(decorated, FAVORITES).map((item) => item.id), ['go-doc']);
    assert.deepEqual(filterByCategory(decorated, 'eco').map((item) => item.id), ['gitea', 'github']);
});

test('sections split favourites, the five most recent and the rest by category', () => {
    const usage = {
        opens: {},
        lastOpened: {'links:gitea': '2026-09-26T10:00:00Z', 'links:github': '2026-09-25T10:00:00Z'},
        favorites: ['links:github'],
    };
    const categories = [{id: 'eco', name: 'Eco'}, {id: 'docs', name: 'Docs'}];
    const result = sections(decorate(links, usage), categories, 5);

    assert.deepEqual(result.map((section) => [section.id, section.items.map((item) => item.id)]), [
        ['favorites', ['github']],
        ['recent', ['gitea']],
        ['docs', ['go-doc']],
    ]);
});

test('sections without favourites or recents is just the categories, hidden items left out', () => {
    const result = sections(decorate([...links, {...apps[0], hidden: true}], {opens: {}, lastOpened: {}, favorites: []}), [{id: 'eco', name: 'Eco'}, {id: 'docs', name: 'Docs'}, {id: 'applications', name: 'Apps'}], 5);
    assert.deepEqual(result.map((section) => section.id), ['eco', 'docs']);
});

test('sections lists the most opened item first inside each category, ties keep the library order', () => {
    // The "eco" category holds gitea (library order 0) then github
    // (library order 1); opens 0 and 3 should flip that to github first.
    const usage = {opens: {'links:github': 3}, lastOpened: {}, favorites: []};
    const categories = [{id: 'eco', name: 'Eco'}, {id: 'docs', name: 'Docs'}];
    const opened = sections(decorate(links, usage), categories, 5);
    assert.deepEqual(opened.find((section) => section.id === 'eco').items.map((item) => item.id), ['github', 'gitea']);

    // Equal opens (both never opened) must not reorder the category: the
    // tie-break is the library order, which the stable sort preserves.
    const untouched = sections(decorate(links, {opens: {}, lastOpened: {}, favorites: []}), categories, 5);
    assert.deepEqual(untouched.find((section) => section.id === 'eco').items.map((item) => item.id), ['gitea', 'github']);
});

test('unifiedSearch mixes both tabs by score and pairs an Edge app with its link', () => {
    const usage = {opens: {}, lastOpened: {}, favorites: []};
    const rows = unifiedSearch(decorate(apps, usage), decorate(links, usage), 'git');

    assert.deepEqual(rows.map((row) => row.key), ['links:gitea', 'apps:edge-github']);
    assert.equal(rows[1].web.id, 'github', 'the link rides along on the Edge app');
});

test('pairSamePage leaves unpaired items alone', () => {
    const rows = pairSamePage([apps[0], links[0]], 'x');
    assert.equal(rows.length, 2);
    assert.equal(rows[0].web, undefined);
});

test('pageKey normalises like Go: case, www, default port, trailing slash, fragment', () => {
    assert.equal(pageKey('HTTPS://WWW.Example.org:443/Docs/#top'), 'https://example.org/Docs');
    assert.equal(pageKey('https://example.org/'), 'https://example.org');
    assert.equal(pageKey('http://example.org:8080/x?y=1'), 'http://example.org:8080/x?y=1');
    assert.equal(pageKey('not a url'), '');
    assert.equal(pageKey(undefined), '');
});

// The owner's case: a web app on one path of a host must not swallow the
// link to the host's front page.
const wayApp = {id: 'edge-way', key: 'apps:edge-way', kind: 'app', source: 'edge', name: 'Go On The Way', description: 'home.example.org', host: 'home.example.org', url: 'https://home.example.org/way/', category: 'edge', keywords: []};
const homeLink = {id: 'homepage', key: 'links:homepage', kind: 'link', name: 'Homepage', description: 'All the services', host: 'home.example.org', url: 'https://home.example.org', category: 'eco', keywords: []};
const noUsage = {opens: {}, lastOpened: {}, favorites: []};

test('same host, different page: two rows, nothing folded', () => {
    const rows = unifiedSearch(decorate([wayApp], noUsage), decorate([homeLink], noUsage), 'home');

    assert.deepEqual(rows.map((row) => row.key).sort(), ['apps:edge-way', 'links:homepage']);
    assert.ok(rows.every((row) => row.web === undefined));
});

test('same page: one row, the link rides on the app', () => {
    const sameApp = {...wayApp, id: 'edge-home', key: 'apps:edge-home', name: 'Homepage', url: 'https://www.home.example.org/'};
    const rows = unifiedSearch(decorate([sameApp], noUsage), decorate([homeLink], noUsage), 'home');

    assert.deepEqual(rows.map((row) => row.key), ['apps:edge-home']);
    assert.equal(rows[0].web.id, 'homepage');
});

test('a link that matches better than its app keeps its own row', () => {
    const code = {...apps[1], id: 'edge-code', key: 'apps:edge-code', name: 'Code'};
    const rows = unifiedSearch(decorate([code], noUsage), decorate([links[2]], noUsage), 'git');

    assert.deepEqual(rows.map((row) => row.key), ['links:github', 'apps:edge-code']);
    assert.ok(rows.every((row) => row.web === undefined));
});

test('visibleUnder shows only the hidden apps under their chip, and never elsewhere', () => {
    const items = [{...apps[0], hidden: true}, apps[1]];

    assert.deepEqual(visibleUnder(items, HIDDEN).map((item) => item.id), ['app-mail']);
    assert.deepEqual(visibleUnder(items, 'applications'), []);
    assert.deepEqual(visibleUnder(items, 'edge').map((item) => item.id), ['edge-github']);
});

// A search-only app: found by typing, never on the grid or under a chip.
const calculator = {id: 'app-calculator', key: 'apps:app-calculator', kind: 'app', source: 'applications', name: 'Calculator', description: '', category: 'applications', keywords: [], searchOnly: true};

test('a search-only app is found by typing and nowhere else', () => {
    const items = decorate([calculator, apps[0]], {opens: {}, lastOpened: {}, favorites: ['apps:app-calculator']});

    assert.deepEqual(unifiedSearch(items, [], 'calc').map((row) => row.id), ['app-calculator']);
    assert.deepEqual(filterByCategory(items, 'applications').map((item) => item.id), ['app-mail']);
    assert.deepEqual(filterByCategory(items, FAVORITES), [], 'not even as a favourite');

    const grid = sections(items, [{id: 'applications', name: 'Applications'}], 5);
    assert.deepEqual(grid.flatMap((section) => section.items.map((item) => item.id)), ['app-mail']);
});
