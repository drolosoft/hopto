import test from 'node:test';
import assert from 'node:assert/strict';
import {looksLikeURL, withScheme, hostOf, addOffer, newDraft, editDraft, withInput, withInspection, pickCategory, moveCategory, localProblems, canSave, linkInput, withSaveResult, problemText, withPick, adoptDraft, appInput, isDiscoveredID} from '../src/draft.js';
import {translator} from '../src/i18n.js';

const categories = [{id: 'eco', name: 'Ecosystem'}, {id: 'docs', name: 'Docs'}];

test('looksLikeURL takes a scheme or anything with a dot and no spaces', () => {
    const cases = {
        'https://example.org': true,
        'HTTP://EXAMPLE.ORG': true,
        'example.org': true,
        'nas.local:5000/admin': true,
        '100.64.0.1': true,
        'example.org/docs/v1.2': true,
        'Grafana': false,
        'two words.org': false,
        '.hidden': false,
        'end.': false,
        'a..b': false,
        '': false,
        '   ': false,
    };

    for (const [text, expected] of Object.entries(cases)) {
        assert.equal(looksLikeURL(text), expected, text);
    }
});

test('withScheme adds https:// only when there is no scheme', () => {
    assert.equal(withScheme('example.org'), 'https://example.org');
    assert.equal(withScheme(' example.org/x '), 'https://example.org/x');
    assert.equal(withScheme('http://nas.local'), 'http://nas.local');
    assert.equal(withScheme('ftp://files.example.org'), 'ftp://files.example.org');
    assert.equal(withScheme(''), '');
    assert.equal(hostOf('https://example.org:8443/x'), 'example.org');
    assert.equal(hostOf('not a url'), '');
});

test('addOffer: always with no results, only for addresses when there are some', () => {
    assert.equal(addOffer('', 0), null);
    assert.equal(addOffer('  ', 0), null);
    assert.deepEqual(addOffer(' grafana ', 0), {text: 'grafana', url: false});
    assert.equal(addOffer('grafana', 2), null);
    assert.deepEqual(addOffer('github.com', 2), {text: 'github.com', url: true});
});

test('newDraft: an address goes to the URL, a word becomes the name', () => {
    const fromURL = newDraft({tab: 'links', text: 'example.org', category: 'docs', categories});
    assert.equal(fromURL.url, 'https://example.org');
    assert.equal(fromURL.name, '');
    assert.equal(fromURL.category, 'docs', 'the active chip is preselected');
    assert.equal(fromURL.newCategory, null);
    assert.deepEqual(fromURL.touched, []);
    assert.equal(fromURL.returnQuery, 'example.org');

    const fromName = newDraft({tab: 'links', text: 'Grafana', category: 'favorites', categories});
    assert.equal(fromName.url, '');
    assert.equal(fromName.name, 'Grafana');
    assert.equal(fromName.category, 'eco', 'a virtual chip falls back to the first category');
    assert.deepEqual(fromName.touched, ['name']);

    const empty = newDraft({tab: 'links', categories: [], returnQuery: 'old'});
    assert.equal(empty.newCategory, '', 'no categories: the new-category field opens');
    assert.equal(empty.returnQuery, 'old');
});

test('withInput clears the field problem and, for the URL, what the old URL said', () => {
    const draft = {...newDraft({text: 'example.org', categories}), problems: {url: 'url.invalid', name: 'name.required'}, duplicate: {id: 'x'}, inspecting: true};
    const next = withInput(draft, 'url', 'http://example.org/y');

    assert.deepEqual(next.problems, {name: 'name.required'});
    assert.equal(next.duplicate, null);
    assert.equal(next.inspecting, false);
    assert.equal(next.insecure, true);
    assert.deepEqual(next.touched, ['url']);
    assert.equal(draft.url, 'https://example.org', 'the old draft is not mutated');

    const typed = withInput({...draft, newCategory: '', problems: {category: 'category.name'}}, 'newCategory', 'Tools');
    assert.equal(typed.newCategory, 'Tools');
    assert.deepEqual(typed.problems, {});
});

test('withInspection ignores an answer for another URL', () => {
    const draft = withInput(newDraft({text: 'example.org', categories}), 'url', 'https://other.example.org');
    const stale = withInspection(draft, {url: 'https://example.org', name: 'Old', description: '', iconDataUrl: '', insecure: false, duplicate: null, sameHost: []});
    assert.equal(stale, draft);
});

test('a typed name survives the inspection; untouched fields are filled', () => {
    const draft = withInput(newDraft({text: 'example.org', categories}), 'name', 'Mine');
    const info = {url: 'https://example.org', name: 'Example Domain', description: 'Illustrative', iconDataUrl: 'data:image/png;base64,AA', insecure: false, duplicate: {tab: 'links', id: 'ex', name: 'Ex'}, sameHost: [{tab: 'links', id: 'ex2', name: 'Ex 2'}]};
    const next = withInspection({...draft, inspecting: true}, info);

    assert.equal(next.name, 'Mine');
    assert.equal(next.description, 'Illustrative');
    assert.equal(next.iconDataUrl, 'data:image/png;base64,AA');
    assert.equal(next.duplicate.id, 'ex');
    assert.equal(next.sameHost.length, 1);
    assert.equal(next.inspecting, false);
});

test('in edit mode the link itself is neither a duplicate nor a neighbour', () => {
    const draft = {...newDraft({text: 'example.org', categories}), mode: 'edit', id: 'ex'};
    const info = {url: 'https://example.org', name: '', description: '', iconDataUrl: '', insecure: false, duplicate: {tab: 'links', id: 'ex', name: 'Ex'}, sameHost: [{tab: 'links', id: 'ex', name: 'Ex'}]};
    const next = withInspection(draft, info);

    assert.equal(next.duplicate, null);
    assert.deepEqual(next.sameHost, []);
});

test('pickCategory picks a chip or opens the new-category field; moveCategory wraps', () => {
    const draft = newDraft({text: 'example.org', categories});

    assert.equal(pickCategory(draft, categories, 1).category, 'docs');
    assert.equal(pickCategory(draft, categories, 2).newCategory, '');
    assert.equal(pickCategory(draft, categories, 7), draft, 'past the chips: nothing');

    const onNew = pickCategory(draft, categories, 2);
    assert.equal(pickCategory(onNew, categories, 0).newCategory, null, 'a real chip closes the field');

    assert.equal(moveCategory(draft, categories, 1).category, 'docs');
    assert.equal(moveCategory(draft, categories, -1).newCategory, '', 'left from the first chip lands on "new"');
    assert.equal(moveCategory(onNew, categories, 1).category, 'eco', 'right from "new" wraps to the first');
});

test('localProblems and canSave: an address is needed and a duplicate blocks', () => {
    const empty = newDraft({text: 'Grafana', categories});
    assert.deepEqual(localProblems(empty), {url: 'url.invalid'});
    assert.equal(canSave(empty), false);

    const ready = withInput(empty, 'url', 'grafana.example.org');
    assert.deepEqual(localProblems(ready), {});
    assert.equal(canSave(ready), true);
    assert.equal(canSave({...ready, duplicate: {id: 'x'}}), false);
    assert.equal(canSave({...ready, saving: true}), false);

    const unnamed = {...ready, newCategory: '  '};
    assert.deepEqual(localProblems(unnamed), {category: 'category.name'});
});

test('linkInput trims, adds the scheme and names an unnamed link after its host', () => {
    const draft = {...newDraft({text: 'example.org/x', categories}), description: '  About  ', keywords: ['k']};
    assert.deepEqual(linkInput(draft), {url: 'https://example.org/x', name: 'example.org', description: 'About', category: 'eco', keywords: ['k'], icon: ''});
});

test('withSaveResult: an id closes, problems stay on the draft', () => {
    const draft = {...newDraft({text: 'example.org', categories}), saving: true};

    const saved = withSaveResult(draft, {id: 'example', problems: {}, duplicate: null});
    assert.equal(saved.savedId, 'example');

    const refused = withSaveResult(draft, {id: '', problems: {name: 'name.long'}, duplicate: null});
    assert.equal(refused.savedId, '');
    assert.deepEqual(refused.draft.problems, {name: 'name.long'});
    assert.equal(refused.draft.saving, false);

    const twin = withSaveResult(draft, {id: '', problems: {}, duplicate: {tab: 'links', id: 'ex', name: 'Ex'}});
    assert.equal(twin.draft.duplicate.id, 'ex');
});

test('editDraft fills the editor from a list entry and keeps its keywords', () => {
    const entry = {id: 'gitea', key: 'links:gitea', kind: 'link', name: 'Gitea', description: 'Repos', url: 'https://repos.example.com', path: '', bundleId: '', category: 'docs', keywords: ['git'], iconUrl: '/user-icons/gitea.png?v=1'};
    const draft = editDraft(entry, categories, 'git');

    assert.equal(draft.mode, 'edit');
    assert.equal(draft.tab, 'links');
    assert.equal(draft.id, 'gitea');
    assert.equal(draft.url, 'https://repos.example.com');
    assert.equal(draft.category, 'docs');
    assert.deepEqual(draft.keywords, ['git']);
    assert.notEqual(draft.keywords, entry.keywords, 'a copy, not the list\'s array');
    assert.equal(draft.iconDataUrl, '/user-icons/gitea.png?v=1');
    assert.deepEqual(draft.touched, ['url', 'name', 'description'], 'an inspection never overwrites what is saved');
    assert.equal(draft.returnQuery, 'git');

    const app = editDraft({id: 'mine', kind: 'app', name: 'Mine', description: '', path: '/Applications/Mine.app', bundleId: 'org.example.mine', category: 'tools', keywords: []}, [{id: 'tools', name: 'Tools'}]);
    assert.equal(app.tab, 'apps');
    assert.equal(app.path, '/Applications/Mine.app');
    assert.equal(app.bundleId, 'org.example.mine');
});

test('problemText translates a key and falls back for an unknown one', () => {
    const t = translator('en');
    assert.equal(problemText(t, 'name.required'), 'The name is required');
    assert.equal(problemText(t, 'something.new'), 'Check this field');
    assert.equal(problemText(t, ''), '');
});

const appCategories = [{id: 'tools', name: 'Tools'}];
const picked = {path: '/Applications/Example.app', bundleId: 'org.example.app', name: 'Example', description: '', iconDataUrl: 'data:image/png;base64,AA', problem: '', duplicate: null};

test('withPick fills an app draft; a cancel changes nothing', () => {
    const draft = newDraft({tab: 'apps', categories: appCategories});
    assert.equal(draft.twin, null);
    assert.equal(withPick(draft, {...picked, path: ''}), draft);

    const next = withPick(draft, picked);
    assert.equal(next.path, '/Applications/Example.app');
    assert.equal(next.bundleId, 'org.example.app');
    assert.equal(next.name, 'Example');
    assert.equal(next.iconDataUrl, 'data:image/png;base64,AA');
    assert.deepEqual(localProblems(next), {});
});

test('withPick keeps a typed name, and a refused path stays refused', () => {
    const typed = withInput(newDraft({tab: 'apps', categories: appCategories}), 'name', 'Mine');
    const next = withPick(typed, {...picked, path: '/Users/someone/Downloads/Loose.app', problem: 'app.path'});

    assert.equal(next.name, 'Mine');
    assert.deepEqual(next.problems, {path: 'app.path'});
    assert.deepEqual(localProblems(next), {path: 'app.path'});
    assert.equal(canSave(next), false);
    assert.deepEqual(withPick(next, picked).problems, {}, 'a good pick clears it');
});

test('a hand-added duplicate blocks, a discovered one is a twin that only warns', () => {
    const draft = newDraft({tab: 'apps', categories: appCategories});

    const blocked = withPick(draft, {...picked, duplicate: {tab: 'apps', id: 'example', name: 'Example'}});
    assert.equal(blocked.duplicate.id, 'example');
    assert.equal(canSave(blocked), false);

    const twin = withPick(draft, {...picked, duplicate: {tab: 'apps', id: 'app-example', name: 'Example'}});
    assert.equal(twin.duplicate, null);
    assert.equal(twin.twin.id, 'app-example');
    assert.equal(canSave(twin), true);

    assert.equal(isDiscoveredID('edge-x'), true);
    assert.equal(isDiscoveredID('mine'), false);
    assert.equal(isDiscoveredID(undefined), false);
});

test('an app draft needs a .app; appInput names it after the bundle when unnamed', () => {
    const empty = newDraft({tab: 'apps', categories: appCategories});
    assert.deepEqual(localProblems(empty), {path: 'app.target'});

    const unnamed = {...withPick(empty, picked), name: '  ', description: ' Tools '};
    assert.deepEqual(appInput(unnamed), {path: '/Applications/Example.app', bundleId: 'org.example.app', name: 'Example', description: 'Tools', category: 'tools'});
});

test('adoptDraft adds a discovered app by hand, pointing at itself as the twin', () => {
    const mail = {id: 'app-mail', key: 'apps:app-mail', kind: 'app', source: 'applications', name: 'Mail', description: '', path: '/Applications/Mail.app', bundleId: 'com.apple.mail', category: 'applications', keywords: [], iconUrl: '/user-icons/app-mail.png?v=1'};
    const draft = adoptDraft(mail, appCategories, 'ma');

    assert.equal(draft.mode, 'add');
    assert.equal(draft.tab, 'apps');
    assert.equal(draft.path, '/Applications/Mail.app');
    assert.equal(draft.bundleId, 'com.apple.mail');
    assert.equal(draft.category, 'tools', 'a virtual category is not a place to add to');
    assert.deepEqual(draft.twin, {tab: 'apps', id: 'app-mail', name: 'Mail'});
    assert.equal(draft.returnQuery, 'ma');
});
