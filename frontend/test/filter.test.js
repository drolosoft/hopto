import test from 'node:test';
import assert from 'node:assert/strict';
import {normalize, score, decorate, rankItems, filterByCategory, sections, unifiedSearch, pairByHost, FAVORITES} from '../src/filter.js';

const links = [
    {id: 'gitea', key: 'links:gitea', kind: 'link', name: 'Gitea', description: 'Repos en el servidor', host: 'repos.example.com', category: 'eco', keywords: ['git']},
    {id: 'go-doc', key: 'links:go-doc', kind: 'link', name: 'Go', description: 'Documentación oficial de Go', host: 'go.dev', category: 'docs', keywords: []},
    {id: 'github', key: 'links:github', kind: 'link', name: 'GitHub', description: 'Pull requests', host: 'github.com', category: 'eco', keywords: []},
];

const apps = [
    {id: 'app-mail', key: 'apps:app-mail', kind: 'app', source: 'applications', name: 'Mail', description: '', category: 'applications', keywords: []},
    {id: 'edge-github', key: 'apps:edge-github', kind: 'app', source: 'edge', name: 'GitHub', description: 'github.com', host: 'github.com', category: 'edge', keywords: []},
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

test('unifiedSearch mixes both tabs by score and pairs an Edge app with its link', () => {
    const usage = {opens: {}, lastOpened: {}, favorites: []};
    const rows = unifiedSearch(decorate(apps, usage), decorate(links, usage), 'git');

    assert.deepEqual(rows.map((row) => row.key), ['links:gitea', 'apps:edge-github']);
    assert.equal(rows[1].web.id, 'github', 'the link rides along on the Edge app');
});

test('pairByHost leaves unpaired items alone', () => {
    const rows = pairByHost([apps[0], links[0]]);
    assert.equal(rows.length, 2);
    assert.equal(rows[0].web, undefined);
});
