/* The notification channels of a plugin's settings page, for a page without
 * a framework. Every channel is a sub-card of its own with its switch, what it
 * sends, its few fields and a test. A channel is added in a picker window, as
 * GlimStone's PickerWindow draws one: the services as tiles with their marks
 * under a search, and once one is picked its fields in the same window.
 * The channels keep to the kit's notify format, so the same file serves every plugin
 * that keeps its channels that way.
 *
 * mount(el, adapter, opts) draws the card into el. The adapter loads the
 * channels, saves the whole list (it answers with the list as saved, secrets
 * masked) and tests one channel. opts carries the strings (t, the nc_ keys),
 * the language, the kinds the plugin offers, the marks (notify-marks.js), the
 * host's classes for fields, buttons and switches, and saved(ok, text) for
 * the page's saved line. Every change on the page is saved at once, text a
 * moment after the last key, and a new channel when its window is saved. A
 * secret the service keeps is never shown: its field stays empty and says
 * so, and an empty field keeps it. */
(function (root, factory) {
  if (typeof module === 'object' && module.exports) module.exports = factory();
  else root.GlimNotifyCard = factory();
})(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  var MASK = '********';

  // GlimStone's IconInfo, the glyph every (i) of the house wears.
  var INFO = "<svg class=\"gl\" viewBox=\"0 0 14 14\" aria-hidden=\"true\" focusable=\"false\"><path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M7 14c3.866 0 7 -3.134 7 -7 0 -3.86599 -3.134 -7 -7 -7 -3.86599 0 -7 3.13401 -7 7 0 3.866 3.13401 7 7 7ZM5.5 9.375c-0.34518 0 -0.625 0.27982 -0.625 0.625 0 0.3452 0.27982 0.625 0.625 0.625h3c0.34518 0 0.625 -0.2798 0.625 -0.625 0 -0.34518 -0.27982 -0.625 -0.625 -0.625h-0.875V6.5c0 -0.34518 -0.27982 -0.625 -0.625 -0.625H6c-0.34518 0 -0.625 0.27982 -0.625 0.625s0.27982 0.625 0.625 0.625h0.375v2.25H5.5ZM8 4c0 0.55228 -0.44772 1 -1 1s-1 -0.44772 -1 -1 0.44772 -1 1 -1 1 0.44772 1 1Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path></svg>";

  // The services' own names, which no language translates. Mail and
  // webhooks are nobody's and go by the page's words.
  var BRANDS = { unraid: 'Unraid', discord: 'Discord', slack: 'Slack', ntfy: 'ntfy', gotify: 'Gotify', matrix: 'Matrix', apprise: 'Apprise', healthchecks: 'Healthchecks.io' };
  var WORDS = { smtp: 'nc_smtp', webhook: 'nc_webhook' };
  var HINTS = {
    unraid: 'nc_unraid_hint', discord: 'nc_discord_hint', slack: 'nc_slack_hint', ntfy: 'nc_ntfy_hint', gotify: 'nc_gotify_hint',
    matrix: 'nc_matrix_hint', apprise: 'nc_apprise_hint', smtp: 'nc_smtp_hint', webhook: 'nc_webhook_hint', healthchecks: 'nc_hc_hint'
  };
  // The open standards stand after the services, in a section of their own.
  var STANDARDS = ['smtp', 'webhook'];
  // A kind a page can have only once, since a second would say the same.
  var SINGLE = ['unraid'];

  var WEBHOOK_BODY = '{"subject":"%%title%%","body":"%%body%%","ok":%%ok%%}';

  /* What each kind asks for. path is where the value sits in the channel,
   * type how it is edited: text, secret (kept by the service, never shown),
   * number, choice, words (a list on one line), lines and headers. */
  var FIELDS = {
    unraid: [],
    discord: [{ path: 'discord.url', label: 'nc_address', ph: 'https://discord.com/api/webhooks/...' }],
    slack: [{ path: 'slack.url', label: 'nc_address', ph: 'https://hooks.slack.com/services/...' }],
    ntfy: [
      { path: 'ntfy.url', label: 'nc_address', ph: 'https://ntfy.sh/your-topic' },
      { path: 'ntfy.token', label: 'nc_token', hint: 'nc_token_hint', type: 'secret' }
    ],
    gotify: [
      { path: 'gotify.url', label: 'nc_address', ph: 'https://gotify.example.org' },
      { path: 'gotify.token', label: 'nc_token', hint: 'nc_token_hint', type: 'secret' },
      { path: 'gotify.priorityFailure', label: 'nc_prio_fail', type: 'number', min: 0, max: 10, dflt: 8 },
      { path: 'gotify.prioritySuccess', label: 'nc_prio_ok', type: 'number', min: 0, max: 10, dflt: 5 }
    ],
    matrix: [
      { path: 'matrix.homeserver', label: 'nc_matrix_home', ph: 'https://matrix.example.org' },
      { path: 'matrix.room', label: 'nc_matrix_room', ph: '!abc:example.org' },
      { path: 'matrix.token', label: 'nc_matrix_token', type: 'secret' }
    ],
    apprise: [
      { path: 'apprise.url', label: 'nc_address', ph: 'http://apprise:8000/notify/apps' },
      { path: 'apprise.urls', label: 'nc_apprise_urls', hint: 'nc_apprise_urls_hint', type: 'words', ph: 'tgram://bottoken/chatid' },
      { path: 'apprise.tags', label: 'nc_apprise_tags', hint: 'nc_apprise_tags_hint' }
    ],
    smtp: [
      { path: 'smtp.host', label: 'nc_smtp_host', ph: 'mail.example.org' },
      { path: 'smtp.tls', label: 'nc_smtp_tls', type: 'choice', options: [['starttls', 'STARTTLS'], ['tls', 'TLS'], ['none', 'nc_tls_none']], dflt: 'starttls' },
      { path: 'smtp.port', label: 'nc_smtp_port', type: 'number', min: 1, max: 65535 },
      { path: 'smtp.username', label: 'nc_smtp_user' },
      { path: 'smtp.password', label: 'nc_smtp_password', type: 'secret' },
      { path: 'smtp.from', label: 'nc_smtp_from', ph: 'unraid@example.org' },
      { path: 'smtp.to', label: 'nc_smtp_to', hint: 'nc_smtp_to_hint', ph: 'me@example.org' }
    ],
    webhook: [
      { path: 'webhook.url', label: 'nc_address', ph: 'https://hooks.example.org/in' },
      { path: 'webhook.method', label: 'nc_method', type: 'choice', options: [['POST', 'POST'], ['PUT', 'PUT'], ['GET', 'GET']], dflt: 'POST' },
      { path: 'webhook.headers', label: 'nc_headers', hint: 'nc_headers_hint', type: 'headers', ph: 'Authorization: Bearer ...' },
      { path: 'webhook.body', label: 'nc_body', hint: 'nc_body_hint', type: 'lines', unless: ['webhook.method', 'GET'] }
    ],
    healthchecks: [{ path: 'healthchecks.url', label: 'nc_address', ph: 'https://hc-ping.com/...' }]
  };

  function name(kind, t) {
    return BRANDS[kind] || t[WORDS[kind]] || kind;
  }

  function get(c, path) {
    return path.split('.').reduce(function (o, k) { return o == null ? undefined : o[k]; }, c);
  }

  function set(c, path, v) {
    var keys = path.split('.'), o = c;
    keys.slice(0, -1).forEach(function (k) { if (!o[k] || typeof o[k] !== 'object') o[k] = {}; o = o[k]; });
    o[keys[keys.length - 1]] = v;
  }

  // newChannel is a channel of kind, switched on and still empty, under an
  // id no other channel has.
  function newChannel(kind, channels) {
    var taken = {};
    (channels || []).forEach(function (c) { taken[c.id] = true; });
    var id = kind;
    for (var n = 2; taken[id]; n++) id = kind + '-' + n;
    var c = { id: id, kind: kind, enabled: true, on: 'always' };
    switch (kind) {
      case 'unraid': break;
      case 'webhook': c.webhook = { url: '', method: 'POST', headers: { 'Content-Type': 'application/json' }, body: WEBHOOK_BODY }; break;
      case 'matrix': c.matrix = { homeserver: '', room: '', token: '' }; break;
      case 'smtp': c.smtp = { host: '', from: '', to: '' }; break;
      case 'healthchecks': c.healthchecks = {}; break;
      default: c[kind] = { url: '' };
    }
    return c;
  }

  // Lowercases and drops accents and hyphens, so "e-mail" finds "Email".
  function fold(s) {
    return String(s).normalize('NFD').replace(/[̀-ͯ]/g, '').replace(/ß/g, 'ss').replace(/-/g, '').toLowerCase();
  }

  // Whether every word of the query turns up in at least one of the names.
  function matches(query, names) {
    var said = names.map(fold);
    return fold(query).split(/\s+/).filter(Boolean).every(function (w) {
      return said.some(function (n) { return n.indexOf(w) >= 0; });
    });
  }

  /* sections are the tiles of the kinds offered that the query leaves: the
   * services, then mail and webhooks, each by the name shown in the reader's
   * order. A kind a page can have once is off while it is there. */
  function sections(kinds, channels, t, lang, query) {
    var have = {};
    (channels || []).forEach(function (c) { have[c.kind] = true; });
    function tiles(standard) {
      return kinds.filter(function (k) { return (STANDARDS.indexOf(k) >= 0) === standard && matches(query || '', [name(k, t), k]); })
        .map(function (k) { return { kind: k, name: name(k, t), hint: t[HINTS[k]] || '', off: SINGLE.indexOf(k) >= 0 && !!have[k] }; })
        .sort(function (a, b) { return a.name.localeCompare(b.name, lang, { sensitivity: 'base' }); });
    }
    return [{ id: 'services', title: t.nc_services, tiles: tiles(false) }, { id: 'standards', title: t.nc_standards, tiles: tiles(true) }]
      .filter(function (s) { return s.tiles.length; });
  }

  /* Where a key moves the focus from tile from, given every tile's box in
   * order: the tile beside it, the nearest one in the row above or below,
   * the first or the last. Up from the first row is the search. Null for a
   * key the grid leaves alone. GlimStone's pickKey. */
  function pickKey(key, from, boxes, rtl) {
    var last = boxes.length - 1;
    if (key === 'Home') return 0;
    if (key === 'End') return last;
    if (key === (rtl ? 'ArrowLeft' : 'ArrowRight')) return Math.min(from + 1, last);
    if (key === (rtl ? 'ArrowRight' : 'ArrowLeft')) return Math.max(from - 1, 0);
    if (key !== 'ArrowUp' && key !== 'ArrowDown') return null;
    var here = boxes[from], down = key === 'ArrowDown';
    function centre(b) { return b.left + b.width / 2; }
    // A row is every tile whose top lies within half a tile of the first one's.
    function sameRow(a, b) { return Math.abs(a.top - b.top) < Math.min(a.height, b.height) / 2; }
    var beyond = boxes.map(function (box, index) { return { box: box, index: index }; })
      .filter(function (o) { return !sameRow(o.box, here) && (down ? o.box.top > here.top : o.box.top < here.top); });
    if (!beyond.length) return down ? from : 'search';
    var near = beyond.reduce(function (a, b) { return Math.abs(b.box.top - here.top) < Math.abs(a.box.top - here.top) ? b : a; });
    return beyond.filter(function (o) { return sameRow(o.box, near.box); })
      .reduce(function (a, b) { return Math.abs(centre(b.box) - centre(here)) < Math.abs(centre(a.box) - centre(here)) ? b : a; }).index;
  }

  // The text a field shows. A kept secret shows nothing, a list one line.
  function shown(f, v) {
    if (f.type === 'secret') return v === MASK ? '' : v || '';
    if (f.type === 'words') return (v || []).join(' ');
    if (f.type === 'headers') return Object.keys(v || {}).sort().map(function (k) { return k + ': ' + v[k]; }).join('\n');
    if (f.type === 'number') return v == null || v === '' ? (f.dflt == null ? '' : String(f.dflt)) : String(v);
    if (f.type === 'choice') return v || f.dflt;
    return v == null ? '' : String(v);
  }

  // The value a field's text stands for. An empty secret field keeps the
  // secret the service holds.
  function taken(f, text, before) {
    var s = String(text == null ? '' : text);
    if (f.type === 'secret') return s.trim() === '' && before === MASK ? MASK : s.trim();
    if (f.type === 'words') return s.split(/\s+/).filter(Boolean);
    if (f.type === 'number') { var n = parseInt(s, 10); return isNaN(n) ? undefined : n; }
    if (f.type === 'headers') {
      var out = {};
      s.split('\n').forEach(function (l) {
        var i = l.indexOf(':');
        if (i > 0 && l.slice(0, i).trim()) out[l.slice(0, i).trim()] = l.slice(i + 1).trim();
      });
      return out;
    }
    return f.type === 'lines' ? s : s.trim();
  }

  // The fields a channel shows, its name and what it sends first.
  function fieldsOf(c, t) {
    return [
      { path: 'name', label: 'nc_name', hint: 'nc_name_hint', ph: name(c.kind, t) },
      { path: 'on', label: 'nc_on', type: 'choice', options: [['always', 'nc_on_always'], ['failure', 'nc_on_failure']], dflt: 'always' }
    ].concat((FIELDS[c.kind] || []).filter(function (f) {
      return !(f.unless && String(get(c, f.unless[0]) || '').toUpperCase() === f.unless[1]);
    }));
  }

  function fail(t, code) {
    return t['nc_fail_' + code] || t.nc_test_failed;
  }

  function mount(el, adapter, opts) {
    var doc = el.ownerDocument || document;
    var t = opts.t || {}, lang = opts.lang || 'en', kinds = opts.kinds || Object.keys(FIELDS);
    var marks = (opts.marks && opts.marks.marks) || {}, lit = (opts.marks && opts.marks.tiles) || {};
    var cls = Object.assign({ input: '', button: '', switch: '' }, opts.classes);
    var list = [], typing = 0, problems = {}, addBtn = null;

    function h(tag, attrs, kids) {
      var n = doc.createElement(tag);
      Object.keys(attrs || {}).forEach(function (k) {
        if (k === 'text') n.textContent = attrs[k];
        else if (k === 'class') n.className = attrs[k];
        else if (attrs[k] != null && attrs[k] !== false) n.setAttribute(k, attrs[k] === true ? '' : attrs[k]);
      });
      (kids || []).forEach(function (c) { if (c) n.appendChild(c); });
      return n;
    }

    // The marks and glyphs are fixed strings of this file and its marks
    // file, never anything a channel holds.
    function mark(kind, extra) {
      var m = h('span', { class: 'glim-nc-mark' + (extra ? ' ' + extra : ''), 'aria-hidden': 'true' });
      m.innerHTML = marks[kind] || '';
      return m;
    }

    function info(tip) {
      if (!tip) return null;
      var i = h('span', { class: 'glim-nc-info', tabindex: '0', role: 'img', 'aria-label': tip, 'data-tip': tip });
      i.innerHTML = INFO;
      return i;
    }

    function button(text, extra, onclick) {
      var b = h('button', { type: 'button', class: (cls.button + ' glim-nc-btn ' + (extra || '')).trim(), text: text });
      b.addEventListener('click', onclick);
      return b;
    }

    function byId(id) {
      for (var i = 0; i < list.length; i++) if (list[i].id === id) return list[i];
      return null;
    }

    function save(now) {
      clearTimeout(typing);
      if (!now) { typing = setTimeout(function () { save(true); }, 700); return; }
      var sent = JSON.parse(JSON.stringify(list));
      adapter.save(sent).then(function (saved) {
        problems = {};
        // The fields keep what is typed in them, so nothing jumps under the
        // cursor. The list is the one the service kept.
        list = saved;
        if (opts.saved) opts.saved(true, t.nc_saved_line || '');
      }, function (e) {
        problems = {};
        ((e && e.problems) || []).forEach(function (p) { problems[(sent[p.index] || {}).id] = p; });
        if (opts.saved) opts.saved(false, (e && e.code) === 'invalid' ? t.nc_not_saved + ': ' + t.nc_fail_invalid : t.nc_not_saved);
        draw();
      });
    }

    /* One field of the channel that of() returns. changed(done, redraw) hears
     * every edit: done once the edit is over, redraw when other fields may
     * come or go with it. On the page a choice is a select, as the page's
     * others are; in the window it is a row of segments, since the window
     * draws its own controls. */
    function field(of, f, changed, inWindow) {
      var c = of();
      var id = 'glim-nc-' + (inWindow ? 'new' : c.id) + '-' + f.path.replace(/\./g, '-');
      var v = get(c, f.path), input;
      var fieldCls = inWindow ? 'glim-nc-in glim-nc-win-in' : (cls.input + ' glim-nc-in').trim();
      function take(text, done) {
        var ch = of();
        if (!ch) return;
        set(ch, f.path, taken(f, text, get(ch, f.path)));
        changed(done, f.type === 'choice');
      }
      if (f.type === 'choice' && inWindow) {
        input = h('span', { id: id, class: 'glim-nc-seg', role: 'radiogroup' });
        f.options.forEach(function (o) {
          var on = o[0] === shown(f, v);
          var s = h('button', { type: 'button', role: 'radio', class: 'glim-nc-seg-opt', 'aria-checked': String(on), tabindex: on ? '0' : '-1', 'data-value': o[0], text: t[o[1]] || o[1] });
          s.addEventListener('click', function () { take(o[0], true); });
          input.appendChild(s);
        });
        input.addEventListener('keydown', function (e) {
          var i = f.options.map(function (o) { return o[0]; }).indexOf(shown(f, get(of(), f.path)));
          var step = { ArrowRight: 1, ArrowDown: 1, ArrowLeft: -1, ArrowUp: -1 }[e.key];
          if (!step) return;
          e.preventDefault();
          take(f.options[(i + step + f.options.length) % f.options.length][0], true);
        });
      } else if (f.type === 'choice') {
        input = h('select', { id: id, class: fieldCls + ' glim-nc-select' });
        f.options.forEach(function (o) { input.appendChild(h('option', { value: o[0], text: t[o[1]] || o[1] })); });
        input.value = shown(f, v);
        input.addEventListener('change', function () { take(input.value, true); });
      } else {
        var lines = f.type === 'lines' || f.type === 'headers';
        input = lines
          ? h('textarea', { id: id, class: fieldCls + ' glim-nc-lines', rows: '3', spellcheck: 'false', placeholder: f.ph })
          : h('input', {
            id: id, class: fieldCls, spellcheck: 'false',
            type: f.type === 'secret' ? 'password' : f.type === 'number' ? 'number' : 'text',
            autocomplete: f.type === 'secret' ? 'new-password' : 'off',
            placeholder: f.type === 'secret' && v === MASK ? t.nc_saved : f.ph, min: f.min, max: f.max
          });
        input.value = shown(f, v);
        input.addEventListener('input', function () { take(input.value, false); });
        input.addEventListener('change', function () { take(input.value, true); });
      }
      var lbl = f.type === 'choice' && inWindow
        ? h('span', { class: 'glim-nc-name', id: id + '-l', text: t[f.label] || f.label })
        : h('label', { class: 'glim-nc-name', for: id, text: t[f.label] || f.label });
      if (f.type === 'choice' && inWindow) input.setAttribute('aria-labelledby', id + '-l');
      return h('div', { class: 'glim-nc-row' }, [h('span', { class: 'glim-nc-lbl' }, [lbl, info(t[f.hint])]), h('div', { class: 'glim-nc-ctl' }, [input])]);
    }

    // The test of a channel, its button and the line that says how it went.
    function tester(of, make) {
      var res = h('span', { class: 'glim-nc-res', role: 'status' });
      var b = (make || button)(t.nc_test, '', function () {
        var ch = of();
        if (!ch) return;
        res.className = 'glim-nc-res glim-nc-run';
        res.textContent = t.nc_testing;
        adapter.test(JSON.parse(JSON.stringify(ch))).then(function (r) {
          res.className = 'glim-nc-res ' + (r.ok ? 'glim-nc-ok' : 'glim-nc-bad');
          res.textContent = r.ok ? t.nc_test_ok : t.nc_test_failed + ': ' + fail(t, r.code);
        }, function () {
          res.className = 'glim-nc-res glim-nc-bad';
          res.textContent = t.nc_test_failed;
        });
      });
      return { button: b, result: res };
    }

    function channel(c) {
      var title = (c.name || '').trim() || name(c.kind, t);
      var of = function () { return byId(c.id); };
      var sw = h('input', { type: 'checkbox', role: 'switch', class: (cls.switch + ' glim-nc-switch').trim(), 'aria-label': t.nc_use + ', ' + title });
      sw.checked = !!c.enabled;
      sw.addEventListener('change', function () {
        var ch = of();
        if (!ch) return;
        ch.enabled = sw.checked;
        draw();
        save(true);
      });
      var card = h('section', { class: 'glim-nc-ch', 'data-kind': c.kind }, [
        h('div', { class: 'glim-nc-ch-hd' }, [mark(c.kind), h('span', { class: 'glim-nc-title', text: title }), info(t[HINTS[c.kind]]), sw])
      ]);
      if (problems[c.id]) card.appendChild(h('p', { class: 'glim-nc-err', role: 'alert', text: fail(t, problems[c.id].code) }));
      if (c.enabled) {
        fieldsOf(c, t).forEach(function (f) {
          card.appendChild(field(of, f, function (done, redraw) {
            if (redraw) draw();
            save(done || redraw);
          }));
        });
      }
      var test = tester(of);
      test.button.disabled = !c.enabled;
      var remove = button(t.nc_remove, '', function () {
        list = list.filter(function (o) { return o.id !== c.id; });
        draw();
        save(true);
      });
      card.appendChild(h('div', { class: 'glim-nc-tools' }, [test.button, test.result, h('span', { class: 'glim-nc-gap' }), remove]));
      return card;
    }

    var FOCUSABLE = 'button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

    /* The picker window: its title as a badge on the top edge with the (i),
     * the search at the top end and the tiles in their groups under it, which
     * alone scroll and keep the height they opened with while a query narrows
     * them. A picked tile's fields take the tiles' place, with Back and Save
     * beside Cancel in the foot. Escape empties the search first, then closes;
     * the focus goes back to Add. */
    function picker() {
      var query = '', draft = null, busy = false, scroll = null, search = null;
      var title = h('h2', { class: 'glim-nc-win-title', id: 'glim-nc-win-t' }, [h('span', { text: t.nc_add }), info(t.nc_add_hint)]);
      var body = h('div', { class: 'glim-nc-win-body' });
      var act = h('div', { class: 'glim-nc-win-act' });
      var box = h('div', { class: 'glim-nc-win', role: 'dialog', 'aria-modal': 'true', 'aria-labelledby': title.id, tabindex: '-1' }, [title, body, act]);
      var scrim = h('div', { class: 'glim-nc-scrim' });
      var wins = h('div', { class: 'glim-nc-wins' }, [scrim, box]);
      doc.body.appendChild(wins);

      function close() {
        if (!wins.parentNode) return;
        wins.parentNode.removeChild(wins);
        doc.removeEventListener('keydown', onKey, true);
        (addBtn && addBtn.isConnected ? addBtn : el).focus();
      }

      function winButton(text, extra, onclick) {
        var b = h('button', { type: 'button', class: 'glim-nc-win-btn' + (extra ? ' ' + extra : ''), text: text });
        b.addEventListener('click', onclick);
        return b;
      }

      function tilesOf() {
        return scroll ? Array.prototype.filter.call(scroll.querySelectorAll('button.glim-nc-tile'), function (b) { return !b.disabled; }) : [];
      }

      function grid() {
        scroll.textContent = '';
        var secs = sections(kinds, list, t, lang, query);
        if (!secs.length) scroll.appendChild(h('p', { class: 'glim-nc-none', text: (t.nc_none || '').replace('{query}', query.trim()) }));
        secs.forEach(function (s) {
          var ul = h('ul', { class: 'glim-nc-tiles' });
          s.tiles.forEach(function (tile) {
            var b = h('button', { type: 'button', class: 'glim-nc-tile', 'data-kind': tile.kind }, [mark(tile.kind, 'glim-nc-tile-mark'), h('span', { class: 'glim-nc-tile-name', text: tile.name })]);
            b.disabled = tile.off;
            b.addEventListener('click', function () { form(newChannel(tile.kind, list)); });
            var li = h('li', { class: 'glim-nc-tilebox' + (tile.hint ? ' glim-nc-tile-hinted' : '') }, [b, info(tile.hint)]);
            var l = lit[tile.kind];
            if (l) li.setAttribute('style', '--nc-tile:' + l.tile + ';--nc-tile-ink:' + l.ink);
            ul.appendChild(li);
          });
          scroll.appendChild(h('section', { class: 'glim-nc-sec' }, [h('h3', { class: 'glim-nc-eyebrow', text: s.title }), ul]));
        });
      }

      function tiles() {
        draft = null;
        body.className = 'glim-nc-win-body glim-nc-win-pick';
        body.textContent = '';
        act.textContent = '';
        search = h('input', { type: 'search', class: 'glim-nc-search', 'aria-label': t.nc_search, placeholder: t.nc_search, spellcheck: 'false', autocomplete: 'off' });
        search.value = query;
        search.addEventListener('input', function () { query = search.value; grid(); });
        scroll = h('div', { class: 'glim-nc-scroll' });
        scroll.addEventListener('keydown', function (e) {
          var all = tilesOf(), from = all.indexOf(e.target);
          if (from < 0) return;
          var rtl = doc.defaultView && doc.defaultView.getComputedStyle(scroll).direction === 'rtl';
          var to = pickKey(e.key, from, all.map(function (b) { return b.getBoundingClientRect(); }), rtl);
          if (to === null) return;
          e.preventDefault();
          (to === 'search' ? search : all[to]).focus();
        });
        body.appendChild(search);
        body.appendChild(scroll);
        grid();
        act.appendChild(winButton(t.nc_close, '', close));
        if (scroll.clientHeight) scroll.style.minHeight = scroll.clientHeight + 'px';
        search.focus();
      }

      function form(c) {
        draft = c;
        var of = function () { return draft; };
        var err = h('p', { class: 'glim-nc-err', role: 'alert' });
        var test = tester(of, winButton);
        function fill() {
          body.className = 'glim-nc-win-body';
          body.textContent = '';
          body.appendChild(h('div', { class: 'glim-nc-ch-hd' }, [mark(c.kind), h('span', { class: 'glim-nc-title', text: name(c.kind, t) }), info(t[HINTS[c.kind]])]));
          var rows = h('div', { class: 'glim-nc-form' });
          fieldsOf(draft, t).forEach(function (f) {
            rows.appendChild(field(of, f, function (done, redraw) {
              if (!redraw) return;
              // A segment's group keeps the focus, on the segment chosen.
              var on = doc.activeElement, at = on && (on.id || (on.parentNode && on.parentNode.id));
              fill();
              var back = at ? doc.getElementById(at) : null;
              if (back && back.getAttribute('role') === 'radiogroup') back = back.querySelector('[aria-checked="true"]');
              if (back) back.focus();
            }, true));
          });
          body.appendChild(rows);
          body.appendChild(h('div', { class: 'glim-nc-tools' }, [test.button, test.result]));
          body.appendChild(err);
        }
        fill();
        act.textContent = '';
        var keep = winButton(t.nc_save, 'glim-nc-key', function () {
          if (busy) return;
          busy = true;
          keep.disabled = true;
          err.textContent = '';
          clearTimeout(typing);
          var sent = JSON.parse(JSON.stringify(list.concat([draft])));
          adapter.save(sent).then(function (saved) {
            busy = false;
            problems = {};
            list = saved;
            if (opts.saved) opts.saved(true, t.nc_saved_line || '');
            draw();
            close();
          }, function (e) {
            busy = false;
            keep.disabled = false;
            var mine = ((e && e.problems) || []).filter(function (p) { return p.index === sent.length - 1; })[0];
            err.textContent = t.nc_not_saved + ': ' + fail(t, mine ? mine.code : (e && e.code) || 'engine');
          });
        });
        act.appendChild(winButton(t.nc_cancel, '', close));
        act.appendChild(winButton(t.nc_back, '', tiles));
        act.appendChild(keep);
        var first = body.querySelector('input, textarea');
        if (first) first.focus();
      }

      function onKey(e) {
        if (e.key === 'Escape') {
          e.preventDefault();
          e.stopPropagation();
          if (search && e.target === search && search.value !== '') {
            search.value = '';
            query = '';
            grid();
          } else if (!busy) close();
        } else if (e.key === 'Enter' && search && e.target === search && query.trim() !== '') {
          var one = tilesOf();
          if (one.length === 1) { e.preventDefault(); one[0].click(); }
        } else if (e.key === 'ArrowDown' && search && e.target === search) {
          var all = tilesOf();
          if (all[0]) { e.preventDefault(); all[0].focus(); }
        } else if (e.key === 'Tab') {
          var stops = Array.prototype.filter.call(box.querySelectorAll(FOCUSABLE), function (n) { return n.offsetParent !== null; });
          if (!stops.length) return;
          var first = stops[0], last = stops[stops.length - 1];
          if (e.shiftKey && doc.activeElement === first) { e.preventDefault(); last.focus(); }
          else if (!e.shiftKey && doc.activeElement === last) { e.preventDefault(); first.focus(); }
        }
      }
      scrim.addEventListener('click', function () { if (!busy) close(); });
      doc.addEventListener('keydown', onKey, true);
      tiles();
      return { close: close, box: box };
    }

    function draw() {
      el.textContent = '';
      var root = h('div', { class: 'glim-nc' });
      if (!list.length) root.appendChild(h('p', { class: 'glim-nc-sub', text: t.nc_empty }));
      list.forEach(function (c) { root.appendChild(channel(c)); });
      addBtn = button(t.nc_add, 'glim-nc-add', function () { picker(); });
      addBtn.setAttribute('aria-haspopup', 'dialog');
      root.appendChild(h('div', { class: 'glim-nc-addrow' }, [addBtn]));
      el.appendChild(root);
    }

    adapter.load().then(function (channels) {
      list = channels || [];
      draw();
    }, function (e) {
      if (opts.saved) opts.saved(false, fail(t, (e && e.code) || 'engine'));
    });
    return { list: function () { return list; }, draw: draw, pick: picker };
  }

  return { MASK: MASK, FIELDS: FIELDS, newChannel: newChannel, sections: sections, matches: matches, pickKey: pickKey, shown: shown, taken: taken, name: name, get: get, set: set, mount: mount };
});
