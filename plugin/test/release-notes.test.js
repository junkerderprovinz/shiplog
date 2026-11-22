'use strict';
const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('node:path');

const PLUGIN = path.join(__dirname, '..', 'src', 'shiplog', 'usr', 'local', 'emhttp', 'plugins', 'shiplog');
const notes = require(path.join(PLUGIN, 'scripts', 'release-notes.js'));

// Elements and text nodes, enough to read back what the notes were built from.
function doc() {
  const d = {
    createTextNode: (text) => ({ nodeType: 3, text, get textContent() { return this.text; } }),
    createElement: (tag) => ({
      tagName: tag.toUpperCase(), className: '', attrs: {}, kids: [], ownerDocument: d,
      setAttribute(k, v) { this.attrs[k] = String(v); },
      appendChild(c) { this.kids.push(c); return c; },
      set textContent(v) { this.kids = [d.createTextNode(v)]; },
      get textContent() { return this.kids.map((k) => k.textContent).join(''); },
    }),
  };
  return d;
}
const all = (n) => (n.kids || []).flatMap((k) => [k, ...all(k)]);
function draw(md) {
  const into = doc().createElement('div');
  notes.render(md, into);
  return into;
}

test('the notes of a release read as on its release page', () => {
  const box = draw('Short summary of the release.\n\n## ✨ Added\n- **Version card.** With **bold**,\n  `code` and *care*.\n- Second point\n\n1. one\n2. two');
  assert.deepEqual(box.kids.map((k) => k.tagName), ['P', 'DIV', 'UL', 'OL']);
  const h = box.kids[1];
  assert.equal(h.className, 'sl-notes-h');
  assert.deepEqual([h.attrs.role, h.attrs['aria-level'], h.textContent], ['heading', '3', '✨ Added']);
  const first = box.kids[2].kids[0];
  assert.equal(first.textContent, 'Version card. With bold, code and care.', 'a wrapped line joins its point');
  assert.deepEqual(all(first).filter((n) => n.tagName).map((n) => n.tagName), ['STRONG', 'STRONG', 'CODE', 'EM']);
  assert.equal(box.kids[3].kids.length, 2);
});

test('markup in the notes stays text', () => {
  const box = draw('<script>alert(1)</script>\n\n```\n<b>raw</b>\n```\n\n<img src=x onerror=alert(2)>');
  assert.ok(!all(box).some((n) => /^(SCRIPT|B|IMG)$/.test(n.tagName || '')));
  assert.equal(box.kids[0].textContent, '<script>alert(1)</script>');
  assert.equal(box.kids[1].tagName, 'PRE');
  assert.equal(box.kids[1].textContent, '<b>raw</b>');
});

test('only a link to http or https becomes an anchor, and it opens a new tab', () => {
  const box = draw('[page](https://example.org) and [trap](javascript:alert) and [file](file:///etc/passwd)');
  const links = all(box).filter((n) => n.tagName === 'A');
  assert.equal(links.length, 1);
  assert.deepEqual([links[0].href, links[0].target, links[0].rel], ['https://example.org', '_blank', 'noopener noreferrer']);
  assert.equal(box.textContent, 'page and trap and file');
});

test('the drawing never writes HTML', () => {
  const src = require('node:fs').readFileSync(path.join(PLUGIN, 'scripts', 'release-notes.js'), 'utf8');
  assert.doesNotMatch(src, /innerHTML|insertAdjacentHTML|outerHTML|document\.write/);
});
