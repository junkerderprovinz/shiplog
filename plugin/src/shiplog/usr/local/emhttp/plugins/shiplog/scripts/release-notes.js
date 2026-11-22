/* Release notes in GitHub's Markdown, drawn into an element of the page as
 * FireSquire and TowerTorch draw theirs: headings, paragraphs, lists, code,
 * bold, italics and links. Everything is built from elements and text nodes,
 * so nothing in the notes is ever read as HTML, and a link becomes an anchor
 * only when it leads to http or https. */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.ShipLogNotes = factory();
})(this, function () {
  'use strict';

  function el(doc, tag, cls, text) {
    var n = doc.createElement(tag);
    if (cls) n.className = cls;
    if (text != null) n.textContent = text;
    return n;
  }

  function inline(doc, text, into) {
    var re = /\*\*(.+?)\*\*|\x60([^\x60]+)\x60|\[([^\]]+)\]\(([^)\s]+)\)|\*(\S(?:.*?\S)?)\*/g, last = 0, m;
    while ((m = re.exec(text))) {
      if (m.index > last) into.appendChild(doc.createTextNode(text.slice(last, m.index)));
      if (m[1] != null) inline(doc, m[1], into.appendChild(el(doc, 'strong')));
      else if (m[2] != null) into.appendChild(el(doc, 'code', null, m[2]));
      else if (m[3] != null) {
        if (/^https?:\/\//i.test(m[4])) {
          var a = el(doc, 'a', null, m[3]);
          a.href = m[4];
          a.target = '_blank';
          a.rel = 'noopener noreferrer';
          into.appendChild(a);
        } else into.appendChild(doc.createTextNode(m[3]));
      } else inline(doc, m[5], into.appendChild(el(doc, 'em')));
      last = re.lastIndex;
    }
    if (last < text.length) into.appendChild(doc.createTextNode(text.slice(last)));
  }

  function blocks(md) {
    var out = [], cur = null, code = null;
    String(md || '').replace(/\r\n?/g, '\n').split('\n').forEach(function (ln) {
      if (code) {
        if (/^\s*\x60{3}/.test(ln)) code = null;
        else code.text += (code.text ? '\n' : '') + ln;
        return;
      }
      var m;
      if (/^\s*\x60{3}/.test(ln)) { code = { type: 'pre', text: '' }; out.push(code); cur = null; }
      else if (!ln.trim() || /^\s*([-*_])(\s*\1){2,}\s*$/.test(ln)) cur = null;
      else if ((m = /^(#{1,6})\s+(.*?)\s*#*\s*$/.exec(ln))) { out.push({ type: 'h', text: m[2] }); cur = null; }
      else if ((m = /^\s*([-*+]|\d+[.)])\s+(.*)$/.exec(ln))) {
        var kind = /\d/.test(m[1]) ? 'ol' : 'ul';
        if (!cur || cur.type !== kind) { cur = { type: kind, items: [] }; out.push(cur); }
        cur.items.push(m[2]);
      } else if (cur && (cur.type === 'ul' || cur.type === 'ol')) cur.items[cur.items.length - 1] += ' ' + ln.trim();
      else if (cur && cur.type === 'p') cur.text += ' ' + ln.trim().replace(/^>\s?/, '');
      else { cur = { type: 'p', text: ln.trim().replace(/^>\s?/, '') }; out.push(cur); }
    });
    return out;
  }

  // A heading stays a div with the heading role, so Unraid's and TowerTorch's
  // sheets for h1 to h6 leave it alone.
  function render(md, into) {
    var doc = into.ownerDocument;
    blocks(md).forEach(function (b) {
      if (b.type === 'pre') { into.appendChild(el(doc, 'pre')).appendChild(el(doc, 'code', null, b.text)); return; }
      if (b.type === 'h') {
        var h = el(doc, 'div', 'sl-notes-h');
        h.setAttribute('role', 'heading');
        h.setAttribute('aria-level', '3');
        inline(doc, b.text, into.appendChild(h));
        return;
      }
      if (b.type === 'p') { inline(doc, b.text, into.appendChild(el(doc, 'p'))); return; }
      var list = into.appendChild(el(doc, b.type));
      b.items.forEach(function (x) { inline(doc, x, list.appendChild(el(doc, 'li'))); });
    });
  }

  return { render: render };
});
