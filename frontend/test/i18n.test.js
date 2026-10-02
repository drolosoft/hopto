import test from 'node:test';
import assert from 'node:assert/strict';
import {resolveLanguage, translator, MESSAGES} from '../src/i18n.js';

test('the setting wins, auto follows the browser, unknown falls back to English', () => {
    assert.equal(resolveLanguage('es', 'en-US'), 'es');
    assert.equal(resolveLanguage('en', 'es-ES'), 'en');
    assert.equal(resolveLanguage('auto', 'es-ES'), 'es');
    assert.equal(resolveLanguage('auto', 'en-GB'), 'en');
    assert.equal(resolveLanguage('auto', undefined), 'en');
    assert.equal(resolveLanguage('fr', 'fr-FR'), 'en');
});

test('t substitutes parameters and falls back to English for a missing key', () => {
    const t = translator('es');
    assert.equal(t('empty.elsewhere', {count: 3, tab: 'Mis links'}), 'Nada aquí · 3 en Mis links (⇥)');
    assert.equal(t.language, 'es');
    assert.equal(translator('en')('empty.elsewhere', {count: 3, tab: 'Links'}), 'Nothing here · 3 in Links (⇥)');
    assert.equal(t('no.such.key'), 'no.such.key');
});

test('plural picks one or other', () => {
    const t = translator('en');
    assert.equal(t.plural('apps', 1), '1 app');
    assert.equal(t.plural('apps', 4), '4 apps');
    assert.equal(translator('es').plural('links', 0), '0 links');
});

test('both dictionaries have the same keys', () => {
    assert.deepEqual(Object.keys(MESSAGES.es).sort(), Object.keys(MESSAGES.en).sort());
});

test('the translator fills the modifier placeholders for the platform', () => {
    assert.equal(translator('en')('footer.pair'), '↩ app · ⌘↩ web');
    assert.equal(translator('en', 'windows')('footer.pair'), 'Enter app · Ctrl+Enter web');
    assert.equal(translator('es', 'windows')('help.edit').startsWith('Ctrl+N nuevo'), true);
});
