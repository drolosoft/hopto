import test from 'node:test';
import assert from 'node:assert/strict';
import {normalize, matches, filterItems, decorate, sortByUse, FAVORITES} from '../src/filter.js';

const items = [
    {id: 'cronometro', name: 'Cronómetro', description: 'Cuenta atrás de 1:05 con semáforo', category: 'utilidades'},
    {id: 'gitea', name: 'Gitea', description: 'Los repos, en el servidor', host: 'repos.example.com', category: 'ecosistema'},
    {id: 'go-doc', name: 'Go', description: 'Documentación oficial de Go', host: 'go.dev', category: 'documentacion'},
];

test('normalize strips accents and case', () => {
    assert.equal(normalize('Cronómetro Música'), 'cronometro musica');
    assert.equal(normalize(undefined), '');
});

test('an empty query matches everything', () => {
    assert.ok(items.every((item) => matches(item, '')));
    assert.ok(items.every((item) => matches(item, '   ')));
});

test('matches ignores accents and searches name, description and host', () => {
    assert.ok(matches(items[0], 'cronometro'));
    assert.ok(matches(items[0], 'semaforo'));
    assert.ok(matches(items[1], 'repos.example'));
    assert.ok(!matches(items[1], 'cronometro'));
});

test('every word of the query has to match', () => {
    assert.ok(matches(items[1], 'gitea servidor'));
    assert.ok(!matches(items[1], 'gitea vps'));
});

test('filterItems combines category and query', () => {
    assert.deepEqual(filterItems(items, '', '').map((item) => item.id), ['cronometro', 'gitea', 'go-doc']);
    assert.deepEqual(filterItems(items, 'documentacion', '').map((item) => item.id), ['go-doc']);
    assert.deepEqual(filterItems(items, 'documentacion', 'gitea'), []);
    assert.deepEqual(filterItems(items, '', 'go').map((item) => item.id), ['go-doc']);
});

test('decorate adds key, opens and favorite from the usage file', () => {
    const usage = {opens: {'links:gitea': 4}, favorites: ['links:go-doc']};
    const decorated = decorate(items, 'links', usage);

    assert.deepEqual(decorated.map((item) => [item.key, item.opens, item.favorite]), [
        ['links:cronometro', 0, false],
        ['links:gitea', 4, false],
        ['links:go-doc', 0, true],
    ]);
    assert.equal(items[1].opens, undefined, 'the catalog must not be touched');
});

test('sortByUse puts the most opened first and keeps catalog order otherwise', () => {
    const decorated = decorate(items, 'links', {opens: {'links:go-doc': 2, 'links:gitea': 2, 'links:cronometro': 1}, favorites: []});
    assert.deepEqual(sortByUse(decorated).map((item) => item.id), ['gitea', 'go-doc', 'cronometro']);
});

test('the favourites chip filters on the flag and combines with the query', () => {
    const decorated = decorate(items, 'links', {opens: {}, favorites: ['links:gitea', 'links:go-doc']});
    assert.deepEqual(filterItems(decorated, FAVORITES, '').map((item) => item.id), ['gitea', 'go-doc']);
    assert.deepEqual(filterItems(decorated, FAVORITES, 'repos').map((item) => item.id), ['gitea']);
});
