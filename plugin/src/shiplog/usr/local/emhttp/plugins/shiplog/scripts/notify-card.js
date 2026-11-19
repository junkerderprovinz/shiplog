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

  // GlimStone's infoIcon(), the stroked (i) every app of the house wears.
  var INFO = '<svg viewBox="0 0 16 16" fill="none" aria-hidden="true" focusable="false"><circle cx="8" cy="8" r="7" stroke="currentColor" stroke-width="1.3"/><circle cx="8" cy="4.6" r="0.9" fill="currentColor"/><path d="M8 7v4.4" stroke="currentColor" stroke-width="1.3" stroke-linecap="round"/></svg>';

  // The glyphs of GlimStone's glyphs.json the card's buttons wear before their words.
  var GLYPHS = {
    add: "<svg viewBox=\"0.006 0.006 13.988 13.988\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><path fill-rule=\"evenodd\" d=\"M5.756 12.873 L5.774 12.994 L5.845 13.228 L5.961 13.444 L6.034 13.543 L6.207 13.716 L6.411 13.852 L6.522 13.905 L6.756 13.976 L6.877 13.994 L7.123 13.994 L7.244 13.976 L7.478 13.905 L7.694 13.789 L7.793 13.716 L7.966 13.543 L8.102 13.339 L8.155 13.228 L8.226 12.994 L8.25 12.75 L8.25 8.25 L12.75 8.25 L12.994 8.226 L13.228 8.155 L13.444 8.039 L13.543 7.966 L13.716 7.793 L13.789 7.694 L13.905 7.478 L13.976 7.244 L13.994 7.123 L13.994 6.877 L13.976 6.756 L13.905 6.522 L13.789 6.306 L13.634 6.116 L13.444 5.961 L13.228 5.845 L12.994 5.774 L12.75 5.75 L8.25 5.75 L8.25 1.25 L8.226 1.006 L8.155 0.772 L8.039 0.556 L7.884 0.366 L7.793 0.284 L7.589 0.148 L7.478 0.095 L7.363 0.054 L7.123 0.006 L6.877 0.006 L6.637 0.054 L6.411 0.148 L6.306 0.211 L6.116 0.366 L5.961 0.556 L5.845 0.772 L5.774 1.006 L5.75 1.25 L5.75 5.75 L1.25 5.75 L1.006 5.774 L0.887 5.804 L0.661 5.898 L0.556 5.961 L0.366 6.116 L0.211 6.306 L0.148 6.411 L0.054 6.637 L0.006 6.877 L0.006 7.123 L0.054 7.363 L0.148 7.589 L0.284 7.793 L0.457 7.966 L0.661 8.102 L0.772 8.155 L1.006 8.226 L1.25 8.25 L5.75 8.25Z\" /></svg>",
    test: "<svg viewBox=\"0 0 14 14\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><g> <path fill-rule=\"evenodd\" d=\"M13.8536 0.146461c0.1397 0.13972 0.184 0.348619 0.1131 0.533043L8.96667 13.6795c-0.07232 0.188 -0.25043 0.3143 -0.45181 0.3203 -0.20138 0.006 -0.38667 -0.1095 -0.47004 -0.2929L5.95386 9.1068l3.07647 -3.07647c0.29289 -0.29289 0.29289 -0.76777 0 -1.06066s-0.76777 -0.29289 -1.06066 0L4.89319 8.04615 0.2931 5.9552c-0.183409 -0.08337 -0.29886581 -0.26867 -0.292877753 -0.47005C0.00621034 5.28378 0.132474 5.10566 0.320512 5.03334L13.3205 0.0333416c0.1844 -0.0709322 0.3933 -0.02660085 0.5331 0.1131194Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path> </g></svg>",
    remove: "<svg viewBox=\"0 0 14 14\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><g> <path fill-rule=\"evenodd\" d=\"M5.76256 2.01256C6.09075 1.68437 6.53587 1.5 7 1.5c0.46413 0 0.90925 0.18437 1.23744 0.51256 0.20736 0.20737 0.35731 0.46141 0.43961 0.73744h-3.3541c0.0823 -0.27603 0.23225 -0.53007 0.43961 -0.73744ZM3.78868 2.75c0.10537 -0.67679 0.42285 -1.30773 0.91322 -1.798097C5.3114 0.34241 6.13805 0 7 0c0.86195 0 1.6886 0.34241 2.2981 0.951903 0.49037 0.490367 0.8079 1.121307 0.9132 1.798097H13c0.4142 0 0.75 0.33579 0.75 0.75 0 0.41422 -0.3358 0.75 -0.75 0.75h-1v8.25c0 0.3978 -0.158 0.7794 -0.4393 1.0607S10.8978 14 10.5 14h-7c-0.39783 0 -0.77936 -0.158 -1.06066 -0.4393C2.15804 13.2794 2 12.8978 2 12.5V4.25H1c-0.414214 0 -0.75 -0.33578 -0.75 -0.75 0 -0.41421 0.335786 -0.75 0.75 -0.75h2.78868ZM5 5.87646c0.34518 0 0.625 0.27983 0.625 0.625V10.503c0 0.3451 -0.27982 0.625 -0.625 0.625s-0.625 -0.2799 -0.625 -0.625V6.50146c0 -0.34517 0.27982 -0.625 0.625 -0.625Zm4.625 0.625c0 -0.34517 -0.27982 -0.625 -0.625 -0.625s-0.625 0.27983 -0.625 0.625V10.503c0 0.3451 0.27982 0.625 0.625 0.625s0.625 -0.2799 0.625 -0.625V6.50146Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path> </g></svg>",
    close: "<svg viewBox=\"0 0 14 14\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><path fill-rule=\"evenodd\" d=\"M11.957 13.716 L12.056 13.789 L12.272 13.905 L12.506 13.976 L12.75 14 L12.994 13.976 L13.228 13.905 L13.444 13.789 L13.543 13.716 L13.716 13.543 L13.789 13.444 L13.905 13.228 L13.976 12.994 L14 12.75 L13.976 12.506 L13.905 12.272 L13.852 12.161 L13.789 12.056 L13.634 11.866 L8.768 7 L13.716 2.043 L13.789 1.944 L13.905 1.728 L13.976 1.494 L14 1.25 L13.976 1.006 L13.905 0.772 L13.789 0.556 L13.716 0.457 L13.543 0.284 L13.339 0.148 L13.113 0.054 L12.873 0.006 L12.627 0.006 L12.387 0.054 L12.272 0.095 L12.056 0.211 L11.866 0.366 L7 5.232 L2.043 0.284 L1.839 0.148 L1.613 0.054 L1.494 0.024 L1.25 0 L1.127 0.006 L0.887 0.054 L0.661 0.148 L0.457 0.284 L0.284 0.457 L0.211 0.556 L0.095 0.772 L0.024 1.006 L0 1.25 L0.024 1.494 L0.095 1.728 L0.211 1.944 L0.366 2.134 L5.232 7 L0.284 11.957 L0.211 12.056 L0.095 12.272 L0.024 12.506 L0 12.75 L0.024 12.994 L0.095 13.228 L0.211 13.444 L0.284 13.543 L0.457 13.716 L0.661 13.852 L0.772 13.905 L1.006 13.976 L1.25 14 L1.494 13.976 L1.728 13.905 L1.944 13.789 L2.134 13.634 L7 8.768Z\" /></svg>",
    back: "<svg viewBox=\"4.25 3.6 12.8 12.8\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><path d=\"M6.6 10 13 3.6l1.7 1.7L10 10l4.7 4.7-1.7 1.7L6.6 10Z\" /></svg>",
    search: "<svg viewBox=\"-0.001 0 13.988 13.988\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><path fill-rule=\"evenodd\" d=\"M2 6a4 4 0 1 1 8 0 4 4 0 0 1 -8 0Zm4 -6a6 6 0 1 0 3.476 10.89l2.817 2.817a1 1 0 0 0 1.414 -1.414l-2.816 -2.816A6 6 0 0 0 6 0Z\" clip-rule=\"evenodd\"/></svg>",
    save: "<svg viewBox=\"3 3 18 18\" fill=\"currentColor\" aria-hidden=\"true\" focusable=\"false\"><path d=\"M15,9H5V5H15M12,19A3,3 0 0,1 9,16A3,3 0 0,1 12,13A3,3 0 0,1 15,16A3,3 0 0,1 12,19M17,3H5C3.89,3 3,3.9 3,5V19A2,2 0 0,0 5,21H19A2,2 0 0,0 21,19V7L17,3Z\" /></svg>"
  };

  /* GlimStone's NOTIFY_PROVIDERS (@glimstone/core/notifyProviders) without
   * the marks, which notify-marks.js carries. The services keep their own
   * names in every language, and mail and webhooks go by the page's words. */
  var SHOUTRRR = ['Bark', 'Google Chat', 'Home Assistant', 'IFTTT', 'Join', 'Lark', 'Mattermost', 'MQTT', 'Opsgenie', 'PagerDuty', 'Pushbullet',
    'Pushover', 'Rocket.Chat', 'Signal', 'Teams', 'Telegram', 'Twilio', 'WeCom', 'XMPP', 'Zulip'];
  var CATALOG = {
    groups: ['service', 'standard'],
    entries: [
      { id: 'apprise', name: 'Apprise', group: 'service' },
      { id: 'discord', name: 'Discord', group: 'service' },
      { id: 'gotify', name: 'Gotify', group: 'service' },
      { id: 'healthchecks', name: 'Healthchecks.io', group: 'service' },
      { id: 'matrix', name: 'Matrix', group: 'service', terms: ['Element'] },
      { id: 'ntfy', name: 'ntfy', group: 'service' },
      { id: 'shoutrrr', name: 'Shoutrrr', group: 'service', terms: SHOUTRRR },
      { id: 'slack', name: 'Slack', group: 'service' },
      { id: 'unraid', name: 'Unraid', group: 'service' },
      { id: 'smtp', name: 'Email', kind: 'SMTP', group: 'standard', glyph: 'IconMail', word: true },
      { id: 'webhook', name: 'Webhook', group: 'standard', terms: ['HTTP'], glyph: 'IconLink', word: true }
    ]
  };
  var WORDS = { smtp: 'nc_smtp', webhook: 'nc_webhook' };
  var HINTS = {
    unraid: 'nc_unraid_hint', discord: 'nc_discord_hint', slack: 'nc_slack_hint', ntfy: 'nc_ntfy_hint', gotify: 'nc_gotify_hint',
    matrix: 'nc_matrix_hint', apprise: 'nc_apprise_hint', smtp: 'nc_smtp_hint', webhook: 'nc_webhook_hint', healthchecks: 'nc_hc_hint'
  };
  // The webhook takes anything, so it stands last.
  var LAST = 'webhook';
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

  function entry(kind) {
    for (var i = 0; i < CATALOG.entries.length; i++) if (CATALOG.entries[i].id === kind) return CATALOG.entries[i];
    return null;
  }

  function name(kind, t) {
    var e = entry(kind);
    if (!e) return kind;
    return e.word ? t[WORDS[kind]] || e.name : e.name;
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

  /* GlimStone's catalogPicker: the tiles and groups of a picker for the ids
   * an app offers, group by group in the catalog's order and alphabetical
   * within one, and only the groups that keep a tile. An id the catalog does
   * not know is left out. */
  function catalogPicker(catalog, pick) {
    var rank = function (g) { return catalog.groups.indexOf(g); };
    var items = catalog.entries.filter(function (e) { return pick.ids.indexOf(e.id) >= 0; }).map(function (e) {
      var item = Object.assign({ id: e.id, label: e.name, group: e.group }, pick.own ? pick.own(e) : undefined);
      if (e.kind && item.sub === undefined) item.sub = e.kind;
      // A translated name still finds the provider by the name its own documentation uses.
      var terms = (item.label !== e.name ? [e.name] : []).concat(e.terms || [], item.terms || []);
      if (terms.length) item.terms = terms;
      if (item.mark == null) item.mark = e.mark != null ? e.mark : pick.glyph(e);
      return item;
    }).sort(function (a, b) {
      return rank(a.group) - rank(b.group) || a.label.localeCompare(b.label, pick.lang, { sensitivity: 'base' });
    });
    var groups = catalog.groups.filter(function (g) {
      return items.some(function (i) { return i.group === g; });
    }).map(function (g) {
      var title = pick.titles[g];
      return typeof title === 'string' ? { id: g, title: title } : Object.assign({ id: g }, title);
    });
    return { items: items, groups: groups };
  }

  /* sections are the tiles of the kinds offered that the query leaves, as
   * the catalog groups them: the services, then mail and webhooks, each by
   * the name shown in the reader's order with the webhook last. The search
   * reads the name, its second line, the kind and the catalog's words. A
   * kind a page can have once is off while it is there. */
  function sections(kinds, channels, t, lang, query) {
    var have = {};
    (channels || []).forEach(function (c) { have[c.kind] = true; });
    var pick = catalogPicker(CATALOG, {
      ids: kinds, titles: { service: t.nc_services, standard: t.nc_standards }, lang: lang,
      glyph: function (e) { return e.id; },
      own: function (e) { return { label: name(e.id, t), hint: t[HINTS[e.id]] || '' }; }
    });
    return pick.groups.map(function (g) {
      var tiles = pick.items.filter(function (i) {
        return i.group === g.id && matches(query || '', [i.label, i.sub || '', i.id].concat(i.terms || []));
      }).sort(function (a, b) { return Number(a.id === LAST) - Number(b.id === LAST); });
      return {
        id: g.id, title: g.title,
        tiles: tiles.map(function (i) { return { kind: i.id, name: i.label, sub: i.sub || '', hint: i.hint, off: SINGLE.indexOf(i.id) >= 0 && !!have[i.id] }; })
      };
    }).filter(function (s) { return s.tiles.length; });
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

  /* GlimStone's one floating bubble for an (i): a body child placed from the
   * trigger's box, clamped 8px into the viewport, above the trigger when below
   * would clip, with its arrow on the trigger. Focus opens it only after a key,
   * a press, Escape or a scroll closes it. The theme plugin draws the same
   * bubble for every data-tip of the page, so under its look the card leaves
   * the (i) to it. */
  var bubbled = null;
  function wireBubble(doc) {
    if (bubbled === doc) return;
    bubbled = doc;
    var bubble = null, on = null, keys = false;
    function hide() {
      if (bubble) bubble.style.display = 'none';
      on = null;
    }
    function show(trigger) {
      if (!bubble) {
        bubble = doc.createElement('div');
        bubble.className = 'glim-nc-bubble';
        doc.body.appendChild(bubble);
      }
      var r = trigger.getBoundingClientRect(), root = doc.documentElement;
      bubble.textContent = trigger.getAttribute('data-tip');
      bubble.style.display = 'block';
      var w = bubble.offsetWidth, ht = bubble.offsetHeight, cx = r.left + r.width / 2;
      var x = Math.max(8 + w / 2, Math.min(root.clientWidth - 8 - w / 2, cx));
      var above = r.bottom + 8 + ht > root.clientHeight && r.top - 8 - ht >= 0;
      bubble.classList.toggle('glim-nc-bubble-above', above);
      bubble.style.left = x + 'px';
      bubble.style.top = (above ? r.top - 8 - ht : r.bottom + 8) + 'px';
      bubble.style.setProperty('--nc-tip-ax', Math.max(10, Math.min(w - 10, cx - (x - w / 2))) + 'px');
    }
    function over(e) {
      if (doc.documentElement.classList.contains('tt-popups-on') || (e.type === 'focusin' && !keys)) return;
      var t = e.target && e.target.closest ? e.target.closest('.glim-nc-info[data-tip]') : null;
      if (!t || t === on) return;
      on = t;
      show(t);
    }
    function out(e) {
      if (on && !(e.relatedTarget && on.contains(e.relatedTarget))) hide();
    }
    doc.addEventListener('mouseover', over);
    doc.addEventListener('mouseout', out);
    doc.addEventListener('focusin', over);
    doc.addEventListener('focusout', out);
    doc.addEventListener('pointerdown', function () { keys = false; hide(); }, true);
    doc.addEventListener('keydown', function (e) { keys = true; if (e.key === 'Escape') hide(); }, true);
    if (doc.defaultView) doc.defaultView.addEventListener('scroll', hide, true);
  }

  function mount(el, adapter, opts) {
    var doc = el.ownerDocument || document;
    var t = opts.t || {}, lang = opts.lang || 'en', kinds = opts.kinds || Object.keys(FIELDS);
    var marks = (opts.marks && opts.marks.marks) || {}, lit = (opts.marks && opts.marks.tiles) || {};
    var cls = Object.assign({ input: '', button: '', switch: '' }, opts.classes);
    var list = [], typing = 0, problems = {}, addBtn = null;
    wireBubble(doc);

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

    // A button's glyph stands before its words, as on every button of the
    // house. The theme plugin leaves a button with its own glyph alone.
    function labelled(b, glyph, text) {
      var g = h('span', { class: 'glim-nc-g', 'aria-hidden': 'true' });
      g.innerHTML = GLYPHS[glyph] || '';
      b.appendChild(g);
      b.appendChild(h('span', { class: 'glim-nc-l', text: text }));
      return b;
    }

    function button(text, glyph, extra, onclick) {
      var b = labelled(h('button', { type: 'button', class: (cls.button + ' glim-nc-btn ' + (extra || '')).trim() }), glyph, text);
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

    /* The test of a channel, its button and the line that says how it went.
     * In the window's foot the button also shows it, as GlimStone's window
     * actions do: busy while it runs, then in the colour of the outcome with
     * the outcome as its title, its words as they were. */
    function tester(of, make, toned) {
      var res = h('span', { class: 'glim-nc-res', role: 'status' });
      var b = (make || button)(t.nc_test, 'test', '', function () {
        var ch = of();
        if (!ch) return;
        function done(ok, text) {
          res.className = 'glim-nc-res ' + (ok ? 'glim-nc-ok' : 'glim-nc-bad') + (toned ? ' glim-nc-win-start' : '');
          res.textContent = text;
          if (!toned) return;
          b.removeAttribute('aria-busy');
          b.disabled = false;
          b.classList.add(ok ? 'glim-nc-tone-ok' : 'glim-nc-tone-bad');
          b.setAttribute('title', text);
        }
        res.className = 'glim-nc-res glim-nc-run' + (toned ? ' glim-nc-win-start' : '');
        res.textContent = t.nc_testing;
        if (toned) {
          b.classList.remove('glim-nc-tone-ok', 'glim-nc-tone-bad');
          b.removeAttribute('title');
          b.setAttribute('aria-busy', 'true');
          b.disabled = true;
        }
        adapter.test(JSON.parse(JSON.stringify(ch))).then(function (r) {
          done(r.ok, r.ok ? t.nc_test_ok : t.nc_test_failed + ': ' + fail(t, r.code));
        }, function () {
          done(false, t.nc_test_failed);
        });
      });
      if (toned) res.className = 'glim-nc-res glim-nc-win-start';
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
        h('div', { class: 'glim-nc-ch-hd' }, [mark(c.kind), h('span', { class: 'glim-nc-title', text: title }), info(t[HINTS[c.kind]]), h('span', { class: 'glim-nc-gap' }), sw])
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
      var remove = button(t.nc_remove, 'remove', '', function () {
        list = list.filter(function (o) { return o.id !== c.id; });
        draw();
        save(true);
      });
      card.appendChild(h('div', { class: 'glim-nc-tools' }, [test.result, test.button, remove]));
      return card;
    }

    var FOCUSABLE = 'button:not([disabled]), input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

    /* The picker window: its title as a badge on the top edge with the (i),
     * the search at the top end and the tiles in their groups under it, which
     * alone scroll and keep the height they opened with while a query narrows
     * them. A picked tile's fields take the tiles' place, with Back and Save
     * beside Cancel in the foot, and Back marks the tile picked before and
     * gives it the focus. Escape empties the search first, then closes; the
     * focus goes back to Add. */
    function picker() {
      var query = '', draft = null, busy = false, scroll = null, search = null, picked = null;
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

      function winButton(text, glyph, extra, onclick) {
        var b = labelled(h('button', { type: 'button', class: 'glim-nc-win-btn' + (extra ? ' ' + extra : '') }), glyph, text);
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
            var words = h('span', { class: 'glim-nc-tile-text' }, [
              h('span', { class: 'glim-nc-tile-name', text: tile.name }),
              tile.sub ? h('span', { class: 'glim-nc-tile-sub', text: tile.sub }) : null
            ]);
            var b = h('button', { type: 'button', class: 'glim-nc-tile', 'data-kind': tile.kind, 'aria-label': tile.sub ? tile.name + ', ' + tile.sub : null }, [mark(tile.kind, 'glim-nc-tile-mark'), words]);
            b.disabled = tile.off;
            if (tile.kind === picked) b.setAttribute('aria-current', 'true');
            b.addEventListener('click', function () {
              picked = tile.kind;
              form(newChannel(tile.kind, list));
            });
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
        box.classList.remove('glim-nc-win-form');
        body.className = 'glim-nc-win-body glim-nc-win-pick';
        body.textContent = '';
        act.textContent = '';
        // The search wears its glyph and clears with a grey button of its own.
        search = h('input', { type: 'search', class: 'glim-nc-search', 'aria-label': t.nc_search, placeholder: t.nc_search, spellcheck: 'false', autocomplete: 'off' });
        search.value = query;
        var lens = h('span', { class: 'glim-nc-search-g', 'aria-hidden': 'true' });
        lens.innerHTML = GLYPHS.search;
        var clear = h('button', { type: 'button', class: 'glim-nc-search-clear', 'aria-label': t.nc_clear, title: t.nc_clear });
        clear.innerHTML = GLYPHS.close;
        clear.hidden = !query;
        function typed() {
          query = search.value;
          clear.hidden = !query;
          grid();
        }
        search.oninput = typed;
        clear.addEventListener('click', function () {
          search.value = '';
          typed();
          search.focus();
        });
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
        body.appendChild(h('span', { class: 'glim-nc-search-box' }, [lens, search, clear]));
        body.appendChild(scroll);
        grid();
        act.appendChild(winButton(t.nc_close, 'close', '', close));
        if (scroll.clientHeight) scroll.style.minHeight = scroll.clientHeight + 'px';
        (scroll.querySelector('button.glim-nc-tile[aria-current="true"]') || search).focus();
      }

      // The form narrows the window to a form's width around the same middle,
      // and its test stands in the foot with its verdict at the foot's start.
      function form(c) {
        draft = c;
        box.classList.add('glim-nc-win-form');
        var of = function () { return draft; };
        var err = h('p', { class: 'glim-nc-err', role: 'alert' });
        var test = tester(of, winButton, true);
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
          body.appendChild(err);
        }
        fill();
        act.textContent = '';
        var keep = winButton(t.nc_save, 'save', 'glim-nc-key', function () {
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
        act.appendChild(test.result);
        act.appendChild(winButton(t.nc_cancel, 'close', '', close));
        act.appendChild(winButton(t.nc_back, 'back', '', tiles));
        act.appendChild(test.button);
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
            search.oninput();
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
      // A press beside the window would throw away what was typed into a form.
      scrim.addEventListener('click', function () { if (!busy && !draft) close(); });
      doc.addEventListener('keydown', onKey, true);
      tiles();
      return { close: close, box: box };
    }

    function draw() {
      el.textContent = '';
      var root = h('div', { class: 'glim-nc' });
      if (!list.length) root.appendChild(h('p', { class: 'glim-nc-sub', text: t.nc_empty }));
      list.forEach(function (c) { root.appendChild(channel(c)); });
      addBtn = button(t.nc_add, 'add', 'glim-nc-add', function () { picker(); });
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

  return { MASK: MASK, CATALOG: CATALOG, FIELDS: FIELDS, catalogPicker: catalogPicker, newChannel: newChannel, sections: sections, matches: matches, pickKey: pickKey, shown: shown, taken: taken, name: name, get: get, set: set, mount: mount };
});
