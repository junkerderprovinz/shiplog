/* The notification channels of a plugin's settings page, for a page without
 * a framework. Every channel is a card of its own with its switch, what it
 * sends, its few fields and a test. A channel is added by picking its
 * service from tiles with the service's mark, in the kit's notify format, so
 * the same file serves every plugin that keeps its channels that way.
 *
 * mount(el, adapter, opts) draws the card into el. The adapter loads the
 * channels, saves the whole list (it answers with the list as saved, secrets
 * masked) and tests one channel. opts carries the strings (t, the nc_ keys),
 * the language, the kinds the plugin offers, the marks (notify-marks.js), the
 * host's classes for fields, buttons and switches, and saved(ok, text) for
 * the page's saved line. Every change is saved at once, text a moment after
 * the last key. A secret the service keeps is never shown: its field stays
 * empty and says so, and an empty field keeps it. */
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

  /* sections are the tiles of the kinds offered: the services, then mail and
   * webhooks, each by the name shown in the reader's order. A kind a page
   * can have once is off while it is there. */
  function sections(kinds, channels, t, lang) {
    var have = {};
    (channels || []).forEach(function (c) { have[c.kind] = true; });
    function tiles(standard) {
      return kinds.filter(function (k) { return (STANDARDS.indexOf(k) >= 0) === standard; })
        .map(function (k) { return { kind: k, name: name(k, t), hint: t[HINTS[k]] || '', off: SINGLE.indexOf(k) >= 0 && !!have[k] }; })
        .sort(function (a, b) { return a.name.localeCompare(b.name, lang, { sensitivity: 'base' }); });
    }
    return [{ id: 'services', title: t.nc_services, tiles: tiles(false) }, { id: 'standards', title: t.nc_standards, tiles: tiles(true) }]
      .filter(function (s) { return s.tiles.length; });
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

  function fail(t, code) {
    return t['nc_fail_' + code] || t.nc_test_failed;
  }

  function mount(el, adapter, opts) {
    var doc = el.ownerDocument || document;
    var t = opts.t || {}, lang = opts.lang || 'en', kinds = opts.kinds || Object.keys(FIELDS);
    var marks = (opts.marks && opts.marks.marks) || {}, lit = (opts.marks && opts.marks.tiles) || {};
    var cls = Object.assign({ input: '', button: '', switch: '' }, opts.classes);
    var list = [], picking = false, typing = 0, problems = {};

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
    function mark(kind) {
      var m = h('span', { class: 'glim-nc-mark', 'aria-hidden': 'true' });
      m.innerHTML = marks[kind] || '';
      return m;
    }

    function info(tip) {
      if (!tip) return null;
      var i = h('span', { class: 'glim-nc-info', tabindex: '0', role: 'img', 'aria-label': tip, 'data-tip': tip });
      i.innerHTML = INFO;
      return i;
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

    function field(c, f) {
      var id = 'glim-nc-' + c.id + '-' + f.path.replace(/\./g, '-');
      var v = get(c, f.path), input;
      if (f.type === 'choice') {
        input = h('select', { id: id, class: (cls.input + ' glim-nc-in glim-nc-select').trim() });
        f.options.forEach(function (o) {
          var opt = h('option', { value: o[0], text: t[o[1]] || o[1] });
          input.appendChild(opt);
        });
        input.value = shown(f, v);
      } else if (f.type === 'lines' || f.type === 'headers') {
        input = h('textarea', { id: id, class: (cls.input + ' glim-nc-in glim-nc-lines').trim(), rows: '3', spellcheck: 'false', placeholder: f.ph });
        input.value = shown(f, v);
      } else {
        input = h('input', {
          id: id, class: (cls.input + ' glim-nc-in').trim(), spellcheck: 'false',
          type: f.type === 'secret' ? 'password' : f.type === 'number' ? 'number' : 'text',
          autocomplete: f.type === 'secret' ? 'new-password' : 'off',
          placeholder: f.type === 'secret' && v === MASK ? t.nc_saved : f.ph, min: f.min, max: f.max
        });
        input.value = shown(f, v);
      }
      function take(now) {
        var ch = byId(c.id);
        if (!ch) return;
        set(ch, f.path, taken(f, input.value, get(ch, f.path)));
        if (f.type === 'choice') { draw(); save(true); return; }
        save(now);
      }
      input.addEventListener(f.type === 'choice' ? 'change' : 'input', function () { take(f.type === 'choice'); });
      if (f.type !== 'choice') input.addEventListener('change', function () { take(true); });
      var lbl = h('label', { class: 'glim-nc-name', for: id, text: t[f.label] || f.label });
      return h('div', { class: 'glim-nc-row' }, [h('span', { class: 'glim-nc-lbl' }, [lbl, info(t[f.hint])]), h('div', { class: 'glim-nc-ctl' }, [input])]);
    }

    function channel(c) {
      var title = (c.name || '').trim() || name(c.kind, t);
      var sw = h('input', { type: 'checkbox', role: 'switch', class: (cls.switch + ' glim-nc-switch').trim(), 'aria-label': t.nc_use + ', ' + title });
      sw.checked = !!c.enabled;
      sw.addEventListener('change', function () {
        var ch = byId(c.id);
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
        var nameIn = { path: 'name', label: 'nc_name', hint: 'nc_name_hint', ph: name(c.kind, t) };
        card.appendChild(field(c, nameIn));
        card.appendChild(field(c, { path: 'on', label: 'nc_on', type: 'choice', options: [['always', 'nc_on_always'], ['failure', 'nc_on_failure']], dflt: 'always' }));
        (FIELDS[c.kind] || []).forEach(function (f) {
          if (f.unless && String(get(c, f.unless[0]) || '').toUpperCase() === f.unless[1]) return;
          card.appendChild(field(c, f));
        });
      }
      var res = h('span', { class: 'glim-nc-res', role: 'status' });
      var test = h('button', { type: 'button', class: (cls.button + ' glim-nc-btn').trim(), text: t.nc_test });
      test.disabled = !c.enabled;
      test.addEventListener('click', function () {
        var ch = byId(c.id);
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
      var remove = h('button', { type: 'button', class: (cls.button + ' glim-nc-btn').trim(), text: t.nc_remove });
      remove.addEventListener('click', function () {
        list = list.filter(function (o) { return o.id !== c.id; });
        draw();
        save(true);
      });
      card.appendChild(h('div', { class: 'glim-nc-tools' }, [test, res, h('span', { class: 'glim-nc-gap' }), remove]));
      return card;
    }

    function picker() {
      var box = h('div', { class: 'glim-nc-pick' }, [h('p', { class: 'glim-nc-sub', text: t.nc_add_hint })]);
      sections(kinds, list, t, lang).forEach(function (s) {
        box.appendChild(h('h4', { class: 'glim-nc-pick-h', text: s.title }));
        var ul = h('ul', { class: 'glim-nc-tiles' });
        s.tiles.forEach(function (tile) {
          var b = h('button', { type: 'button', class: 'glim-nc-tile', 'data-kind': tile.kind }, [mark(tile.kind), h('span', { class: 'glim-nc-tile-name', text: tile.name })]);
          var l = lit[tile.kind];
          if (l) b.setAttribute('style', '--nc-tile:' + l.tile + ';--nc-tile-ink:' + l.ink);
          b.disabled = tile.off;
          b.addEventListener('click', function () {
            list = list.concat([newChannel(tile.kind, list)]);
            picking = false;
            draw();
            save(true);
          });
          ul.appendChild(h('li', { class: 'glim-nc-tilebox' }, [b, info(tile.hint)]));
        });
        box.appendChild(ul);
      });
      return box;
    }

    function draw() {
      el.textContent = '';
      var root = h('div', { class: 'glim-nc' });
      if (!list.length) root.appendChild(h('p', { class: 'glim-nc-sub', text: t.nc_empty }));
      list.forEach(function (c) { root.appendChild(channel(c)); });
      var add = h('button', { type: 'button', class: (cls.button + ' glim-nc-btn glim-nc-add').trim(), 'aria-expanded': String(picking), text: picking ? t.nc_close : t.nc_add });
      add.addEventListener('click', function () { picking = !picking; draw(); });
      root.appendChild(h('div', { class: 'glim-nc-addrow' }, [add]));
      if (picking) root.appendChild(picker());
      el.appendChild(root);
    }

    adapter.load().then(function (channels) {
      list = channels || [];
      draw();
    }, function (e) {
      if (opts.saved) opts.saved(false, fail(t, (e && e.code) || 'engine'));
    });
    return { list: function () { return list; }, draw: draw };
  }

  return { MASK: MASK, FIELDS: FIELDS, newChannel: newChannel, sections: sections, shown: shown, taken: taken, name: name, get: get, set: set, mount: mount };
});
