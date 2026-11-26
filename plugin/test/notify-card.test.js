'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');
const fs = require('node:fs');

const PLUGIN = path.join(__dirname, '..', 'src', 'shiplog', 'usr', 'local', 'emhttp', 'plugins', 'shiplog');
const card = require(path.join(PLUGIN, 'scripts', 'notify-card.js'));
const marks = require(path.join(PLUGIN, 'scripts', 'notify-marks.js'));
const en = JSON.parse(fs.readFileSync(path.join(PLUGIN, 'lang', 'en.json'), 'utf8'));
const KINDS = ['unraid', 'discord', 'slack', 'ntfy', 'gotify', 'matrix', 'apprise', 'smtp', 'webhook'];

// Just enough of a document for the card and its window: elements with
// children, attributes, values, focus and events, found again by their class,
// their kind or a plain selector.
function dom() {
  const doc = { activeElement: null, listeners: {} };
  doc.addEventListener = (type, fn) => (doc.listeners[type] = doc.listeners[type] || []).push(fn);
  doc.removeEventListener = (type, fn) => { doc.listeners[type] = (doc.listeners[type] || []).filter((f) => f !== fn); };
  doc.key = (key, target) => {
    const e = { key, target: target || doc.activeElement, shiftKey: false, used: false, preventDefault() { this.used = true; }, stopPropagation() {} };
    for (const fn of doc.listeners.keydown || []) fn(e);
    return e;
  };
  const matches = (n, sel) => sel.split(',').some((one) => {
    const m = /^([a-z]*)((?:\.[\w-]+)*)(?:\[([\w-]+)="([^"]*)"\])?$/.exec(one.trim());
    if (!m) throw new Error('selector ' + one);
    const [, tag, classes, attr, value] = m;
    if (tag && n.tagName !== tag.toUpperCase()) return false;
    if (classes && !classes.slice(1).split('.').every((c) => (' ' + n.className + ' ').includes(' ' + c + ' '))) return false;
    return !attr || n.getAttribute(attr) === value;
  });
  class El {
    constructor(tag) { this.tagName = tag.toUpperCase(); this.children = []; this.attrs = {}; this.listeners = {}; this.value = ''; this.className = ''; this.text = ''; this.disabled = false; this.checked = false; this.style = {}; this.clientHeight = 0; this.offsetParent = {}; }
    get id() { return this.attrs.id || ''; }
    get isConnected() { let n = this; while (n.parent) n = n.parent; return n === doc.body || n === doc.root; }
    appendChild(c) { this.children.push(c); c.parent = this; return c; }
    removeChild(c) { this.children = this.children.filter((x) => x !== c); c.parent = null; return c; }
    get parentNode() { return this.parent || null; }
    setAttribute(k, v) { this.attrs[k] = String(v); }
    getAttribute(k) { return k in this.attrs ? this.attrs[k] : null; }
    addEventListener(type, fn) { (this.listeners[type] = this.listeners[type] || []).push(fn); }
    fire(type, e) { const ev = Object.assign({ target: this, preventDefault() {} }, e); for (const fn of this.listeners[type] || []) fn(ev); if (this['on' + type]) this['on' + type](ev); }
    get classList() { const n = this; const list = () => n.className.split(' ').filter(Boolean); return { add: (...c) => { n.className = [...new Set([...list(), ...c])].join(' '); }, remove: (...c) => { n.className = list().filter((x) => !c.includes(x)).join(' '); }, contains: (c) => list().includes(c) }; }
    removeAttribute(k) { delete this.attrs[k]; }
    click() { if (!this.disabled) this.fire('click'); }
    focus() { doc.activeElement = this; }
    getBoundingClientRect() { return { left: 0, top: 0, width: 10, height: 10 }; }
    set textContent(v) { this.text = String(v); this.children = []; }
    get textContent() { return this.text + this.children.map((c) => c.textContent).join(''); }
    set innerHTML(v) { this.html = v; }
    all(pred, out = []) { for (const c of this.children) { if (pred(c)) out.push(c); c.all(pred, out); } return out; }
    byClass(name) { return this.all((n) => (' ' + n.className + ' ').includes(' ' + name + ' ')); }
    querySelectorAll(sel) { return this.all((n) => matches(n, sel)); }
    querySelector(sel) { return this.querySelectorAll(sel)[0] || null; }
  }
  doc.createElement = (tag) => new El(tag);
  doc.getElementById = (id) => doc.body.all((n) => n.id === id)[0] || null;
  doc.body = new El('body');
  doc.root = new El('div');
  doc.root.ownerDocument = doc;
  return doc;
}

const settle = async () => { for (let i = 0; i < 20; i++) await new Promise((r) => setImmediate(r)); };

function mounted(channels, answers = {}) {
  const doc = dom();
  const el = doc.root;
  const saves = [], tests = [], saved = [];
  const adapter = {
    load: () => Promise.resolve(JSON.parse(JSON.stringify(channels))),
    save: (list) => { saves.push(list); return answers.save ? answers.save(list) : Promise.resolve(JSON.parse(JSON.stringify(list))); },
    test: (c) => { tests.push(c); return Promise.resolve(answers.test || { ok: true }); }
  };
  const view = card.mount(el, adapter, { t: en, lang: 'en', kinds: KINDS, marks, classes: { button: 'sl-btn' }, saved: (ok, text) => saved.push([ok, text]) });
  const win = () => doc.body.byClass('glim-nc-win')[0] || null;
  const foot = (label) => win().byClass('glim-nc-win-btn').find((b) => b.textContent === label);
  const tile = (kind) => win().byClass('glim-nc-tile').find((b) => b.attrs['data-kind'] === kind);
  const open = () => el.byClass('glim-nc-add')[0].click();
  return { doc, el, view, saves, tests, saved, win, foot, tile, open };
}

test('the tiles show the services, then mail and webhooks, each in the order of their names', () => {
  const s = card.sections(KINDS, [], en, 'en');
  assert.deepEqual(s.map((x) => x.title), [en.nc_services, en.nc_standards]);
  assert.deepEqual(s[0].tiles.map((x) => x.name), ['Apprise', 'Discord', 'Gotify', 'Matrix', 'ntfy', 'Slack', 'Unraid']);
  assert.deepEqual(s[1].tiles.map((x) => x.name), [en.nc_smtp, en.nc_webhook]);
  assert.ok(s[0].tiles.every((x) => x.hint), 'every service explains itself');
  assert.equal(card.sections(KINDS, [{ id: 'unraid', kind: 'unraid' }], en, 'en')[0].tiles.find((x) => x.kind === 'unraid').off, true, 'Unraid once');
});

test('a search narrows the tiles by name or kind, accents and hyphens aside', () => {
  const names = (q) => card.sections(KINDS, [], en, 'en', q).flatMap((s) => s.tiles.map((x) => x.kind));
  assert.deepEqual(names('disc'), ['discord']);
  assert.deepEqual(names('e-mail'), ['smtp']);
  assert.deepEqual(names('SMTP'), ['smtp']);
  assert.deepEqual(names('nothing like it'), []);
  assert.equal(names('').length, KINDS.length);
});

// GlimStone's notification catalog at the release glimstone.json follows,
// read from its source: one entry to a line, the Shoutrrr words in a list.
async function glimstoneCatalog() {
  const { version } = JSON.parse(fs.readFileSync(path.join(__dirname, '..', '..', 'glimstone.json'), 'utf8')).targets.find((t) => t.name === 'plugin');
  const url = `https://raw.githubusercontent.com/junkerderprovinz/glimstone/v${version}/packages/core/src/notifyProviders.ts`;
  const res = await fetch(url);
  assert.ok(res.ok, `${url} answered ${res.status}`);
  const src = await res.text();
  const words = (list) => [...list.matchAll(/"([^"]+)"/g)].map((m) => m[1]);
  const named = { SHOUTRRR: words(/const SHOUTRRR = \[([^\]]*)\]/.exec(src)[1]) };
  return [...src.matchAll(/^\s*\{ id: "([^"]+)".*\},?$/gm)].map(([line, id]) => {
    const e = { id, name: /name: "([^"]+)"/.exec(line)[1], group: /group: "([^"]+)"/.exec(line)[1] };
    const kind = /kind: "([^"]+)"/.exec(line), glyph = /glyph: "([^"]+)"/.exec(line), terms = /terms: (\[[^\]]*\]|\w+)/.exec(line);
    if (kind) e.kind = kind[1];
    if (terms) e.terms = terms[1].startsWith('[') ? words(terms[1]) : named[terms[1]];
    if (glyph) e.glyph = glyph[1];
    if (/word: true/.test(line)) e.word = true;
    return e;
  });
}

test("the card's catalog is GlimStone's, marks aside", async () => {
  const want = await glimstoneCatalog();
  assert.ok(want.length >= 10, 'the catalog was read');
  assert.deepEqual(card.CATALOG.groups, ['service', 'standard']);
  assert.deepEqual(card.CATALOG.entries, want);
});

test('every channel kind finds the entry it had before in the catalog', () => {
  const before = {
    unraid: ['Unraid', 'service'], discord: ['Discord', 'service'], slack: ['Slack', 'service'], ntfy: ['ntfy', 'service'],
    gotify: ['Gotify', 'service'], matrix: ['Matrix', 'service'], apprise: ['Apprise', 'service'], healthchecks: ['Healthchecks.io', 'service'],
    smtp: [en.nc_smtp, 'standard'], webhook: [en.nc_webhook, 'standard']
  };
  assert.deepEqual(Object.keys(card.FIELDS).sort(), Object.keys(before).sort());
  const all = Object.keys(before);
  const tiles = card.sections(all, [], en, 'en').flatMap((s) => s.tiles.map((x) => [x.kind, x.name, s.id]));
  assert.deepEqual(tiles.map((x) => x[0]).sort(), all.slice().sort(), 'no kind is left out');
  for (const [kind, name, group] of tiles) {
    assert.deepEqual([name, group], before[kind], kind);
    assert.equal(card.name(kind, en), name, kind);
    assert.equal(card.CATALOG.entries.find((e) => e.id === kind).word === true, group === 'standard', kind);
  }
});

test("the catalog's second line and search words reach the tiles", () => {
  const tiles = card.sections(KINDS, [], en, 'en').flatMap((s) => s.tiles);
  assert.equal(tiles.find((x) => x.kind === 'smtp').sub, 'SMTP');
  assert.ok(tiles.filter((x) => x.kind !== 'smtp').every((x) => x.sub === ''), 'only mail has a second line');
  const found = (q) => card.sections(KINDS, [], en, 'en', q).flatMap((s) => s.tiles.map((x) => x.kind));
  assert.deepEqual(found('element'), ['matrix']);
  assert.deepEqual(found('http'), ['webhook']);
  assert.deepEqual(found('telegram'), [], 'Shoutrrr is no channel of the card');
  const early = Object.assign({}, en, { nc_webhook: 'Aaa' });
  assert.deepEqual(card.sections(KINDS, [], early, 'en')[1].tiles.map((x) => x.kind), ['smtp', 'webhook'], 'the webhook, which takes anything, stays last');
});

test("catalogPicker keeps GlimStone's order and leaves out what nobody offers", () => {
  const catalog = { groups: ['a', 'b', 'c'], entries: [
    { id: 'z', name: 'Zed', group: 'a' }, { id: 'y', name: 'Yak', group: 'a', terms: ['ox'] },
    { id: 'w', name: 'Word', group: 'b', kind: 'thing', word: true }, { id: 'v', name: 'Vee', group: 'c' }
  ] };
  const { items, groups } = card.catalogPicker(catalog, {
    ids: ['z', 'y', 'w', 'nope'], titles: { a: 'A', b: { title: 'B', hint: 'why' }, c: 'C' }, lang: 'en',
    glyph: (e) => 'g:' + e.id, own: (e) => (e.word ? { label: 'Wort' } : undefined)
  });
  assert.deepEqual(groups, [{ id: 'a', title: 'A' }, { id: 'b', title: 'B', hint: 'why' }]);
  assert.deepEqual(items, [
    { id: 'y', label: 'Yak', group: 'a', terms: ['ox'], mark: 'g:y' },
    { id: 'z', label: 'Zed', group: 'a', mark: 'g:z' },
    { id: 'w', label: 'Wort', group: 'b', sub: 'thing', terms: ['Word'], mark: 'g:w' }
  ]);
});

test('the arrows move across the grid, and up from the first row is the search', () => {
  // Three tiles to a row, two rows, the last one short.
  const boxes = [0, 1, 2, 3, 4].map((i) => ({ left: (i % 3) * 100, top: Math.floor(i / 3) * 50, width: 90, height: 40 }));
  assert.equal(card.pickKey('ArrowRight', 0, boxes, false), 1);
  assert.equal(card.pickKey('ArrowRight', 4, boxes, false), 4);
  assert.equal(card.pickKey('ArrowLeft', 1, boxes, true), 2, 'right to left');
  assert.equal(card.pickKey('ArrowDown', 1, boxes, false), 4);
  assert.equal(card.pickKey('ArrowDown', 2, boxes, false), 4, 'the nearest one in the short row');
  assert.equal(card.pickKey('ArrowDown', 4, boxes, false), 4);
  assert.equal(card.pickKey('ArrowUp', 4, boxes, false), 1);
  assert.equal(card.pickKey('ArrowUp', 2, boxes, false), 'search');
  assert.equal(card.pickKey('Home', 3, boxes, false), 0);
  assert.equal(card.pickKey('End', 0, boxes, false), 4);
  assert.equal(card.pickKey('a', 0, boxes, false), null);
});

test('every kind the card offers has its mark, and the services their own colours', () => {
  for (const k of KINDS) assert.match(marks.marks[k], /^<svg [^>]*viewBox=/, k);
  for (const k of KINDS.filter((x) => x !== 'smtp' && x !== 'webhook')) assert.match(marks.tiles[k].tile, /^#[0-9a-f]{6}$/i, k);
  assert.deepEqual(marks.tiles.healthchecks, { tile: '#5cb85f', ink: '#161616' }, "Healthchecks.io lights in its mark's green");
});

test('a new channel is on, sends everything and takes an id no other channel has', () => {
  const c = card.newChannel('discord', [{ id: 'discord' }, { id: 'discord-2' }]);
  assert.deepEqual(c, { id: 'discord-3', kind: 'discord', enabled: true, on: 'always', discord: { url: '' } });
  assert.deepEqual(card.newChannel('unraid', []), { id: 'unraid', kind: 'unraid', enabled: true, on: 'always' });
  assert.equal(card.newChannel('webhook', []).webhook.method, 'POST');
});

test('a kept secret shows as an empty field, and an empty field keeps it', () => {
  const f = { type: 'secret' };
  assert.equal(card.shown(f, card.MASK), '');
  assert.equal(card.taken(f, '', card.MASK), card.MASK);
  assert.equal(card.taken(f, ' new ', card.MASK), 'new');
  assert.equal(card.taken(f, '', ''), '');
  assert.deepEqual(card.taken({ type: 'headers' }, 'Authorization: Bearer x\nbad line\n X-Y : z', {}), { Authorization: 'Bearer x', 'X-Y': 'z' });
  assert.deepEqual(card.taken({ type: 'words' }, ' a://1  b://2 '), ['a://1', 'b://2']);
});

test('Add opens a window with the search and the tiles, which never stand on the page', async () => {
  const p = mounted([]);
  await settle();
  assert.equal(p.el.byClass('glim-nc-tile').length, 0);
  assert.equal(p.win(), null);
  p.open();
  const win = p.win();
  assert.equal(win.attrs.role, 'dialog');
  assert.equal(win.attrs['aria-modal'], 'true');
  assert.equal(p.el.byClass('glim-nc-tile').length, 0, 'the window lives outside the card');
  assert.equal(win.byClass('glim-nc-tile').length, KINDS.length);
  assert.deepEqual(win.byClass('glim-nc-eyebrow').map((h) => h.text), [en.nc_services, en.nc_standards]);
  assert.equal(p.doc.activeElement, win.byClass('glim-nc-search')[0], 'the search has the focus');
  assert.deepEqual(win.byClass('glim-nc-win-btn').map((b) => b.textContent), [en.nc_close]);
  const discord = p.tile('discord').parent;
  assert.match(discord.attrs.style, /--nc-tile:#5865f2/);
  assert.equal(p.tile('webhook').parent.attrs.style, undefined, 'a glyph of nobody lights in the accent');
});

test("mail's tile carries SMTP as its second line and in its name", async () => {
  const p = mounted([]);
  await settle();
  p.open();
  const mail = p.tile('smtp');
  assert.deepEqual(mail.byClass('glim-nc-tile-sub').map((s) => s.text), ['SMTP']);
  assert.equal(mail.attrs['aria-label'], en.nc_smtp + ', SMTP');
  assert.equal(p.tile('discord').byClass('glim-nc-tile-sub').length, 0);
  assert.equal(p.tile('discord').attrs['aria-label'], undefined);
});

test('a picked tile shows its fields in the window, and Save adds the channel at once', async () => {
  const p = mounted([]);
  await settle();
  p.open();
  p.tile('discord').click();
  assert.equal(p.win().byClass('glim-nc-tile').length, 0);
  assert.deepEqual(p.win().byClass('glim-nc-win-btn').map((b) => b.textContent), [en.nc_cancel, en.nc_back, en.nc_test, en.nc_save]);
  assert.ok((' ' + p.win().className + ' ').includes(' glim-nc-win-form '), 'the form narrows the window');
  const url = p.win().byClass('glim-nc-in').find((i) => i.attrs.placeholder === 'https://discord.com/api/webhooks/...');
  url.value = 'https://discord.com/api/webhooks/1/x';
  url.fire('input');
  url.fire('change');
  await settle();
  assert.equal(p.saves.length, 0, 'nothing is saved before Save');
  p.foot(en.nc_save).click();
  await settle();
  assert.equal(p.saves.length, 1);
  assert.deepEqual(p.saves[0].map((c) => [c.kind, c.discord.url]), [['discord', 'https://discord.com/api/webhooks/1/x']]);
  assert.equal(p.win(), null, 'the window closes');
  assert.equal(p.el.byClass('glim-nc-conn').length, 1);
  assert.equal(p.doc.activeElement, p.el.byClass('glim-nc-add')[0], 'the focus is back on Add');
});

test('Cancel adds nothing, and Back keeps the search as it was', async () => {
  const p = mounted([]);
  await settle();
  p.open();
  const search = p.win().byClass('glim-nc-search')[0];
  search.value = 'web';
  search.fire('input');
  assert.deepEqual(p.win().byClass('glim-nc-tile').map((b) => b.attrs['data-kind']), ['webhook']);
  p.tile('webhook').click();
  p.foot(en.nc_back).click();
  assert.equal(p.win().byClass('glim-nc-search')[0].value, 'web');
  assert.equal(p.win().byClass('glim-nc-tile').length, 1);
  p.tile('webhook').click();
  p.foot(en.nc_cancel).click();
  await settle();
  assert.equal(p.win(), null);
  assert.equal(p.saves.length, 0);
});

test('Back marks the tile picked before and gives it the focus', async () => {
  const p = mounted([]);
  await settle();
  p.open();
  assert.equal(p.win().byClass('glim-nc-tile').filter((b) => b.getAttribute('aria-current') === 'true').length, 0);
  p.tile('ntfy').click();
  p.foot(en.nc_back).click();
  const marked = p.win().byClass('glim-nc-tile').filter((b) => b.getAttribute('aria-current') === 'true');
  assert.deepEqual(marked.map((b) => b.attrs['data-kind']), ['ntfy']);
  assert.equal(p.doc.activeElement, p.tile('ntfy'));
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /button\.glim-nc-tile\[aria-current="true"\] \{[^}]*outline: 2px solid var\(--nc-lit\)/);
});

test('in the search Enter picks the one tile left, Escape empties it first and then closes', async () => {
  const p = mounted([]);
  await settle();
  const listening = (p.doc.listeners.keydown || []).length;
  p.open();
  const search = p.win().byClass('glim-nc-search')[0];
  search.value = 'go';
  search.fire('input');
  assert.equal(p.doc.key('Enter', search).used, true);
  assert.ok(p.win().byClass('glim-nc-in').some((i) => i.attrs.placeholder === 'https://gotify.example.org'), 'Gotify picked');
  p.foot(en.nc_back).click();
  const again = p.win().byClass('glim-nc-search')[0];
  p.doc.key('Escape', again);
  assert.equal(again.value, '');
  assert.ok(p.win(), 'the first Escape only empties the search');
  p.doc.key('Escape', again);
  assert.equal(p.win(), null);
  assert.equal(p.doc.listeners.keydown.length, listening, 'the window stops listening once it is gone');
});

test('a choice in the window is a row of segments, and a method without a body hides the body', async () => {
  const p = mounted([]);
  await settle();
  p.open();
  p.tile('webhook').click();
  const body = () => p.win().byClass('glim-nc-lines').filter((i) => i.id.endsWith('webhook-body'));
  assert.equal(body().length, 1);
  const get = p.win().byClass('glim-nc-seg-opt').find((b) => b.text === 'GET');
  get.click();
  assert.equal(body().length, 0);
  assert.equal(p.win().byClass('glim-nc-seg-opt').find((b) => b.text === 'GET').attrs['aria-checked'], 'true');
  p.foot(en.nc_save).click();
  await settle();
  assert.equal(p.saves[0][0].webhook.method, 'GET');
});

test('a channel the service refuses keeps the window open and says why', async () => {
  const bad = (list) => Promise.reject({ code: 'invalid', problems: [{ index: list.length - 1, field: 'url', code: 'invalid' }] });
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }], { save: bad });
  await settle();
  p.open();
  assert.equal(p.tile('unraid').disabled, true, 'Unraid is there already');
  p.tile('slack').click();
  p.foot(en.nc_save).click();
  await settle();
  assert.ok(p.win());
  assert.equal(p.win().byClass('glim-nc-err')[0].textContent, en.nc_not_saved + ': ' + en.nc_fail_invalid);
  assert.deepEqual(p.view.list().map((c) => c.id), ['unraid'], 'the page keeps its list');
});

test("every channel is a row of GlimStone's connections: its lit tile, its name, what it sends, then Test, Edit and Remove", async () => {
  const p = mounted([
    { id: 'ntfy', kind: 'ntfy', name: 'Phone', enabled: true, on: 'failure', ntfy: { url: 'https://ntfy.sh/secret-topic', token: card.MASK } },
    { id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }
  ]);
  await settle();
  const rows = p.el.byClass('glim-nc-conn');
  assert.equal(rows.length, 2);
  assert.equal(rows[0].attrs.role, 'group');
  assert.equal(rows[0].attrs['aria-label'], 'Phone');
  assert.equal(rows[0].byClass('glim-nc-conn-name')[0].text, 'Phone');
  assert.equal(rows[0].byClass('glim-nc-conn-status')[0].text, 'ntfy.sh · ' + en.nc_on_failure, 'the host alone, never the path that may hold a secret');
  assert.equal(rows[1].byClass('glim-nc-conn-status')[0].text, en.nc_on_always);
  assert.deepEqual(rows[0].byClass('glim-nc-btn').map((b) => b.textContent), [en.nc_test, en.nc_edit, en.nc_remove]);
  assert.deepEqual(rows[0].byClass('glim-nc-conn-actions')[0].children.map((b) => b.attrs['data-connection']), ['switch', 'test', 'edit', 'remove'], 'the switch stands first among the actions');
  for (const b of p.el.byClass('glim-nc-btn')) assert.match(b.children[0].html, /^<svg viewBox=/, b.textContent);
  assert.equal(p.el.byClass('glim-nc-in').length, 0, 'the fields wait in the window');
  const tile = rows[0].byClass('glim-nc-conn-tile')[0];
  assert.match(tile.attrs.style || '', /--nc-tile:/);
  assert.equal(tile.attrs['aria-hidden'], 'true');
  const sw = rows[0].byClass('glim-nc-sw')[0];
  assert.equal(sw.tagName, 'INPUT');
  assert.equal(sw.attrs.role, 'switch');
  assert.equal(sw.attrs['aria-label'], 'Phone');
  assert.equal(sw.checked, true);
});

test('a channel switched off says so on its row and can still be tested', async () => {
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: false, on: 'always' }]);
  await settle();
  const row = p.el.byClass('glim-nc-conn')[0];
  assert.equal(row.byClass('glim-nc-sw')[0].checked, false);
  assert.equal(row.byClass('glim-nc-conn-status')[0].text, en.nc_off);
  const test = row.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_test);
  assert.equal(test.disabled, false);
  test.click();
  await settle();
  assert.equal(p.tests.length, 1);
});

test("a row's switch saves the list at once and keeps the channel's settings and its secret", async () => {
  const p = mounted([
    { id: 'unraid', kind: 'unraid', enabled: true, on: 'always' },
    { id: 'matrix', kind: 'matrix', enabled: true, on: 'failure', matrix: { homeserver: 'https://m.example', room: '!r:m', token: card.MASK } }
  ]);
  await settle();
  const sw = p.el.byClass('glim-nc-conn')[1].byClass('glim-nc-sw')[0];
  sw.checked = false;
  sw.fire('change');
  const now = p.el.byClass('glim-nc-conn')[1];
  assert.equal(now.byClass('glim-nc-conn-status')[0].text, 'm.example · ' + en.nc_off, 'the row turns at the press');
  assert.equal(p.doc.activeElement, now.byClass('glim-nc-sw')[0], 'the switch keeps the focus');
  await settle();
  assert.equal(p.saves.length, 1);
  assert.deepEqual(p.saves[0].map((c) => [c.id, c.enabled]), [['unraid', true], ['matrix', false]]);
  assert.deepEqual(p.saves[0][1].matrix, { homeserver: 'https://m.example', room: '!r:m', token: card.MASK });
  assert.deepEqual(p.saved, [[true, en.nc_saved_line]]);
});

test("a switch the service refuses for the channel itself opens its form, switched on, with the reason", async () => {
  let refuse = true;
  const save = (list) => refuse
    ? Promise.reject({ code: 'invalid', problems: [{ index: 0, field: 'slack.url', code: 'invalid' }] })
    : Promise.resolve(JSON.parse(JSON.stringify(list)));
  const p = mounted([{ id: 'slack', kind: 'slack', enabled: false, on: 'always', slack: { url: '' } }], { save });
  await settle();
  const sw = p.el.byClass('glim-nc-sw')[0];
  sw.checked = true;
  sw.fire('change');
  await settle();
  assert.equal(p.el.byClass('glim-nc-sw')[0].checked, false, 'the row turns back');
  assert.equal(p.view.list()[0].enabled, false);
  assert.ok(p.win(), 'the form opens');
  assert.equal(p.win().byClass('glim-nc-err')[0].textContent, en.nc_not_saved + ': ' + en.nc_fail_invalid);
  assert.equal(p.el.byClass('glim-nc-err').length, 0, 'the reason stands in the form, not under the row');
  assert.deepEqual(p.saved, [], 'the form says it, so the page line does not');
  refuse = false;
  p.win().querySelector('input').value = 'https://hooks.slack.com/services/a';
  p.win().querySelector('input').fire('input');
  p.foot(en.nc_save).click();
  await settle();
  assert.equal(p.saves[1][0].enabled, true, 'the form saves the channel switched as asked');
  assert.equal(p.view.list()[0].enabled, true);
});

test('a switch the service refuses for another reason turns back, shakes and says why', async () => {
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }], { save: () => Promise.reject({ code: 'engine' }) });
  await settle();
  const sw = p.el.byClass('glim-nc-sw')[0];
  sw.checked = false;
  sw.fire('change');
  await settle();
  const back = p.el.byClass('glim-nc-sw')[0];
  assert.equal(back.checked, true);
  assert.equal(p.view.list()[0].enabled, true);
  assert.ok(back.className.includes('glim-nc-shake'));
  assert.equal(p.doc.activeElement, back);
  assert.equal(p.win(), null);
  assert.deepEqual(p.saved, [[false, en.nc_not_saved]]);
});

test('a second switch waits while the first one saves', async () => {
  let release;
  const slow = (list) => new Promise((r) => { release = () => r(JSON.parse(JSON.stringify(list))); });
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }, { id: 'ntfy', kind: 'ntfy', enabled: true, on: 'always', ntfy: { url: 'https://ntfy.sh/x' } }], { save: slow });
  await settle();
  const first = p.el.byClass('glim-nc-sw')[0];
  first.checked = false;
  first.fire('change');
  const second = p.el.byClass('glim-nc-sw')[1];
  second.checked = false;
  second.fire('change');
  assert.equal(second.checked, true, 'the second switch stays where it was');
  assert.equal(p.saves.length, 1);
  release();
  await settle();
  assert.deepEqual(p.view.list().map((c) => c.enabled), [false, true]);
});

test("the form has no switch of its own, since the row's switch says whether a channel sends", async () => {
  const p = mounted([{ id: 'slack', kind: 'slack', enabled: false, on: 'always', slack: { url: 'https://hooks.slack.com/services/a' } }]);
  await settle();
  p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_edit).click();
  assert.deepEqual(p.win().byClass('glim-nc-name').map((n) => n.text), [en.nc_name, en.nc_on, en.nc_address]);
  assert.equal(p.win().byClass('glim-nc-seg').length, 1, 'only what it sends is a choice');
  p.foot(en.nc_save).click();
  await settle();
  assert.equal(p.saves[0][0].enabled, false, 'an edit keeps the switch where it was');
  assert.equal(en.nc_use, undefined);
  assert.equal(en.nc_on_state, undefined);
});

test('Edit opens the channel in the window, and Save puts it back in its place', async () => {
  const p = mounted([
    { id: 'unraid', kind: 'unraid', enabled: true, on: 'always' },
    { id: 'slack', kind: 'slack', enabled: true, on: 'always', slack: { url: 'https://hooks.slack.com/services/a' } }
  ]);
  await settle();
  p.el.byClass('glim-nc-conn')[1].byClass('glim-nc-btn').find((b) => b.textContent === en.nc_edit).click();
  assert.ok(p.win());
  assert.equal(p.win().byClass('glim-nc-win-title')[0].children[0].text, en.nc_edit_title);
  assert.deepEqual(p.win().byClass('glim-nc-win-btn').map((b) => b.textContent), [en.nc_cancel, en.nc_test, en.nc_save]);
  const name = p.win().byClass('glim-nc-in').find((i) => i.id.endsWith('-name'));
  name.value = 'Team';
  name.fire('input');
  assert.equal(p.view.list()[1].name, undefined, 'the page keeps its channel until Save');
  p.foot(en.nc_save).click();
  await settle();
  assert.deepEqual(p.saves[0].map((c) => [c.id, c.name]), [['unraid', undefined], ['slack', 'Team']]);
  assert.equal(p.win(), null);
  assert.equal(p.el.byClass('glim-nc-conn-name')[1].text, 'Team');
  const edit = p.el.byClass('glim-nc-conn')[1].byClass('glim-nc-btn').find((b) => b.textContent === en.nc_edit);
  assert.equal(p.doc.activeElement, edit, 'the focus goes back to Edit');
});

test('Cancel in the edit window leaves the channel as it was', async () => {
  const p = mounted([{ id: 'slack', kind: 'slack', enabled: true, on: 'always', slack: { url: 'https://hooks.slack.com/services/a' } }]);
  await settle();
  p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_edit).click();
  const url = p.win().byClass('glim-nc-in').find((i) => i.attrs.placeholder === 'https://hooks.slack.com/services/...');
  url.value = 'https://elsewhere';
  url.fire('input');
  p.foot(en.nc_cancel).click();
  await settle();
  assert.equal(p.saves.length, 0);
  assert.equal(p.view.list()[0].slack.url, 'https://hooks.slack.com/services/a');
});

test('a kept secret waits in the edit window as an empty field and goes back as it was', async () => {
  const p = mounted([{ id: 'matrix', kind: 'matrix', enabled: true, on: 'failure', matrix: { homeserver: 'https://m.example', room: '!r:m', token: card.MASK } }]);
  await settle();
  p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_edit).click();
  const token = p.win().byClass('glim-nc-in').find((i) => i.attrs.type === 'password');
  assert.equal(token.value, '');
  assert.equal(token.attrs.placeholder, en.nc_saved);
  p.foot(en.nc_save).click();
  await settle();
  assert.deepEqual(p.saves[0][0].matrix, { homeserver: 'https://m.example', room: '!r:m', token: card.MASK });
});

test('a field the service refuses keeps the edit window open and says why', async () => {
  const bad = () => Promise.reject({ code: 'invalid', problems: [{ index: 0, field: 'url', code: 'invalid' }] });
  const p = mounted([{ id: 'slack', kind: 'slack', enabled: true, on: 'always', slack: { url: '' } }], { save: bad });
  await settle();
  p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_edit).click();
  p.foot(en.nc_save).click();
  await settle();
  assert.ok(p.win());
  assert.equal(p.win().byClass('glim-nc-err')[0].textContent, en.nc_not_saved + ': ' + en.nc_fail_invalid);
});

test("a row's test wears its outcome on its button and in the row's line, and keeps it through a redraw", async () => {
  const p = mounted([{ id: 'slack', kind: 'slack', enabled: true, on: 'always', slack: { url: 'https://hooks.slack.com/services/a' } }], { test: { ok: false, code: 'not_found' } });
  await settle();
  const verdict = en.nc_test_failed + ': ' + en.nc_fail_not_found;
  const test = p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_test);
  const rest = test.children[0].html;
  test.click();
  assert.equal(test.attrs['aria-busy'], 'true');
  await settle();
  assert.equal(p.tests[0].slack.url, 'https://hooks.slack.com/services/a');
  assert.equal(p.el.byClass('glim-nc-conn-status')[0].text, 'hooks.slack.com · ' + verdict);
  assert.equal(p.el.byClass('glim-nc-res').length, 0, 'no line of its own under the row');
  assert.ok(test.className.includes('glim-nc-tone-bad'));
  assert.ok(test.className.includes('glim-nc-shake'));
  assert.equal(test.attrs.title, verdict);
  assert.notEqual(test.children[0].html, rest, 'a cross in place of the check');
  assert.equal(test.textContent, en.nc_test, 'its words stay');
  p.view.draw();
  const again = p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_test);
  assert.ok(again.className.includes('glim-nc-tone-bad'));
  assert.equal(p.el.byClass('glim-nc-conn-status')[0].text, 'hooks.slack.com · ' + verdict);
});

test("a row's line says where the channel sends, then its test or what it sends", () => {
  const c = { id: 'ntfy', kind: 'ntfy', enabled: true, on: 'always', ntfy: { url: 'https://ntfy.sh/topic' } };
  assert.equal(card.rowStatus(c, null, en), 'ntfy.sh · ' + en.nc_on_always);
  assert.equal(card.rowStatus(Object.assign({}, c, { enabled: false }), null, en), 'ntfy.sh · ' + en.nc_off);
  assert.equal(card.rowStatus(Object.assign({}, c, { enabled: false }), { ok: true, text: en.nc_test_ok }, en), 'ntfy.sh · ' + en.nc_test_ok);
  assert.equal(card.rowStatus({ id: 'unraid', kind: 'unraid', enabled: true, on: 'failure' }, null, en), en.nc_on_failure);
});

test("a row without an address says so, as GlimStone's does, and Unraid needs none", () => {
  assert.equal(en.nc_no_address, 'No address yet');
  assert.equal(card.rowStatus({ id: 'slack', kind: 'slack', enabled: true, on: 'always', slack: { url: '' } }, null, en), en.nc_no_address + ' · ' + en.nc_on_always);
  assert.equal(card.rowStatus({ id: 'smtp', kind: 'smtp', enabled: false, on: 'always', smtp: {} }, null, en), en.nc_no_address + ' · ' + en.nc_off);
  assert.equal(card.rowStatus({ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }, null, en), en.nc_on_always);
});

test('a channel is removed with its button', async () => {
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }, { id: 'ntfy', kind: 'ntfy', enabled: true, on: 'always', ntfy: { url: 'https://ntfy.sh/x' } }]);
  await settle();
  p.el.byClass('glim-nc-conn')[0].byClass('glim-nc-btn').find((b) => b.textContent === en.nc_remove).click();
  await settle();
  assert.deepEqual(p.saves[0].map((c) => c.id), ['ntfy']);
});

test('a removal the service refuses brings the channel back', async () => {
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }], { save: () => Promise.reject({ code: 'engine' }) });
  await settle();
  p.el.byClass('glim-nc-btn').find((b) => b.textContent === en.nc_remove).click();
  assert.equal(p.el.byClass('glim-nc-conn').length, 0);
  await settle();
  assert.equal(p.el.byClass('glim-nc-conn').length, 1);
  assert.deepEqual(p.saved, [[false, en.nc_not_saved]]);
});

test("an empty list reads as GlimStone's empty state over the foot with Add", async () => {
  const p = mounted([]);
  await settle();
  const root = p.el.children[0];
  assert.deepEqual(root.children.map((n) => n.className), ['glim-nc-empty', 'glim-nc-foot']);
  assert.equal(root.byClass('glim-nc-empty-title')[0].text, en.nc_empty);
  assert.equal(root.byClass('glim-nc-foot')[0].children[0], p.el.byClass('glim-nc-add')[0]);
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /\.glim-nc \.glim-nc-empty-title \{[^}]*text-align: center;/, 'the line centres itself, since Unraid justifies every paragraph');
  assert.match(css, /\.glim-nc-foot \{ display: flex; flex-wrap: wrap; justify-content: flex-end;/);
});

test('the window buttons wear their glyphs before their words', async () => {
  const p = mounted([]);
  await settle();
  p.open();
  p.tile('slack').click();
  for (const b of p.win().byClass('glim-nc-win-btn')) assert.match(b.children[0].html, /^<svg viewBox=/, b.textContent);
});

test("the (i) is GlimStone's stroked glyph with its words for the one bubble", async () => {
  const p = mounted([{ id: 'unraid', kind: 'unraid', enabled: true, on: 'always' }]);
  await settle();
  const i = p.el.byClass('glim-nc-info')[0];
  assert.equal(i.attrs['data-tip'], en.nc_unraid_hint);
  assert.equal(i.attrs['aria-label'], en.nc_unraid_hint);
  assert.equal(i.attrs.tabindex, '0');
  assert.match(i.html, /<circle cx="8" cy="8" r="7" stroke="currentColor"/);
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.doesNotMatch(css, /glim-nc-info[^{]*::after/, 'no bubble of its own beside the shared one');
});

test("under the theme plugin a quiet button hovers to its accent-hued ground, and a field's focus stays grey", () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
  const tt = /html\.tt-popups-on \.glim-nc-wins \{([^}]*)\}/.exec(css)[1];
  assert.match(tt, /--nc-w-hover: var\(--tt-hover-dark, #383838\)/);
  assert.match(tt, /--nc-w-raised-hover: #383838/);
  assert.match(css, /button\.glim-nc-win-btn:is\(:hover, :focus-visible\):not\(:disabled\) \{ background: var\(--nc-w-hover\); \}/);
  assert.match(css, /button\.glim-nc-seg-opt:hover \{ background: var\(--nc-w-hover\); \}/);
  assert.match(css, /input\.glim-nc-search:focus \{ background: var\(--nc-w-raised-hover\); \}/);
  assert.doesNotMatch(css, /var\(--nc-w-head\) 22%/, 'no tint mixed into the button');
});

test('a lit tile takes its brand colour before the accent, and nothing draws a line', () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /--nc-lit: var\(--nc-tile, var\(--nc-w-head\)\)/);
  const layer = css.slice(css.indexOf('@layer glim-notify-card'));
  assert.match(layer, /button\.glim-nc-tile \{\s*background-color: var\(--nc-lit\) !important/);
  for (const file of ['notify-card.css']) {
    const sheet = fs.readFileSync(path.join(PLUGIN, 'styles', file), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
    assert.doesNotMatch(sheet, /(^|[;{\s])border(-(top|bottom|left|right|block|inline)(-(start|end))?)?:\s*\d[^;]*solid/m, file);
  }
});

test('a tile name keeps its descenders inside the clip that ends a long one', () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /\.glim-nc-tile-name, \.glim-nc-tile-sub \{[^}]*overflow: hidden;[^}]*padding-block: \.15em; margin-block: -\.15em;/);
});

test("an open select of the page keeps TowerTorch's own plain rows", () => {
  // TowerTorch 1.5.2 lists the options as GlimStone's rows itself, and an
  // important rule in the card's layer would outrank its tint and its check.
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8').replace(/\/\*[\s\S]*?\*\//g, '');
  assert.doesNotMatch(css, /\.tt-tsel-(opt|panel)/);
});

test('a mark at rest keeps its colours, Matrix aside, and a field keeps its control at its end', () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.doesNotMatch(css, /\.glim-nc-wins button\.glim-nc-tile \{\s*--mark-ink/);
  assert.match(css, /\.glim-nc-wins button\.glim-nc-tile\[data-kind="matrix"\] \{ --mark-ink: var\(--nc-w-ink\); \}/);
  assert.match(css, /\.glim-nc-ctl \{ display: flex; justify-content: flex-end;/);
});

test("a connection row follows GlimStone 4.8.1: the tile the row's height, buttons a surface off the well that a test's outcome colours over", () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /\.glim-nc-conn \{[^}]*display: flex;[^}]*overflow: hidden;[^}]*background: var\(--nc-sub\)/);
  assert.match(css, /\.glim-nc-conn-tile \{[^}]*width: 5\.5rem;[^}]*background-color: var\(--nc-tile, var\(--nc-accent\)\)/);
  assert.match(css, /html\.tt-popups-on \.glim-nc \.glim-nc-conn-actions \.glim-nc-btn:not\(:hover, :focus-visible, \.glim-nc-tone-ok, \.glim-nc-tone-bad\) \{ background: var\(--nc-btn\) !important; \}/);
  assert.match(css, /\.glim-nc-conn-foot \{[^}]*flex-wrap: wrap;[^}]*justify-content: flex-end/);
});

test('a row follows GlimStone 4.9.2: the switch keeps its distance from the buttons, and a hovered button fills with the accent', () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  const layer = css.slice(css.indexOf('@layer glim-notify-card'));
  assert.match(layer, /\.glim-nc-conn-actions > input\.glim-nc-sw:not\(:last-child\) \{ margin-inline-end: 12px !important; \}/);
  assert.match(layer, /\.glim-nc-conn-actions button\.glim-nc-btn:not\(:disabled, \.glim-nc-tone-ok, \.glim-nc-tone-bad\):is\(:hover, :focus-visible\) \{\s*background: var\(--nc-accent\) !important; color: var\(--nc-accent-ink\) !important;/);
  assert.doesNotMatch(css, /glim-nc-badge/, 'the switch speaks for itself');
});

test('every string the card asks for is in every language', () => {
  const src = fs.readFileSync(path.join(PLUGIN, 'scripts', 'notify-card.js'), 'utf8');
  const keys = [...new Set(src.match(/nc_[a-z_]+/g))].filter((k) => k !== 'nc_fail_' && k !== 'nc_');
  for (const l of fs.readdirSync(path.join(PLUGIN, 'lang'))) {
    const tx = JSON.parse(fs.readFileSync(path.join(PLUGIN, 'lang', l), 'utf8'));
    for (const k of keys) assert.ok(tx[k], l + ' ' + k);
    assert.match(tx.nc_none, /\{query\}/, l);
  }
});

test("the window's test shows its outcome on its button and at the foot's start, and a press beside a form keeps it", async () => {
  const p = mounted([], { test: { ok: false, code: 'timeout' } });
  await settle();
  p.open();
  p.tile('ntfy').click();
  const test = p.foot(en.nc_test);
  test.click();
  assert.equal(test.attrs['aria-busy'], 'true');
  await settle();
  assert.equal(test.attrs['aria-busy'], undefined);
  assert.ok(test.className.includes('glim-nc-tone-bad'));
  assert.equal(test.attrs.title, en.nc_test_failed + ': ' + en.nc_fail_timeout);
  assert.equal(test.textContent, en.nc_test, 'its words stay');
  assert.match(test.children[0].html, /d="M11\.957 13\.716/, 'a cross for its glyph');
  const start = p.win().byClass('glim-nc-win-start')[0];
  assert.equal(start.textContent, en.nc_test_failed + ': ' + en.nc_fail_timeout);
  p.doc.body.byClass('glim-nc-scrim')[0].click();
  assert.ok(p.win(), 'the form stays open');
});

test("the window's buttons wrap as one group, and the start line takes a line of its own before one of them drops", async () => {
  const p = mounted([]);
  await settle();
  p.open();
  p.tile('ntfy').click();
  const act = p.win().byClass('glim-nc-win-act')[0];
  const [start, group] = act.children;
  assert.ok(start.className.includes('glim-nc-win-start'));
  assert.ok(group.className.includes('glim-nc-win-actions'));
  assert.deepEqual(group.children.map((b) => b.textContent), [en.nc_cancel, en.nc_back, en.nc_test, en.nc_save]);
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /\.glim-nc-win-actions \{ display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: inherit; \}/);
  assert.match(css, /\.glim-nc-win-act \.glim-nc-win-start \{ flex: 1 1 auto; overflow-wrap: anywhere;/);
  assert.doesNotMatch(css, /glim-nc-win-start \{[^}]*min-width/);
});

test("a button's words keep the tails of Arabic inside their clip", () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /\.glim-nc-l \{[^}]*overflow: hidden;[^}]*padding-block: \.15em; margin-block: -\.15em;/);
});

test('a mark that reaches its box keeps its soft edge on a tile', () => {
  const css = fs.readFileSync(path.join(PLUGIN, 'styles', 'notify-card.css'), 'utf8');
  assert.match(css, /\.glim-nc-wins \.glim-nc-tile-mark svg \{ overflow: visible; \}/);
});

test('the search clears with its own button', async () => {
  const p = mounted([]);
  await settle();
  p.open();
  const search = p.win().byClass('glim-nc-search')[0];
  const clear = p.win().byClass('glim-nc-search-clear')[0];
  assert.equal(clear.hidden, true);
  search.value = 'slack';
  search.fire('input');
  assert.equal(clear.hidden, false);
  assert.equal(p.win().byClass('glim-nc-tile').length, 1);
  clear.click();
  assert.equal(search.value, '');
  assert.equal(p.win().byClass('glim-nc-tile').length, KINDS.length);
  assert.equal(p.doc.activeElement, search);
});
